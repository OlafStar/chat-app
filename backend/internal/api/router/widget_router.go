package router

import (
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/endpoints"
	"chat-app-backend/internal/api/middleware"
	tenantservice "chat-app-backend/internal/service/tenant"
	"net/http"
)

func WidgetRoutes(prefix string) api.RouteRegistrar {
	return func(mux *http.ServeMux, s *api.APIServer) {
		service := tenantservice.New(s.Database())
		widgetEndpoints := endpoints.NewWidgetEndpoints(service)

		mux.HandleFunc(prefix+"/widget", s.MakeHTTPHandleFunc(widgetEndpoints.TenantWidgetSettings, middleware.ValidateUserJWT))
	}
}

func WidgetPublicRoutes(prefix string) api.RouteRegistrar {
	return func(mux *http.ServeMux, s *api.APIServer) {
		service := tenantservice.New(s.Database())
		widgetEndpoints := endpoints.NewWidgetEndpoints(service)

		mux.HandleFunc(prefix+"/widget", s.MakeHTTPHandleFunc(widgetEndpoints.PublicWidgetSettings))
	}
}
