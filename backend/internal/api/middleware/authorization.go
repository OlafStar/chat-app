package middleware

import (
	iternal_jwt "chat-app-backend/internal/jwt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
)

func ValidateJWTMiddleware(role iternal_jwt.Role) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.Header.Get("Authorization")

			if tokenString == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			tokenString = tokenString[len("Bearer "):]

			claims, err := iternal_jwt.ParseToken(tokenString, role)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			expires := int64(claims["exp"].(float64))
			if time.Now().Unix() > expires {
				http.Error(w, "Token expired", http.StatusUnauthorized)
				return
			}

			next(w, r)
		}
	}
}

func ValidateMultipleJWTMiddleware(roles ...iternal_jwt.Role) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.Header.Get("Authorization")
			if tokenString == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			tokenString = tokenString[len("Bearer "):]

			var claims jwt.MapClaims
			var err error

			for _, role := range roles {
				claims, err = iternal_jwt.ParseToken(tokenString, role)
				if err == nil {
					break
				}
			}

			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			expires := int64(claims["exp"].(float64))
			if time.Now().Unix() > expires {
				http.Error(w, "Token expired", http.StatusUnauthorized)
				return
			}

			next(w, r)
		}
	}
}

var ValidateUserJWT = ValidateJWTMiddleware(iternal_jwt.RoleUser)
var ValidateAnyJWT = ValidateMultipleJWTMiddleware(iternal_jwt.RoleUser)
