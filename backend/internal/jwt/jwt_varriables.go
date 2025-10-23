package jwt

import (
	"chat-app-backend/internal/env"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	USER_SECRET          string
	RedisClient          *redis.Client
)

const RefreshTokenTTL = 24 * 30 * time.Hour

const (
	RoleUser Role = iota
)

const (
	Client        UserType = "client"
)

var RoleSecrets = map[Role]string{
	RoleUser:          USER_SECRET,
}

func init() {
	USER_SECRET = env.Get("USER_SECRET")

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     env.Get("AUTH_REDIS_URL"),
		Password: env.Get("AUTH_REDIS_PASS"),
		DB:       0,
	})
}
