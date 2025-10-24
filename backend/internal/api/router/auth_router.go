package router

import (
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/endpoints"
	"chat-app-backend/internal/api/middleware"
	"net/http"
)

func AuthRoutes(prefix string) api.RouteRegistrar {
	return func(mux *http.ServeMux, s *api.APIServer) {
		authEndpoints := endpoints.NewAuthEndpoints(s.Database())
		mux.HandleFunc(prefix+"/auth/register", s.MakeHTTPHandleFunc(authEndpoints.Register))
		mux.HandleFunc(prefix+"/auth/login", s.MakeHTTPHandleFunc(authEndpoints.Login))
		mux.HandleFunc(prefix+"/auth/me", s.MakeHTTPHandleFunc(authEndpoints.Me, middleware.ValidateUserJWT))
		mux.HandleFunc(prefix+"/auth/switch", s.MakeHTTPHandleFunc(authEndpoints.Switch, middleware.ValidateUserJWT))
	}
}
