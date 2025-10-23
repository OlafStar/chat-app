package router

import (
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/endpoints"
	"net/http"
)

func UtilsRoutes(prefix string) api.RouteRegistrar {
	return func(mux *http.ServeMux, s *api.APIServer) {
		utilsEndpoints := endpoints.NewUtilsEndpoints()
		mux.HandleFunc(prefix+"/hello-world", s.MakeHTTPHandleFunc(utilsEndpoints.HelloWorld))
		mux.HandleFunc(prefix+"/health", s.MakeHTTPHandleFunc(utilsEndpoints.Health))
	}
}
