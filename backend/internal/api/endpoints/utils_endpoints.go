package endpoints

import (
	"net/http"
)

type UtilsEndpoints interface {
	HelloWorld(http.ResponseWriter, *http.Request) error
	Health(http.ResponseWriter, *http.Request) error
}

type utilsEndpoints struct{}

func NewUtilsEndpoints() UtilsEndpoints {
	return &utilsEndpoints{}
}

func (h *utilsEndpoints) HelloWorld(w http.ResponseWriter, r *http.Request) error {
	return WriteJSON(w, http.StatusOK, map[string]string{"message": "Hello world"})
}

func (h *utilsEndpoints) Health(w http.ResponseWriter, r *http.Request) error {
	return WriteJSON(w, http.StatusOK, struct{}{})
}