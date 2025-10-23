package env

import (
	"os"
)

const (
	AWSRegion              = "AWS_REGION"
	AWSID                  = "AWS_ID"
	AWSSecret              = "AWS_SECRET"
	AWSToken               = "AWS_TOKEN"
	DynamoDBEndpoint       = "DYNAMODB_ENDPOINT"
	UserSecretKey          = "USER_SECRET"
	AdminSecretKey         = "ADMIN_SECRET"
	AuthRedisURL           = "AUTH_REDIS_URL"
	AuthRedisPass          = "AUTH_REDIS_PASS"
	ChatRedisURL           = "CHAT_REDIS_URL"
	ChatRedisPass          = "CHAT_REDIS_PASS"
	WebUrl                 = "WEB_URL"
)

func init() {
	required := []string{
		AWSRegion,
		AWSID,
		AWSSecret,
		// AWSToken,
		UserSecretKey,
		AuthRedisURL,
		ChatRedisURL,
		WebUrl,
	}
	for _, key := range required {
		if os.Getenv(key) == "" {
			panic("env: required environment variable not set: " + key)
		}
	}
}

func Get(key string) string {
	return os.Getenv(key)
}

func GetOrDefault(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

func MustGet(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic("env: required environment variable not set: " + key)
	}
	return val
}
