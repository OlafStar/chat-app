package endpoints

import (
	"net/http"
)

func ExtractTokenFromHeaders(r *http.Request) string {
	tokenString := r.Header.Get("Authorization")

	if tokenString == "" {
		return ""
	}

	return tokenString[len("Bearer "):]
}
