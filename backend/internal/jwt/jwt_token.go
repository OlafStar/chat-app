package jwt

import (
	"chat-app-backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt"
)

func appendRoleChar(token string, role Role) string {
	switch role {
	case RoleUser:
		return token + "1"
	}
	return token
}

func expectedRoleChar(role Role) string {
	switch role {
	case RoleUser:
		return "1"
	}
	return ""
}

func CreateToken(user User, role Role, validUntil int64) (string, error) {
	secret, ok := RoleSecrets[role]
	if !ok {
		return "", fmt.Errorf("invalid role specified")
	}

	if validUntil == 0 {
		now := time.Now()
		validUntil = now.Add(time.Minute * 15).Unix()
	}

	claims := jwt.MapClaims{
		"id":    user.Id,
		"email": user.Email,
		"exp":   validUntil,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return appendRoleChar(tokenString, role), nil
}

func CreateTokenWithRefresh(user User, role Role, validUntil int64) (TokenResponse, error) {
	accessToken, err := CreateToken(user, role, validUntil)
	if err != nil {
		return TokenResponse{}, err
	}

	refreshTokenRaw := utils.CreateToken()
	refreshToken := appendRoleChar(refreshTokenRaw, role)

	userData := map[string]string{
		"id":    user.Id,
		"email": user.Email,
	}
	userDataJSON, _ := json.Marshal(userData)

	err = RedisClient.Set(context.Background(), refreshTokenRaw, userDataJSON, RefreshTokenTTL).Err()
	if err != nil {
		return TokenResponse{}, err
	}

	return TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// Parse token (access) with role char validation
func ParseToken(tokenString string, role Role) (jwt.MapClaims, error) {
	if len(tokenString) == 0 {
		return nil, fmt.Errorf("token string is empty")
	}

	if tokenString[len(tokenString)-1:] != expectedRoleChar(role) {
		return nil, fmt.Errorf("invalid role character in token")
	}
	tokenString = tokenString[:len(tokenString)-1] // Remove role char

	secret, ok := RoleSecrets[role]
	if !ok {
		return nil, fmt.Errorf("invalid role specified")
	}

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("unauthorized: %v", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is not valid - unauthorized")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("claims of unauthorized type")
	}

	return claims, nil
}

func RefreshToken(refreshToken string, role Role) (string, error) {
	if len(refreshToken) == 0 {
		return "", fmt.Errorf("refresh token is empty")
	}
	if refreshToken[len(refreshToken)-1:] != expectedRoleChar(role) {
		return "", fmt.Errorf("invalid role character in refresh token")
	}
	refreshTokenRaw := refreshToken[:len(refreshToken)-1]

	val, err := RedisClient.Get(context.Background(), refreshTokenRaw).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("invalid refresh token")
	} else if err != nil {
		return "", err
	}

	var userData map[string]string
	if err := json.Unmarshal([]byte(val), &userData); err != nil {
		return "", fmt.Errorf("invalid token data")
	}

	user := User{
		Id:    userData["id"],
		Email: userData["email"],
	}

	err = cleanupExpiredTokens(user.Email, refreshTokenRaw)
	if err != nil {
		return "", fmt.Errorf("failed to clean up expired refresh tokens: %v", err)
	}

	err = RedisClient.Expire(context.Background(), refreshTokenRaw, RefreshTokenTTL).Err()
	if err != nil {
		return "", fmt.Errorf("failed to update refresh token expiration: %v", err)
	}

	return CreateToken(user, role, 0)
}

func cleanupExpiredTokens(email, currentRefreshTokenRaw string) error {
	iter := RedisClient.Scan(context.Background(), 0, "*", 0).Iterator()
	for iter.Next(context.Background()) {
		key := iter.Val()
		val, err := RedisClient.Get(context.Background(), key).Result()
		if err == redis.Nil {
			continue
		} else if err != nil {
			return fmt.Errorf("error fetching key %s: %v", key, err)
		}

		var userData map[string]string
		if err := json.Unmarshal([]byte(val), &userData); err != nil {
			continue
		}

		if userData["email"] == email && key != currentRefreshTokenRaw {
			ttl, err := RedisClient.TTL(context.Background(), key).Result()
			if err != nil {
				return fmt.Errorf("error checking TTL for key %s: %v", key, err)
			}
			if ttl <= 0 {
				_ = RedisClient.Del(context.Background(), key).Err()
			}
		}
	}
	return iter.Err()
}
