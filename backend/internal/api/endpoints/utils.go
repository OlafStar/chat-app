package endpoints

import (
	"chat-app-backend/internal/api"
	"fmt"
	"net/http"
)

type ApiMessageResponse struct {
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	return api.WriteJSON(w, status, v)
}

func MethodHandler(
	w http.ResponseWriter,
	r *http.Request,
	allowed map[string]func(http.ResponseWriter, *http.Request) error,
) error {
	if handler, ok := allowed[r.Method]; ok {
		return handler(w, r)
	}
	return &HTTPError{
		StatusCode: http.StatusMethodNotAllowed,
		Message:    "Method not allowed.",
		ErrorLog:   fmt.Errorf("method not allowed"),
	}
}
