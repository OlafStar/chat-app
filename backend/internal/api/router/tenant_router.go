package router

import (
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/endpoints"
	"chat-app-backend/internal/api/middleware"
	tenantservice "chat-app-backend/internal/service/tenant"
	"net/http"
)

func TenantRoutes(prefix string) api.RouteRegistrar {
	return func(mux *http.ServeMux, s *api.APIServer) {
		service := tenantservice.New(s.Database())
		tenantEndpoints := endpoints.NewTenantEndpoints(service)

		mux.HandleFunc(prefix+"/tenant", s.MakeHTTPHandleFunc(tenantEndpoints.UpdateTenant, middleware.ValidateUserJWT))
		mux.HandleFunc(prefix+"/tenant/users", s.MakeHTTPHandleFunc(tenantEndpoints.AddTenantUser, middleware.ValidateUserJWT))
		mux.HandleFunc(prefix+"/tenant/invites/accept", s.MakeHTTPHandleFunc(tenantEndpoints.AcceptInvite))
		mux.HandleFunc(prefix+"/tenant/invites/pending", s.MakeHTTPHandleFunc(tenantEndpoints.ListPendingInvites, middleware.ValidateUserJWT))
	}
}
