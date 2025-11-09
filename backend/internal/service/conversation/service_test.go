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

func (m *memoryRepository) UpdateConversationVisitorEmail(ctx context.Context, tenantID, conversationID, visitorEmail, updatedAt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.ConversationPK(tenantID, conversationID)
	conversation, ok := m.conversations[pk]
	if !ok {
		return ErrNotFound
	}
	conversation.VisitorEmail = visitorEmail
	conversation.UpdatedAt = updatedAt
	m.conversations[pk] = conversation
	return nil
}

func (m *memoryRepository) MarkConversationTenantStart(ctx context.Context, tenantID, conversationID, startedAt, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.ConversationPK(tenantID, conversationID)
	conversation, ok := m.conversations[pk]
	if !ok {
		return ErrNotFound
	}
	conversation.TenantStartedAt = startedAt
	conversation.TenantStartedBy = userID
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

func (m *memoryRepository) CountConversationsStartedBetween(ctx context.Context, tenantID string, start, end time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, c := range m.conversations {
		if c.TenantID != tenantID {
			continue
		}
		ts := parseTime(c.TenantStartedAt)
		if ts.IsZero() {
			continue
		}
		if (ts.Equal(start) || ts.After(start)) && ts.Before(end) {
			count++
		}
	}
	return count, nil
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

func TestAssignVisitorEmail(t *testing.T) {
	repo := newMemoryRepository()
	now := time.Date(2024, 1, 2, 15, 0, 0, 0, time.UTC)
	svc := NewWithRepository(repo, func() time.Time { return now })
	useTestSecret(t)

	tenantID := "tenant-123"
	visitorID := "visitor-123"
	conversationID := "conversation-123"

	repo.tenants[tenantID] = model.TenantItem{TenantID: tenantID}
	repo.visitors[model.VisitorPK(tenantID, visitorID)] = model.VisitorItem{
		PK:        model.VisitorPK(tenantID, visitorID),
		TenantID:  tenantID,
		VisitorID: visitorID,
		CreatedAt: now.Add(-time.Hour).Format(time.RFC3339),
	}
	repo.conversations[model.ConversationPK(tenantID, conversationID)] = model.ConversationItem{
		PK:             model.ConversationPK(tenantID, conversationID),
		ConversationID: conversationID,
		TenantID:       tenantID,
		VisitorID:      visitorID,
		Status:         model.ConversationStatusOpen,
		CreatedAt:      now.Add(-time.Hour).Format(time.RFC3339),
		UpdatedAt:      now.Add(-time.Hour).Format(time.RFC3339),
		LastMessageAt:  now.Add(-time.Hour).Format(time.RFC3339),
	}

	token, err := signVisitorToken(visitorTokenClaims{
		TenantID:       tenantID,
		ConversationID: conversationID,
		VisitorID:      visitorID,
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(24 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("failed to sign visitor token: %v", err)
	}

	result, err := svc.AssignVisitorEmail(context.Background(), token, "User@Example.com")
	if err != nil {
		t.Fatalf("AssignVisitorEmail error: %v", err)
	}

	if want := "user@example.com"; result.VisitorEmail != want {
		t.Fatalf("unexpected visitor email: got %s want %s", result.VisitorEmail, want)
	}

	storedConv, _ := repo.GetConversation(context.Background(), tenantID, conversationID)
	if storedConv.VisitorEmail != "user@example.com" {
		t.Fatalf("conversation not updated: %+v", storedConv)
	}

	storedVisitor, _ := repo.GetVisitor(context.Background(), tenantID, visitorID)
	if storedVisitor.Email != "user@example.com" {
		t.Fatalf("visitor not updated: %+v", storedVisitor)
	}
	if storedVisitor.LastSeenAt == "" {
		t.Fatalf("visitor last seen not set")
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

func TestGetConversationUsageCountsWithinPeriod(t *testing.T) {
	repo := newMemoryRepository()
	now := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	svc := NewWithRepository(repo, func() time.Time { return now })
	useTestSecret(t)

	tenantID := "tenant-99"
	userID := "user-99"
	repo.tenants[tenantID] = model.TenantItem{TenantID: tenantID}
	repo.users[model.TenantScopedPK(tenantID, userID)] = model.UserItem{
		PK:       model.TenantScopedPK(tenantID, userID),
		TenantID: tenantID,
		UserID:   userID,
	}

	addConversation := func(id string, created time.Time, started time.Time) {
		repo.conversations[model.ConversationPK(tenantID, id)] = model.ConversationItem{
			PK:             model.ConversationPK(tenantID, id),
			ConversationID: id,
			TenantID:       tenantID,
			VisitorID:      "visitor-" + id,
			Status:         model.ConversationStatusOpen,
			CreatedAt:      created.Format(time.RFC3339),
			UpdatedAt:      created.Format(time.RFC3339),
			LastMessageAt:  created.Format(time.RFC3339),
			TenantStartedAt: func() string {
				if started.IsZero() {
					return ""
				}
				return started.Format(time.RFC3339)
			}(),
			TenantStartedBy: func() string {
				if started.IsZero() {
					return ""
				}
				return userID
			}(),
		}
	}

	addConversation("in-range-1", time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 3, 1, 2, 0, 0, 0, time.UTC))
	addConversation("in-range-2", time.Date(2024, 3, 30, 10, 0, 0, 0, time.UTC), time.Date(2024, 3, 30, 10, 30, 0, 0, time.UTC))
	addConversation("out-of-range", time.Date(2024, 2, 28, 23, 59, 0, 0, time.UTC), time.Date(2024, 2, 28, 23, 59, 0, 0, time.UTC))
	addConversation("not-started", time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC), time.Time{})

	start := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	result, err := svc.GetConversationUsage(context.Background(), Identity{
		UserID:   userID,
		TenantID: tenantID,
	}, start, end)
	if err != nil {
		t.Fatalf("GetConversationUsage returned error: %v", err)
	}
	if result.StartedCount != 2 {
		t.Fatalf("expected 2 conversations, got %d", result.StartedCount)
	}
	if !result.PeriodStart.Equal(start) || !result.PeriodEnd.Equal(end) {
		t.Fatalf("unexpected period %+v", result)
	}
}

func TestGetConversationUsageRequiresValidIdentity(t *testing.T) {
	repo := newMemoryRepository()
	svc := NewWithRepository(repo, time.Now)

	_, err := svc.GetConversationUsage(context.Background(), Identity{}, time.Now(), time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected error for missing identity")
	}
	svcErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if svcErr.Code != ErrorCodeUnauthorized {
		t.Fatalf("expected unauthorized, got %s", svcErr.Code)
	}
}

func TestPostAgentMessageMarksTenantStart(t *testing.T) {
	repo := newMemoryRepository()
	now := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := NewWithRepository(repo, func() time.Time { return now })
	useTestSecret(t)

	tenantID := "tenant-tenant"
	userID := "user-agent"
	repo.tenants[tenantID] = model.TenantItem{TenantID: tenantID}
	repo.users[model.TenantScopedPK(tenantID, userID)] = model.UserItem{
		PK:       model.TenantScopedPK(tenantID, userID),
		TenantID: tenantID,
		UserID:   userID,
	}

	conversationID := "conv-tenant"
	repo.conversations[model.ConversationPK(tenantID, conversationID)] = model.ConversationItem{
		PK:             model.ConversationPK(tenantID, conversationID),
		ConversationID: conversationID,
		TenantID:       tenantID,
		VisitorID:      "visitor-1",
		Status:         model.ConversationStatusOpen,
		CreatedAt:      now.Add(-time.Hour).Format(time.RFC3339),
		UpdatedAt:      now.Add(-time.Hour).Format(time.RFC3339),
		LastMessageAt:  now.Add(-time.Hour).Format(time.RFC3339),
	}

	result, err := svc.PostAgentMessage(context.Background(), Identity{
		UserID:   userID,
		TenantID: tenantID,
	}, conversationID, "Hello from agent")
	if err != nil {
		t.Fatalf("PostAgentMessage error: %v", err)
	}

	if result.Conversation.TenantStartedAt == "" {
		t.Fatal("expected tenant started timestamp to be set")
	}
	if result.Conversation.TenantStartedBy != userID {
		t.Fatalf("expected tenant started by %s, got %s", userID, result.Conversation.TenantStartedBy)
	}

	stored := repo.conversations[model.ConversationPK(tenantID, conversationID)]
	if stored.TenantStartedAt == "" {
		t.Fatalf("expected stored conversation to update tenantStartedAt")
	}

	firstStart := stored.TenantStartedAt

	// send another message - should not change start time
	if _, err := svc.PostAgentMessage(context.Background(), Identity{
		UserID:   userID,
		TenantID: tenantID,
	}, conversationID, "Second reply"); err != nil {
		t.Fatalf("PostAgentMessage second call error: %v", err)
	}

	stored = repo.conversations[model.ConversationPK(tenantID, conversationID)]
	if stored.TenantStartedAt != firstStart {
		t.Fatalf("expected tenantStartedAt to remain %s, got %s", firstStart, stored.TenantStartedAt)
	}
}
