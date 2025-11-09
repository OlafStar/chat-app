package router

import (
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/endpoints"
	"chat-app-backend/internal/api/middleware"
	conversationservice "chat-app-backend/internal/service/conversation"
	"net/http"
	"strings"
)

func ConversationPublicRoutes(prefix string) api.RouteRegistrar {
	return func(mux *http.ServeMux, s *api.APIServer) {
		service := conversationservice.New(s.Database())
		paths := endpoints.ConversationPaths{
			PublicConversationsPath:          strings.TrimRight(prefix, "/") + "/conversations",
			PublicConversationMessagesPrefix: strings.TrimRight(prefix, "/") + "/conversations/",
		}
		convEndpoints := endpoints.NewConversationEndpointsWithPaths(service, s.Handler(), paths)

		mux.HandleFunc(prefix+"/conversations", s.MakeHTTPHandleFunc(convEndpoints.PublicConversations))
		mux.HandleFunc(prefix+"/conversations/", s.MakeHTTPHandleFunc(convEndpoints.PublicConversationMessages))
	}
}

func ConversationTenantRoutes(prefix string) api.RouteRegistrar {
	return func(mux *http.ServeMux, s *api.APIServer) {
		service := conversationservice.New(s.Database())
		paths := endpoints.ConversationPaths{
			TenantConversationsPath:  strings.TrimRight(prefix, "/") + "/conversations",
			TenantConversationPrefix: strings.TrimRight(prefix, "/") + "/conversations/",
		}
		convEndpoints := endpoints.NewConversationEndpointsWithPaths(service, s.Handler(), paths)

		mux.HandleFunc(prefix+"/conversations", s.MakeHTTPHandleFunc(convEndpoints.Conversations, middleware.ValidateUserJWT))
		mux.HandleFunc(prefix+"/conversations/usage", s.MakeHTTPHandleFunc(convEndpoints.ConversationUsage, middleware.ValidateUserJWT))
		mux.HandleFunc(prefix+"/conversations/", s.MakeHTTPHandleFunc(convEndpoints.ConversationMessages, middleware.ValidateUserJWT))
	}
}

func ConversationWebsocketRoutes(prefix string) api.RouteRegistrar {
	return func(mux *http.ServeMux, s *api.APIServer) {
		service := conversationservice.New(s.Database())
		paths := endpoints.ConversationPaths{
			WebsocketPrefix:        strings.TrimRight(prefix, "/") + "/conversations/",
			TenantNotificationPath: strings.TrimRight(prefix, "/") + "/notifications",
		}
		convEndpoints := endpoints.NewConversationEndpointsWithPaths(service, s.Handler(), paths)

		mux.HandleFunc(prefix+"/conversations/", s.MakeHTTPHandleFunc(convEndpoints.Websocket))
		mux.HandleFunc(prefix+"/notifications", s.MakeHTTPHandleFunc(convEndpoints.NotificationsWebsocket))
	}
}
