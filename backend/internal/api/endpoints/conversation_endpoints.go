package endpoints

import (
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/dto"
	"chat-app-backend/internal/model"
	conversationservice "chat-app-backend/internal/service/conversation"
	"chat-app-backend/internal/websocket"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ConversationEndpoints interface {
	PublicConversations(http.ResponseWriter, *http.Request) error
	PublicConversationMessages(http.ResponseWriter, *http.Request) error
	Conversations(http.ResponseWriter, *http.Request) error
	ConversationMessages(http.ResponseWriter, *http.Request) error
	ConversationUsage(http.ResponseWriter, *http.Request) error
	Websocket(http.ResponseWriter, *http.Request) error
	NotificationsWebsocket(http.ResponseWriter, *http.Request) error
}

type ConversationPaths struct {
	PublicConversationsPath          string
	PublicConversationMessagesPrefix string
	TenantConversationsPath          string
	TenantConversationPrefix         string
	WebsocketPrefix                  string
	TenantNotificationPath           string
}

type conversationEndpoints struct {
	service *conversationservice.Service
	handler *websocket.Handler
	paths   ConversationPaths
}

func NewConversationEndpoints(service *conversationservice.Service, handler *websocket.Handler, prefix string) ConversationEndpoints {
	base := strings.TrimRight(prefix, "/")
	return NewConversationEndpointsWithPaths(service, handler, ConversationPaths{
		PublicConversationsPath:          base + "/public/conversations",
		PublicConversationMessagesPrefix: base + "/public/conversations/",
		TenantConversationsPath:          base + "/conversations",
		TenantConversationPrefix:         base + "/conversations/",
		WebsocketPrefix:                  base + "/ws/conversations/",
		TenantNotificationPath:           base + "/ws/notifications",
	})
}

func NewConversationEndpointsWithPaths(service *conversationservice.Service, handler *websocket.Handler, paths ConversationPaths) ConversationEndpoints {
	return &conversationEndpoints{
		service: service,
		handler: handler,
		paths:   paths,
	}
}

func (h *conversationEndpoints) PublicConversations(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodPost: h.handleCreateConversation,
	})
}

func (h *conversationEndpoints) PublicConversationMessages(w http.ResponseWriter, r *http.Request) error {
	trimmed := strings.TrimRight(r.URL.Path, "/")
	if strings.HasSuffix(trimmed, "/email") {
		return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
			http.MethodPost: h.handleAssignVisitorEmail,
		})
	}

	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodGet:  h.handleListPublicMessages,
		http.MethodPost: h.handlePostVisitorMessage,
	})
}

func (h *conversationEndpoints) Conversations(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodGet: h.handleListConversations,
	})
}

func (h *conversationEndpoints) ConversationMessages(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodGet:  h.handleListMessages,
		http.MethodPost: h.handlePostAgentMessage,
	})
}

func (h *conversationEndpoints) ConversationUsage(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodGet: h.handleConversationUsage,
	})
}

func (h *conversationEndpoints) Websocket(w http.ResponseWriter, r *http.Request) error {
	convID, err := h.extractFromPath(r.URL.Path, h.paths.WebsocketPrefix)
	if err != nil {
		return err
	}
	convID = strings.Trim(convID, "/")
	if convID == "" {
		return &HTTPError{
			StatusCode: http.StatusNotFound,
			Message:    "Conversation not found",
			ErrorLog:   fmt.Errorf("websocket conversation id missing"),
		}
	}

	role := r.URL.Query().Get("role")
	switch role {
	case "visitor":
		token := r.URL.Query().Get("token")
		access, err := h.service.ValidateVisitorAccess(token)
		if err != nil {
			return h.serviceError(err)
		}
		if access.ConversationID != convID {
			return &HTTPError{
				StatusCode: http.StatusForbidden,
				Message:    "Token does not match conversation",
				ErrorLog:   fmt.Errorf("websocket conversation mismatch: %s vs %s", access.ConversationID, convID),
			}
		}
		h.ensureRoom(convID)
		h.handler.JoinRoom(w, r, convID, access.VisitorID)
		return nil

	case "agent", "user", "tenant":
		identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
		if err != nil {
			return h.serviceError(err)
		}
		if identity.TenantID == "" {
			return &HTTPError{StatusCode: http.StatusUnauthorized, Message: "Unauthorized", ErrorLog: fmt.Errorf("websocket missing tenant")}
		}
		h.ensureRoom(convID)
		h.handler.JoinRoom(w, r, convID, identity.UserID)
		return nil

	default:
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Missing or invalid role parameter",
			ErrorLog:   fmt.Errorf("websocket role invalid: %s", role),
		}
	}
}

func (h *conversationEndpoints) NotificationsWebsocket(w http.ResponseWriter, r *http.Request) error {
	if strings.TrimSpace(h.paths.TenantNotificationPath) == "" {
		return &HTTPError{
			StatusCode: http.StatusNotFound,
			Message:    "Websocket not configured",
			ErrorLog:   fmt.Errorf("notification websocket path not configured"),
		}
	}

	if h.handler == nil {
		return &HTTPError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "Websocket not available",
			ErrorLog:   fmt.Errorf("notification websocket handler missing"),
		}
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		return &HTTPError{
			StatusCode: http.StatusUnauthorized,
			Message:    "Missing token",
			ErrorLog:   fmt.Errorf("notification websocket missing token"),
		}
	}

	identity, err := h.service.IdentityFromToken(token)
	if err != nil {
		return h.serviceError(err)
	}
	if identity.TenantID == "" {
		return &HTTPError{
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
			ErrorLog:   fmt.Errorf("notification websocket missing tenant"),
		}
	}

	roomID := tenantNotificationRoomID(identity.TenantID)
	if roomID == "" {
		return &HTTPError{
			StatusCode: http.StatusInternalServerError,
			Message:    "Unable to resolve notification room",
			ErrorLog:   fmt.Errorf("notification websocket invalid tenant room"),
		}
	}

	h.ensureRoom(roomID)
	h.handler.JoinRoom(w, r, roomID, identity.UserID)
	return nil
}

func (h *conversationEndpoints) handleCreateConversation(w http.ResponseWriter, r *http.Request) error {
	var req dto.CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode create conversation request: %w", err),
		}
	}

	result, err := h.service.CreateConversation(r.Context(), conversationservice.CreateConversationParams{
		TenantID:     strings.TrimSpace(req.TenantID),
		TenantAPIKey: strings.TrimSpace(r.Header.Get("X-Tenant-Key")),
		Visitor: conversationservice.VisitorParams{
			VisitorID: strings.TrimSpace(req.Visitor.VisitorID),
			Name:      strings.TrimSpace(req.Visitor.Name),
			Email:     strings.TrimSpace(req.Visitor.Email),
			Metadata:  req.Visitor.Metadata,
		},
		Message:  req.Message.Body,
		Metadata: req.Metadata,
		Origin:   strings.TrimSpace(req.Origin.URL),
	})
	if err != nil {
		return h.serviceError(err)
	}

	h.ensureRoom(result.Conversation.ConversationID)
	h.broadcastEvent("conversation.created", result.Conversation, result.Message)

	resp := dto.CreateConversationResponse{
		Conversation: toConversationMetadata(result.Conversation),
		VisitorToken: result.VisitorToken,
		VisitorID:    result.Conversation.VisitorID,
		Message:      toMessageResponse(result.Message),
	}

	return api.WriteJSON(w, http.StatusCreated, resp)
}

func (h *conversationEndpoints) handlePostVisitorMessage(w http.ResponseWriter, r *http.Request) error {
	convID, err := h.extractPublicMessagePath(r.URL.Path)
	if err != nil {
		return err
	}

	var req dto.PostVisitorMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode visitor message request: %w", err),
		}
	}

	result, err := h.service.PostVisitorMessage(r.Context(), req.VisitorToken, req.Body)
	if err != nil {
		return h.serviceError(err)
	}

	if convID != "" && convID != result.Conversation.ConversationID {
		return &HTTPError{
			StatusCode: http.StatusForbidden,
			Message:    "Token does not match conversation",
			ErrorLog:   fmt.Errorf("visitor message path mismatch: %s vs %s", convID, result.Conversation.ConversationID),
		}
	}

	h.broadcastEvent("message.created", result.Conversation, result.Message)

	return api.WriteJSON(w, http.StatusCreated, toMessageResponse(result.Message))
}

func (h *conversationEndpoints) handleAssignVisitorEmail(w http.ResponseWriter, r *http.Request) error {
	convID, err := h.extractPublicEmailPath(r.URL.Path)
	if err != nil {
		return err
	}

	var req dto.AssignConversationEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode visitor email request: %w", err),
		}
	}

	req.Email = strings.TrimSpace(req.Email)
	req.VisitorToken = strings.TrimSpace(req.VisitorToken)
	if req.VisitorToken == "" {
		req.VisitorToken = strings.TrimSpace(r.Header.Get("X-Visitor-Token"))
	}

	conversation, err := h.service.AssignVisitorEmail(r.Context(), req.VisitorToken, req.Email)
	if err != nil {
		return h.serviceError(err)
	}

	if conversation.ConversationID != convID {
		return &HTTPError{
			StatusCode: http.StatusForbidden,
			Message:    "Token does not match conversation",
			ErrorLog:   fmt.Errorf("visitor email path mismatch: %s vs %s", convID, conversation.ConversationID),
		}
	}

	resp := dto.AssignConversationEmailResponse{
		Conversation: toConversationMetadata(conversation),
	}

	return api.WriteJSON(w, http.StatusOK, resp)
}

func (h *conversationEndpoints) handleListPublicMessages(w http.ResponseWriter, r *http.Request) error {
	conversationID, err := h.extractPublicMessagePath(r.URL.Path)
	if err != nil {
		return err
	}

	token := strings.TrimSpace(r.URL.Query().Get("visitorToken"))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-Visitor-Token"))
	}

	result, err := h.service.ListVisitorMessages(r.Context(), token, conversationID, 100)
	if err != nil {
		return h.serviceError(err)
	}

	resp := dto.ListMessagesResponse{Messages: make([]dto.MessageResponse, len(result.Messages))}
	for i, msg := range result.Messages {
		resp.Messages[i] = toMessageResponse(msg)
	}

	return api.WriteJSON(w, http.StatusOK, resp)
}

func (h *conversationEndpoints) handleListConversations(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	result, err := h.service.ListConversations(r.Context(), identity, 50)
	if err != nil {
		return h.serviceError(err)
	}

	resp := dto.ListConversationsResponse{Conversations: make([]dto.ConversationMetadata, len(result.Conversations))}
	for i, conv := range result.Conversations {
		resp.Conversations[i] = toConversationMetadata(conv)
	}

	return api.WriteJSON(w, http.StatusOK, resp)
}

func (h *conversationEndpoints) handleListMessages(w http.ResponseWriter, r *http.Request) error {
	conversationID, err := h.extractTenantConversationPath(r.URL.Path)
	if err != nil {
		return err
	}

	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	result, err := h.service.ListMessages(r.Context(), identity, conversationID, 100)
	if err != nil {
		return h.serviceError(err)
	}

	resp := dto.ListMessagesResponse{Messages: make([]dto.MessageResponse, len(result.Messages))}
	for i, msg := range result.Messages {
		resp.Messages[i] = toMessageResponse(msg)
	}

	return api.WriteJSON(w, http.StatusOK, resp)
}

func (h *conversationEndpoints) handleConversationUsage(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	start, end, monthLabel, err := resolveUsagePeriod(r.URL.Query().Get("month"))
	if err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid month parameter",
			ErrorLog:   err,
		}
	}

	result, err := h.service.GetConversationUsage(r.Context(), identity, start, end)
	if err != nil {
		return h.serviceError(err)
	}

	resp := dto.ConversationUsageResponse{
		TenantID:             result.TenantID,
		Month:                monthLabel,
		PeriodStart:          result.PeriodStart.Format(time.RFC3339),
		PeriodEnd:            result.PeriodEnd.Format(time.RFC3339),
		ConversationsStarted: result.StartedCount,
	}
	return api.WriteJSON(w, http.StatusOK, resp)
}

func (h *conversationEndpoints) handlePostAgentMessage(w http.ResponseWriter, r *http.Request) error {
	conversationID, err := h.extractTenantConversationPath(r.URL.Path)
	if err != nil {
		return err
	}

	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	var req dto.PostAgentMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode agent message request: %w", err),
		}
	}

	result, err := h.service.PostAgentMessage(r.Context(), identity, conversationID, req.Body)
	if err != nil {
		return h.serviceError(err)
	}

	h.broadcastEvent("message.created", result.Conversation, result.Message)

	return api.WriteJSON(w, http.StatusCreated, toMessageResponse(result.Message))
}

func resolveUsagePeriod(param string) (time.Time, time.Time, string, error) {
	month := strings.TrimSpace(param)
	var start time.Time
	if month == "" {
		now := time.Now().UTC()
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		month = start.Format("2006-01")
	} else {
		parsed, err := time.Parse("2006-01", month)
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("invalid month value: %w", err)
		}
		start = time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.UTC)
		month = start.Format("2006-01")
	}
	end := start.AddDate(0, 1, 0)
	return start, end, month, nil
}

func (h *conversationEndpoints) extractPublicMessagePath(path string) (string, error) {
	return h.extractPublicConversationAction(path, "messages")
}

func (h *conversationEndpoints) extractPublicEmailPath(path string) (string, error) {
	return h.extractPublicConversationAction(path, "email")
}

func (h *conversationEndpoints) extractPublicConversationAction(path, action string) (string, error) {
	prefix := h.paths.PublicConversationMessagesPrefix
	if prefix == "" {
		return "", &HTTPError{StatusCode: http.StatusNotFound, Message: "Conversation not found", ErrorLog: fmt.Errorf("public route not configured")}
	}
	trimmed := strings.TrimPrefix(path, prefix)
	if trimmed == path {
		return "", &HTTPError{StatusCode: http.StatusNotFound, Message: "Conversation not found", ErrorLog: fmt.Errorf("public path mismatch: %s", path)}
	}
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 || parts[1] != action {
		return "", &HTTPError{StatusCode: http.StatusNotFound, Message: "Conversation not found", ErrorLog: fmt.Errorf("invalid public %s path: %s", action, path)}
	}
	return parts[0], nil
}

func (h *conversationEndpoints) extractTenantConversationPath(path string) (string, error) {
	prefix := h.paths.TenantConversationPrefix
	if prefix == "" {
		return "", &HTTPError{StatusCode: http.StatusNotFound, Message: "Conversation not found", ErrorLog: fmt.Errorf("tenant messaging not configured")}
	}
	trimmed := strings.TrimPrefix(path, prefix)
	if trimmed == path {
		return "", &HTTPError{StatusCode: http.StatusNotFound, Message: "Conversation not found", ErrorLog: fmt.Errorf("conversation path mismatch: %s", path)}
	}
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 2 || parts[1] != "messages" {
		return "", &HTTPError{StatusCode: http.StatusNotFound, Message: "Conversation not found", ErrorLog: fmt.Errorf("invalid conversation path: %s", path)}
	}
	return parts[0], nil
}

func (h *conversationEndpoints) extractFromPath(path, prefix string) (string, error) {
	if prefix == "" {
		return "", &HTTPError{StatusCode: http.StatusNotFound, Message: "Conversation not found", ErrorLog: fmt.Errorf("websocket not configured")}
	}
	trimmed := strings.TrimPrefix(path, prefix)
	if trimmed == path {
		return "", &HTTPError{StatusCode: http.StatusNotFound, Message: "Conversation not found", ErrorLog: fmt.Errorf("path mismatch: %s", path)}
	}
	return trimmed, nil
}

func (h *conversationEndpoints) ensureRoom(conversationID string) {
	if conversationID == "" || h.handler == nil {
		return
	}
	h.handler.CreateRoom(conversationID)
}

func (h *conversationEndpoints) broadcastEvent(eventType string, conversation model.ConversationItem, message model.MessageItem) {
	payload := map[string]interface{}{
		"type":          eventType,
		"conversation":  toConversationMetadata(conversation),
		"message":       toMessageResponse(message),
		"broadcastedAt": time.Now().UTC().Format(time.RFC3339),
	}

	h.notifyRoom(conversation.ConversationID, payload)
	h.notifyTenant(conversation.TenantID, payload)
}

func (h *conversationEndpoints) notifyTenant(tenantID string, payload interface{}) {
	roomID := tenantNotificationRoomID(tenantID)
	h.notifyRoom(roomID, payload)
}

func (h *conversationEndpoints) notifyRoom(roomID string, payload interface{}) {
	if roomID == "" {
		return
	}

	if err := websocket.Publish(roomID, payload); err != nil {
		fmt.Printf("failed to publish websocket payload for room %s: %v\n", roomID, err)
	}

	if h.handler != nil {
		h.handler.NotifyRoom(roomID, payload)
	}
}

func (h *conversationEndpoints) serviceError(err error) error {
	if err == nil {
		return nil
	}

	svcErr, ok := err.(*conversationservice.Error)
	if !ok {
		return &HTTPError{
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal server error",
			ErrorLog:   fmt.Errorf("conversation service: %w", err),
		}
	}

	var logErr error
	if svcErr.Err != nil {
		logErr = fmt.Errorf("%s: %w", svcErr.Message, svcErr.Err)
	} else {
		logErr = svcErr
	}

	switch svcErr.Code {
	case conversationservice.ErrorCodeValidation:
		return &HTTPError{StatusCode: http.StatusBadRequest, Message: svcErr.Message, ErrorLog: logErr}
	case conversationservice.ErrorCodeUnauthorized:
		return &HTTPError{StatusCode: http.StatusUnauthorized, Message: svcErr.Message, ErrorLog: logErr}
	case conversationservice.ErrorCodeForbidden:
		return &HTTPError{StatusCode: http.StatusForbidden, Message: svcErr.Message, ErrorLog: logErr}
	case conversationservice.ErrorCodeNotFound:
		return &HTTPError{StatusCode: http.StatusNotFound, Message: svcErr.Message, ErrorLog: logErr}
	case conversationservice.ErrorCodeConflict:
		return &HTTPError{StatusCode: http.StatusConflict, Message: svcErr.Message, ErrorLog: logErr}
	default:
		return &HTTPError{StatusCode: http.StatusInternalServerError, Message: "Internal server error", ErrorLog: logErr}
	}
}

func toConversationMetadata(item model.ConversationItem) dto.ConversationMetadata {
	return dto.ConversationMetadata{
		ConversationID: item.ConversationID,
		VisitorID:      item.VisitorID,
		VisitorName:    item.VisitorName,
		VisitorEmail:   item.VisitorEmail,
		Status:         string(item.Status),
		AssignedUserID: item.AssignedUserID,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		LastMessageAt:  item.LastMessageAt,
		OriginURL:      item.OriginURL,
		Metadata:       cloneMetadata(item.Metadata),
	}
}

func toMessageResponse(item model.MessageItem) dto.MessageResponse {
	return dto.MessageResponse{
		MessageID:      item.MessageID,
		ConversationID: item.ConversationID,
		SenderType:     item.SenderType,
		SenderID:       item.SenderID,
		Body:           item.Body,
		CreatedAt:      item.CreatedAt,
	}
}

func cloneMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func tenantNotificationRoomID(tenantID string) string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ""
	}
	return fmt.Sprintf("tenant:%s:notifications", tenantID)
}
