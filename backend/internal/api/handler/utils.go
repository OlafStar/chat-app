package handler

import (
	"chat-app-backend/internal/api"
	"net/http"
)

type ApiMessageResponse struct {
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	return api.WriteJSON(w, status, v)
}
