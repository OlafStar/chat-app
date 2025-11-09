package endpoints

import (
	"bytes"
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/middleware"
	"chat-app-backend/internal/dto"
	internaljwt "chat-app-backend/internal/jwt"
	"chat-app-backend/internal/model"
	"chat-app-backend/internal/queue"
	conversationservice "chat-app-backend/internal/service/conversation"
	"chat-app-backend/internal/websocket"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"testing"
	"time"
)

type memoryRepository struct {
	mu            sync.Mutex
	tenants       map[string]model.TenantItem
	users         map[string]model.UserItem
	visitors      map[string]model.VisitorItem
	conversations map[string]model.ConversationItem
	messages      map[string][]model.MessageItem
	keys          map[string]string
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		tenants:       make(map[string]model.TenantItem),
		users:         make(map[string]model.UserItem),
		visitors:      make(map[string]model.VisitorItem),
		conversations: make(map[string]model.ConversationItem),
		messages:      make(map[string][]model.MessageItem),
		keys:          make(map[string]string),
	}
}

func (m *memoryRepository) GetTenant(ctx context.Context, tenantID string) (model.TenantItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenant, ok := m.tenants[tenantID]
	if !ok {
		return model.TenantItem{}, conversationservice.ErrNotFound
	}
	return tenant, nil
}

func (m *memoryRepository) GetTenantByAPIKey(ctx context.Context, apiKey string) (model.TenantItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tenantID, ok := m.keys[apiKey]; ok {
		if tenant, ok := m.tenants[tenantID]; ok {
			return tenant, nil
		}
	}
	return model.TenantItem{}, conversationservice.ErrNotFound
}

func (m *memoryRepository) GetUser(ctx context.Context, tenantID, userID string) (model.UserItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.TenantScopedPK(tenantID, userID)
	user, ok := m.users[pk]
	if !ok {
		return model.UserItem{}, conversationservice.ErrNotFound
	}
	return user, nil
}

func (m *memoryRepository) GetVisitor(ctx context.Context, tenantID, visitorID string) (model.VisitorItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.VisitorPK(tenantID, visitorID)
	visitor, ok := m.visitors[pk]
	if !ok {
		return model.VisitorItem{}, conversationservice.ErrNotFound
	}
	return visitor, nil
}

func (m *memoryRepository) PutVisitor(ctx context.Context, visitor model.VisitorItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.visitors[visitor.PK] = visitor
	return nil
}

func (m *memoryRepository) CreateConversation(ctx context.Context, conversation model.ConversationItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conversations[conversation.PK] = conversation
	return nil
}

func (m *memoryRepository) UpdateConversationActivity(ctx context.Context, tenantID, conversationID, updatedAt, lastMessageAt string, assignedUserID *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.ConversationPK(tenantID, conversationID)
	conv, ok := m.conversations[pk]
	if !ok {
		return conversationservice.ErrNotFound
	}
	conv.UpdatedAt = updatedAt
	conv.LastMessageAt = lastMessageAt
	if assignedUserID != nil {
		conv.AssignedUserID = *assignedUserID
	}
	m.conversations[pk] = conv
	return nil
}

func (m *memoryRepository) GetConversation(ctx context.Context, tenantID, conversationID string) (model.ConversationItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.ConversationPK(tenantID, conversationID)
	conv, ok := m.conversations[pk]
	if !ok {
		return model.ConversationItem{}, conversationservice.ErrNotFound
	}
	return conv, nil
}

func (m *memoryRepository) ListConversations(ctx context.Context, tenantID string, limit int) ([]model.ConversationItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]model.ConversationItem, 0)
	for _, conv := range m.conversations {
		if conv.TenantID == tenantID {
			items = append(items, conv)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].LastMessageAt > items[j].LastMessageAt })
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (m *memoryRepository) CreateMessage(ctx context.Context, message model.MessageItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[message.ConversationID] = append(m.messages[message.ConversationID], message)
	return nil
}

func (m *memoryRepository) ListMessages(ctx context.Context, tenantID, conversationID string, limit int) ([]model.MessageItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]model.MessageItem, 0)
	for _, msg := range m.messages[conversationID] {
		if msg.TenantID == tenantID {
			items = append(items, msg)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, items[i].CreatedAt)
		tj, _ := time.Parse(time.RFC3339, items[j].CreatedAt)
		return ti.Before(tj)
	})
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}
	return items, nil
}

func setupConversationTestHandler(t *testing.T) (http.Handler, *conversationservice.Service, *memoryRepository) {
	t.Helper()

	repo := newMemoryRepository()
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	svc := conversationservice.NewWithRepository(repo, func() time.Time { return now })

	useTestVisitorSecret(t)

	originalSecret := internaljwt.RoleSecrets[internaljwt.RoleUser]
	internaljwt.RoleSecrets[internaljwt.RoleUser] = "jwt-test-secret"
	t.Cleanup(func() {
		internaljwt.RoleSecrets[internaljwt.RoleUser] = originalSecret
	})

	hub := websocket.NewHub()
	go hub.Run()
	handler := websocket.NewHandler(hub)

	queueManager := queue.NewRequestQueueManager(10, 1)
	server := api.NewAPIServer(":0", queueManager, nil, handler)

	endpoints := NewConversationEndpoints(svc, handler, "/api")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/public/conversations", server.MakeHTTPHandleFunc(endpoints.PublicConversations))
	mux.HandleFunc("/api/public/conversations/", server.MakeHTTPHandleFunc(endpoints.PublicConversationMessages))
	mux.HandleFunc("/api/conversations", server.MakeHTTPHandleFunc(endpoints.Conversations, middleware.ValidateUserJWT))
	mux.HandleFunc("/api/conversations/", server.MakeHTTPHandleFunc(endpoints.ConversationMessages, middleware.ValidateUserJWT))
	mux.HandleFunc("/api/ws/conversations/", server.MakeHTTPHandleFunc(endpoints.Websocket))

	t.Cleanup(queueManager.Shutdown)

	return mux, svc, repo
}

func useTestVisitorSecret(t *testing.T) {
	t.Helper()
	original := []byte(os.Getenv("USER_SECRET"))
	if len(original) == 0 {
		original = []byte("fallback-secret")
	}
	conversationservice.SetVisitorTokenSecret([]byte("visitor-secret"))
	t.Cleanup(func() {
		conversationservice.SetVisitorTokenSecret(original)
	})
}

func TestCreateConversationEndpoint(t *testing.T) {
	handler, _, repo := setupConversationTestHandler(t)
	repo.tenants["tenant-1"] = model.TenantItem{TenantID: "tenant-1"}
	repo.keys["public-key"] = "tenant-1"

	payload := dto.CreateConversationRequest{
		TenantID: "",
		Visitor: dto.CreateVisitorPayload{
			Name: "Visitor",
		},
		Message: dto.CreateMessagePayload{Body: "Hi"},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/public/conversations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Key", "public-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", res.StatusCode)
	}

	var resp dto.CreateConversationResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.VisitorToken == "" {
		t.Fatal("expected visitor token")
	}
	if resp.Conversation.ConversationID == "" {
		t.Fatal("expected conversation id")
	}
}

func TestVisitorMessageInvalidToken(t *testing.T) {
	handler, _, repo := setupConversationTestHandler(t)
	repo.tenants["tenant-1"] = model.TenantItem{TenantID: "tenant-1"}
	repo.keys["public-key"] = "tenant-1"

	payload := dto.PostVisitorMessageRequest{Body: "Hello", VisitorToken: "bad.token"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/public/conversations/conv/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestPublicListMessagesRequiresToken(t *testing.T) {
	handler, svc, repo := setupConversationTestHandler(t)
	repo.tenants["tenant-1"] = model.TenantItem{TenantID: "tenant-1"}
	repo.keys["public-key"] = "tenant-1"

	result, err := svc.CreateConversation(context.Background(), conversationservice.CreateConversationParams{
		TenantAPIKey: "public-key",
		Message:      "Hello",
		Visitor:      conversationservice.VisitorParams{Name: "Visitor"},
	})
	if err != nil {
		t.Fatalf("CreateConversation error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/public/conversations/"+result.Conversation.ConversationID+"/messages", nil)
	req.Header.Set("X-Tenant-Key", "public-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestPublicListMessagesSuccess(t *testing.T) {
	handler, svc, repo := setupConversationTestHandler(t)
	repo.tenants["tenant-1"] = model.TenantItem{TenantID: "tenant-1"}
	repo.keys["public-key"] = "tenant-1"

	result, err := svc.CreateConversation(context.Background(), conversationservice.CreateConversationParams{
		TenantAPIKey: "public-key",
		Message:      "Hello",
		Visitor:      conversationservice.VisitorParams{Name: "Visitor"},
	})
	if err != nil {
		t.Fatalf("CreateConversation error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/public/conversations/"+result.Conversation.ConversationID+"/messages", nil)
	req.Header.Set("X-Tenant-Key", "public-key")
	req.Header.Set("X-Visitor-Token", result.VisitorToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp dto.ListMessagesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Body != "Hello" {
		t.Fatalf("unexpected message body %s", resp.Messages[0].Body)
	}
}

func TestAgentCannotPostToForeignConversation(t *testing.T) {
	handler, svc, repo := setupConversationTestHandler(t)
	tenantA := "tenant-a"
	tenantB := "tenant-b"
	repo.tenants[tenantA] = model.TenantItem{TenantID: tenantA}
	repo.tenants[tenantB] = model.TenantItem{TenantID: tenantB}
	repo.keys["key-a"] = tenantA
	repo.keys["key-b"] = tenantB

	repo.users[model.TenantScopedPK(tenantA, "user-a")] = model.UserItem{
		PK:       model.TenantScopedPK(tenantA, "user-a"),
		TenantID: tenantA,
		UserID:   "user-a",
		Email:    "user@example.com",
	}

	conv := model.ConversationItem{
		PK:             model.ConversationPK(tenantB, "conv-1"),
		ConversationID: "conv-1",
		TenantID:       tenantB,
		VisitorID:      "visitor-1",
		Status:         model.ConversationStatusOpen,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
		LastMessageAt:  time.Now().UTC().Format(time.RFC3339),
	}
	repo.conversations[conv.PK] = conv

	token, err := internaljwt.CreateToken(internaljwt.User{Id: "user-a", TenantID: tenantA, Email: "user@example.com"}, internaljwt.RoleUser, time.Now().Add(time.Hour).Unix())
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	payload := dto.PostAgentMessageRequest{Body: "Agent reply"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/conversations/conv-1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 404 or 401, got %d", rec.Code)
	}

	_ = svc // ensure svc referenced for coverage
}
