package conversation

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"chat-app-backend/internal/model"
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
		return model.TenantItem{}, ErrNotFound
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
	return model.TenantItem{}, ErrNotFound
}

func (m *memoryRepository) GetUser(ctx context.Context, tenantID, userID string) (model.UserItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.TenantScopedPK(tenantID, userID)
	user, ok := m.users[pk]
	if !ok {
		return model.UserItem{}, ErrNotFound
	}
	return user, nil
}

func (m *memoryRepository) GetVisitor(ctx context.Context, tenantID, visitorID string) (model.VisitorItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.VisitorPK(tenantID, visitorID)
	visitor, ok := m.visitors[pk]
	if !ok {
		return model.VisitorItem{}, ErrNotFound
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
	conversation, ok := m.conversations[pk]
	if !ok {
		return ErrNotFound
	}
	conversation.UpdatedAt = updatedAt
	conversation.LastMessageAt = lastMessageAt
	if assignedUserID != nil {
		conversation.AssignedUserID = *assignedUserID
	}
	m.conversations[pk] = conversation
	return nil
}

func (m *memoryRepository) GetConversation(ctx context.Context, tenantID, conversationID string) (model.ConversationItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.ConversationPK(tenantID, conversationID)
	conversation, ok := m.conversations[pk]
	if !ok {
		return model.ConversationItem{}, ErrNotFound
	}
	return conversation, nil
}

func (m *memoryRepository) ListConversations(ctx context.Context, tenantID string, limit int) ([]model.ConversationItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]model.ConversationItem, 0)
	for _, c := range m.conversations {
		if c.TenantID == tenantID {
			items = append(items, c)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastMessageAt > items[j].LastMessageAt
	})
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
		ti := parseTime(items[i].CreatedAt)
		tj := parseTime(items[j].CreatedAt)
		return ti.Before(tj)
	})
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}
	return items, nil
}

func useTestSecret(t *testing.T) {
	t.Helper()
	original := make([]byte, len(visitorTokenSecret))
	copy(original, visitorTokenSecret)
	SetVisitorTokenSecret([]byte("test-secret"))
	t.Cleanup(func() {
		SetVisitorTokenSecret(original)
	})
}

func TestCreateConversation(t *testing.T) {
	repo := newMemoryRepository()
	now := time.Date(2024, 1, 2, 15, 0, 0, 0, time.UTC)
	svc := NewWithRepository(repo, func() time.Time { return now })
	useTestSecret(t)

	tenantID := "tenant-1"
	repo.tenants[tenantID] = model.TenantItem{TenantID: tenantID}
	repo.keys["api-key-1"] = tenantID

	result, err := svc.CreateConversation(context.Background(), CreateConversationParams{
		TenantAPIKey: "api-key-1",
		Visitor: VisitorParams{
			Name:  "Visitor",
			Email: "visitor@example.com",
		},
		Message: "Hello there",
	})
	if err != nil {
		t.Fatalf("CreateConversation error: %v", err)
	}

	if result.Conversation.TenantID != tenantID {
		t.Fatalf("unexpected tenant id %s", result.Conversation.TenantID)
	}
	if result.Message.Body != "Hello there" {
		t.Fatalf("unexpected message body %s", result.Message.Body)
	}
	if result.VisitorToken == "" {
		t.Fatal("expected visitor token")
	}
}

func TestPostVisitorMessageRejectsInvalidToken(t *testing.T) {
	repo := newMemoryRepository()
	now := time.Date(2024, 1, 2, 15, 0, 0, 0, time.UTC)
	svc := NewWithRepository(repo, func() time.Time { return now })
	useTestSecret(t)

	_, err := svc.PostVisitorMessage(context.Background(), "invalid.token", "Hello")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	svcErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if svcErr.Code != ErrorCodeUnauthorized {
		t.Fatalf("expected unauthorized, got %s", svcErr.Code)
	}
}

func TestPostAgentMessageRejectsCrossTenant(t *testing.T) {
	repo := newMemoryRepository()
	now := time.Date(2024, 1, 2, 15, 0, 0, 0, time.UTC)
	svc := NewWithRepository(repo, func() time.Time { return now })
	useTestSecret(t)

	tenantA := "tenant-a"
	tenantB := "tenant-b"
	repo.tenants[tenantA] = model.TenantItem{TenantID: tenantA}
	repo.tenants[tenantB] = model.TenantItem{TenantID: tenantB}
	repo.keys["api-key-a"] = tenantA
	repo.keys["api-key-b"] = tenantB

	userA := model.UserItem{
		PK:       model.TenantScopedPK(tenantA, "user-a"),
		TenantID: tenantA,
		UserID:   "user-a",
	}
	repo.users[userA.PK] = userA

	conversation := model.ConversationItem{
		PK:             model.ConversationPK(tenantB, "conv-1"),
		ConversationID: "conv-1",
		TenantID:       tenantB,
		VisitorID:      "visitor-1",
		Status:         model.ConversationStatusOpen,
		CreatedAt:      now.Format(time.RFC3339),
		UpdatedAt:      now.Format(time.RFC3339),
		LastMessageAt:  now.Format(time.RFC3339),
	}
	repo.conversations[conversation.PK] = conversation

	_, err := svc.PostAgentMessage(context.Background(), Identity{
		UserID:   "user-a",
		TenantID: tenantA,
	}, conversation.ConversationID, "Hello")
	if err == nil {
		t.Fatal("expected error for cross-tenant access")
	}
	svcErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if svcErr.Code != ErrorCodeNotFound && svcErr.Code != ErrorCodeUnauthorized {
		t.Fatalf("expected not found or unauthorized, got %s", svcErr.Code)
	}
}

func TestListVisitorMessages(t *testing.T) {
	repo := newMemoryRepository()
	now := time.Date(2024, 1, 2, 15, 0, 0, 0, time.UTC)
	svc := NewWithRepository(repo, func() time.Time { return now })
	useTestSecret(t)

	tenantID := "tenant-1"
	repo.tenants[tenantID] = model.TenantItem{TenantID: tenantID}
	repo.keys["api-key-1"] = tenantID

	result, err := svc.CreateConversation(context.Background(), CreateConversationParams{
		TenantAPIKey: "api-key-1",
		Message:      "Hello",
		Visitor:      VisitorParams{Name: "Visitor"},
	})
	if err != nil {
		t.Fatalf("CreateConversation error: %v", err)
	}

	list, err := svc.ListVisitorMessages(context.Background(), result.VisitorToken, result.Conversation.ConversationID, 50)
	if err != nil {
		t.Fatalf("ListVisitorMessages error: %v", err)
	}
	if len(list.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(list.Messages))
	}
	if list.Messages[0].Body != "Hello" {
		t.Fatalf("unexpected message body %s", list.Messages[0].Body)
	}
}

func TestListVisitorMessagesRejectsMismatchedToken(t *testing.T) {
	repo := newMemoryRepository()
	now := time.Date(2024, 1, 2, 15, 0, 0, 0, time.UTC)
	svc := NewWithRepository(repo, func() time.Time { return now })
	useTestSecret(t)

	tenantID := "tenant-1"
	repo.tenants[tenantID] = model.TenantItem{TenantID: tenantID}
	repo.keys["api-key-1"] = tenantID

	result, err := svc.CreateConversation(context.Background(), CreateConversationParams{
		TenantAPIKey: "api-key-1",
		Message:      "Hello",
		Visitor:      VisitorParams{Name: "Visitor"},
	})
	if err != nil {
		t.Fatalf("CreateConversation error: %v", err)
	}

	_, err = svc.ListVisitorMessages(context.Background(), result.VisitorToken, "other-conv", 50)
	if err == nil {
		t.Fatal("expected error for mismatched conversation")
	}
	svcErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if svcErr.Code != ErrorCodeForbidden {
		t.Fatalf("expected forbidden, got %s", svcErr.Code)
	}
}
