package endpoints

import (
	"bytes"
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/middleware"
	"chat-app-backend/internal/dto"
	internaljwt "chat-app-backend/internal/jwt"
	"chat-app-backend/internal/model"
	"chat-app-backend/internal/queue"
	tenantservice "chat-app-backend/internal/service/tenant"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type tenantTestRepository struct {
	mu           sync.Mutex
	tenants      map[string]model.TenantItem
	users        map[string]model.UserItem
	usersByEmail map[string]map[string]string
	invites      map[string]model.TenantInviteItem
	keys         map[string]map[string]model.TenantAPIKeyItem
}

func newTenantTestRepository() *tenantTestRepository {
	return &tenantTestRepository{
		tenants:      make(map[string]model.TenantItem),
		users:        make(map[string]model.UserItem),
		usersByEmail: make(map[string]map[string]string),
		invites:      make(map[string]model.TenantInviteItem),
		keys:         make(map[string]map[string]model.TenantAPIKeyItem),
	}
}

func (m *tenantTestRepository) GetTenant(ctx context.Context, tenantID string) (model.TenantItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenant, ok := m.tenants[tenantID]
	if !ok {
		return model.TenantItem{}, tenantservice.ErrNotFound
	}
	return tenant, nil
}

func (m *tenantTestRepository) UpdateTenantName(ctx context.Context, tenantID, name string) (model.TenantItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenant, ok := m.tenants[tenantID]
	if !ok {
		return model.TenantItem{}, tenantservice.ErrNotFound
	}
	tenant.Name = name
	m.tenants[tenantID] = tenant
	return tenant, nil
}

func (m *tenantTestRepository) GetUser(ctx context.Context, tenantID, userID string) (model.UserItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.TenantScopedPK(tenantID, userID)
	user, ok := m.users[pk]
	if !ok {
		return model.UserItem{}, tenantservice.ErrNotFound
	}
	return user, nil
}

func (m *tenantTestRepository) ListUsersByTenant(ctx context.Context, tenantID string) ([]model.UserItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.UserItem, 0)
	for _, user := range m.users {
		if user.TenantID == tenantID {
			out = append(out, user)
		}
	}
	return out, nil
}

func (m *tenantTestRepository) ListUsersByEmail(ctx context.Context, email string) ([]model.UserItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	users := make([]model.UserItem, 0)
	for _, user := range m.users {
		if user.Email == email {
			users = append(users, user)
		}
	}
	return users, nil
}

func (m *tenantTestRepository) CreateUser(ctx context.Context, user model.UserItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.PK] = user
	if _, ok := m.usersByEmail[user.TenantID]; !ok {
		m.usersByEmail[user.TenantID] = make(map[string]string)
	}
	m.usersByEmail[user.TenantID][user.Email] = user.PK
	return nil
}

func (m *tenantTestRepository) FindUserByEmail(ctx context.Context, tenantID, email string) (model.UserItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantUsers, ok := m.usersByEmail[tenantID]
	if !ok {
		return model.UserItem{}, tenantservice.ErrNotFound
	}
	pk, ok := tenantUsers[email]
	if !ok {
		return model.UserItem{}, tenantservice.ErrNotFound
	}
	user, ok := m.users[pk]
	if !ok {
		return model.UserItem{}, tenantservice.ErrNotFound
	}
	return user, nil
}

func (m *tenantTestRepository) CreateInvite(ctx context.Context, invite model.TenantInviteItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invites[invite.Token] = invite
	return nil
}

func (m *tenantTestRepository) GetInvite(ctx context.Context, token string) (model.TenantInviteItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	invite, ok := m.invites[token]
	if !ok {
		return model.TenantInviteItem{}, tenantservice.ErrNotFound
	}
	return invite, nil
}

func (m *tenantTestRepository) ListInvitesByEmail(ctx context.Context, email string) ([]model.TenantInviteItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	invites := make([]model.TenantInviteItem, 0)
	for _, invite := range m.invites {
		if invite.Email == email {
			invites = append(invites, invite)
		}
	}
	return invites, nil
}

func (m *tenantTestRepository) FindActiveInvite(ctx context.Context, tenantID, email string) (model.TenantInviteItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	target := model.TenantScopedPK(tenantID, email)
	for _, invite := range m.invites {
		if invite.TenantEmail == target && invite.Status == "pending" {
			return invite, nil
		}
	}
	return model.TenantInviteItem{}, tenantservice.ErrNotFound
}

func (m *tenantTestRepository) UpdateInviteStatus(ctx context.Context, token, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	invite, ok := m.invites[token]
	if !ok {
		return tenantservice.ErrNotFound
	}
	invite.Status = status
	m.invites[token] = invite
	return nil
}

func (m *tenantTestRepository) DeleteInvite(ctx context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.invites, token)
	return nil
}

func (m *tenantTestRepository) ListTenantAPIKeys(ctx context.Context, tenantID string) ([]model.TenantAPIKeyItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenantKeys := m.keys[tenantID]
	if tenantKeys == nil {
		return []model.TenantAPIKeyItem{}, nil
	}

	keys := make([]model.TenantAPIKeyItem, 0, len(tenantKeys))
	for _, key := range tenantKeys {
		keys = append(keys, key)
	}
	return keys, nil
}

func (m *tenantTestRepository) CreateTenantAPIKey(ctx context.Context, item model.TenantAPIKeyItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.keys[item.TenantID]; !ok {
		m.keys[item.TenantID] = make(map[string]model.TenantAPIKeyItem)
	}
	m.keys[item.TenantID][item.KeyID] = item
	return nil
}

func (m *tenantTestRepository) DeleteTenantAPIKey(ctx context.Context, tenantID, keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tenantKeys, ok := m.keys[tenantID]; ok {
		delete(tenantKeys, keyID)
	}
	return nil
}

func (m *tenantTestRepository) GetTenantAPIKey(ctx context.Context, tenantID, keyID string) (model.TenantAPIKeyItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tenantKeys, ok := m.keys[tenantID]; ok {
		if key, ok := tenantKeys[keyID]; ok {
			return key, nil
		}
	}
	return model.TenantAPIKeyItem{}, tenantservice.ErrNotFound
}

func tenantFixedTime() time.Time {
	return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
}

func setupTenantHandler(t *testing.T, repo tenantservice.Repository) (http.Handler, func()) {
	t.Helper()

	internaljwt.RoleSecrets[internaljwt.RoleUser] = "test-secret"

	service := tenantservice.NewWithRepository(repo, tenantFixedTime)
	tenantEndpoints := NewTenantEndpoints(service)

	queueManager := queue.NewRequestQueueManager(10, 1)
	server := api.NewAPIServer(":0", queueManager, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/tenant", server.MakeHTTPHandleFunc(tenantEndpoints.UpdateTenant, middleware.ValidateUserJWT))
	mux.HandleFunc("/api/tenant/users", server.MakeHTTPHandleFunc(tenantEndpoints.AddTenantUser, middleware.ValidateUserJWT))
	mux.HandleFunc("/api/tenant/api-keys", server.MakeHTTPHandleFunc(tenantEndpoints.TenantAPIKeys, middleware.ValidateUserJWT))
	mux.HandleFunc("/api/tenant/invites/accept", server.MakeHTTPHandleFunc(tenantEndpoints.AcceptInvite))
	mux.HandleFunc("/api/tenant/invites/pending", server.MakeHTTPHandleFunc(tenantEndpoints.ListPendingInvites, middleware.ValidateUserJWT))

	return mux, func() {
		queueManager.Shutdown()
	}
}

func bearer(t *testing.T, user model.UserItem) string {
	t.Helper()
	token, err := internaljwt.CreateToken(internaljwt.User{
		Id:           user.UserID,
		TenantID:     user.TenantID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
	}, internaljwt.RoleUser, 0)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	return "Bearer " + token
}

func TestTenantUpdateNameEndpoint(t *testing.T) {
	repo := newTenantTestRepository()

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Old Name",
		Plan:     "starter",
		Seats:    1,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	body := map[string]string{"name": "New Name"}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/tenant", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearer(t, owner))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp dto.TenantResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Name != "New Name" {
		t.Fatalf("expected updated name, got %s", resp.Name)
	}
	if resp.RemainingSeats == nil || *resp.RemainingSeats != 1 {
		t.Fatalf("expected remaining seats 1, got %v", resp.RemainingSeats)
	}
}

func TestTenantListAPIKeysEndpoint(t *testing.T) {
	repo := newTenantTestRepository()

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	repo.keys[tenant.TenantID] = map[string]model.TenantAPIKeyItem{
		"key-1": {
			TenantID:  tenant.TenantID,
			KeyID:     "key-1",
			APIKey:    "pingy_ONE",
			CreatedAt: tenantFixedTime().Add(-time.Hour).Format(time.RFC3339),
		},
		"key-2": {
			TenantID:  tenant.TenantID,
			KeyID:     "key-2",
			APIKey:    "pingy_TWO",
			CreatedAt: tenantFixedTime().Format(time.RFC3339),
		},
	}

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/tenant/api-keys", nil)
	req.Header.Set("Authorization", bearer(t, owner))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp dto.TenantAPIKeyListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(resp.Keys))
	}
	if resp.Keys[0].APIKey != "pingy_TWO" {
		t.Fatalf("expected newest key first, got %s", resp.Keys[0].APIKey)
	}
}

func TestTenantCreateAPIKeyEndpoint(t *testing.T) {
	repo := newTenantTestRepository()

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/tenant/api-keys", nil)
	req.Header.Set("Authorization", bearer(t, owner))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp dto.CreateTenantAPIKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Key.APIKey == "" {
		t.Fatalf("expected api key")
	}
	if resp.Key.KeyID == "" {
		t.Fatalf("expected key id")
	}
	if !strings.HasPrefix(resp.Key.APIKey, "pingy_") {
		t.Fatalf("expected api key prefix, got %s", resp.Key.APIKey)
	}
}

func TestTenantDeleteAPIKeyEndpoint(t *testing.T) {
	repo := newTenantTestRepository()

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	repo.CreateTenantAPIKey(context.Background(), model.TenantAPIKeyItem{
		TenantID:  tenant.TenantID,
		KeyID:     "key-1",
		APIKey:    "pingy_KEY",
		CreatedAt: tenantFixedTime().Format(time.RFC3339),
	})

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	payload := dto.DeleteTenantAPIKeyRequest{KeyID: "key-1"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodDelete, "/api/tenant/api-keys", bytes.NewReader(body))
	req.Header.Set("Authorization", bearer(t, owner))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if _, err := repo.GetTenantAPIKey(context.Background(), tenant.TenantID, "key-1"); !errors.Is(err, tenantservice.ErrNotFound) {
		t.Fatalf("expected key to be removed, got %v", err)
	}
}

func TestTenantAddUserEndpoint(t *testing.T) {
	repo := newTenantTestRepository()

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	body := map[string]string{
		"name":     "Agent",
		"email":    "agent@example.com",
		"password": "password",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/tenant/users", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearer(t, owner))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp dto.AddTenantUserResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.User == nil {
		t.Fatalf("expected created user details, got %+v", resp)
	}
	if resp.User.Email != "agent@example.com" {
		t.Fatalf("expected created user email, got %s", resp.User.Email)
	}
	if resp.Invite != nil {
		t.Fatalf("expected no invite, got %+v", resp.Invite)
	}
	if resp.RemainingSeats != 1 {
		t.Fatalf("expected remaining seats 1, got %d", resp.RemainingSeats)
	}

	if _, err := repo.FindUserByEmail(context.Background(), tenant.TenantID, "agent@example.com"); err != nil {
		t.Fatalf("expected user to be stored: %v", err)
	}
}

func TestTenantAddUserSeatLimit(t *testing.T) {
	repo := newTenantTestRepository()

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	member := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "member-1"),
		TenantID:     tenant.TenantID,
		UserID:       "member-1",
		Email:        "member@example.com",
		Name:         "Member",
		Role:         "member",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), member)

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	body := map[string]string{
		"name":     "Agent",
		"email":    "agent@example.com",
		"password": "password",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/tenant/users", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearer(t, owner))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantAddUserRejectsInvalidRole(t *testing.T) {
	repo := newTenantTestRepository()

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	body := map[string]string{
		"name":     "Agent",
		"email":    "agent@example.com",
		"password": "password",
		"role":     "owner",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/tenant/users", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearer(t, owner))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantAddUserReturnsInviteForExistingAccount(t *testing.T) {
	repo := newTenantTestRepository()

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	existing := model.UserItem{
		PK:           model.TenantScopedPK("tenant-2", "existing"),
		TenantID:     "tenant-2",
		UserID:       "existing",
		Email:        "agent@example.com",
		Name:         "Agent",
		Role:         "member",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), existing)

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	body := map[string]string{
		"name":  "Agent",
		"email": "agent@example.com",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/tenant/users", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearer(t, owner))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp dto.AddTenantUserResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.User != nil {
		t.Fatalf("expected invite-only response, got user %+v", resp.User)
	}
	if resp.Invite == nil {
		t.Fatal("expected invite response")
	}
	if resp.Invite.Email != "agent@example.com" {
		t.Fatalf("unexpected invite email %s", resp.Invite.Email)
	}
	if resp.RemainingSeats != 2 {
		t.Fatalf("expected remaining seats 2, got %d", resp.RemainingSeats)
	}
}

func TestTenantAcceptInviteEndpoint(t *testing.T) {
	repo := newTenantTestRepository()

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	existing := model.UserItem{
		PK:           model.TenantScopedPK("tenant-2", "existing"),
		TenantID:     "tenant-2",
		UserID:       "existing",
		Email:        "agent@example.com",
		Name:         "Agent",
		Role:         "member",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), existing)

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	inviteReq := httptest.NewRequest(http.MethodPost, "/api/tenant/users", bytes.NewReader([]byte(`{"name":"Agent","email":"agent@example.com"}`)))
	inviteReq.Header.Set("Content-Type", "application/json")
	inviteReq.Header.Set("Authorization", bearer(t, owner))

	inviteRec := httptest.NewRecorder()
	handler.ServeHTTP(inviteRec, inviteReq)

	if inviteRec.Code != http.StatusCreated {
		t.Fatalf("expected invite creation status 201, got %d: %s", inviteRec.Code, inviteRec.Body.String())
	}

	var inviteResp dto.AddTenantUserResponse
	if err := json.NewDecoder(inviteRec.Body).Decode(&inviteResp); err != nil {
		t.Fatalf("decode invite response: %v", err)
	}
	if inviteResp.Invite == nil {
		t.Fatal("expected invite data")
	}

	acceptBody, _ := json.Marshal(map[string]string{
		"token": inviteResp.Invite.Token,
	})
	acceptReq := httptest.NewRequest(http.MethodPost, "/api/tenant/invites/accept", bytes.NewReader(acceptBody))
	acceptReq.Header.Set("Content-Type", "application/json")

	acceptRec := httptest.NewRecorder()
	handler.ServeHTTP(acceptRec, acceptReq)

	if acceptRec.Code != http.StatusCreated {
		t.Fatalf("expected accept status 201, got %d: %s", acceptRec.Code, acceptRec.Body.String())
	}

	var acceptResp dto.AcceptInviteResponse
	if err := json.NewDecoder(acceptRec.Body).Decode(&acceptResp); err != nil {
		t.Fatalf("decode accept response: %v", err)
	}
	if acceptResp.User.Email != "agent@example.com" {
		t.Fatalf("unexpected user email %s", acceptResp.User.Email)
	}
	if acceptResp.Tenant.TenantID != tenant.TenantID {
		t.Fatalf("unexpected tenant id %s", acceptResp.Tenant.TenantID)
	}
}

func TestTenantListPendingInvites(t *testing.T) {
	repo := newTenantTestRepository()

	tenantA := model.TenantItem{
		TenantID: "tenant-a",
		Name:     "Tenant A",
		Plan:     "starter",
		Seats:    3,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	tenantB := model.TenantItem{
		TenantID: "tenant-b",
		Name:     "Tenant B",
		Plan:     "growth",
		Seats:    10,
		Created:  tenantFixedTime().Add(-2 * time.Hour).Format(time.RFC3339),
	}
	repo.tenants[tenantA.TenantID] = tenantA
	repo.tenants[tenantB.TenantID] = tenantB

	invitee := model.UserItem{
		PK:        model.TenantScopedPK(tenantA.TenantID, "invitee"),
		TenantID:  tenantA.TenantID,
		UserID:    "invitee",
		Email:     "agent@example.com",
		Name:      "Invitee",
		Role:      "member",
		Status:    "active",
		CreatedAt: tenantFixedTime().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), invitee)

	validSoon := model.TenantInviteItem{
		Token:       "valid-soon",
		TenantID:    tenantA.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenantA.TenantID, "agent@example.com"),
		InvitedBy:   "owner-a",
		Role:        "admin",
		Status:      "pending",
		CreatedAt:   tenantFixedTime().Add(-time.Minute).Format(time.RFC3339),
		ExpiresAt:   tenantFixedTime().Add(2 * time.Hour).Format(time.RFC3339),
	}
	validLater := model.TenantInviteItem{
		Token:       "valid-later",
		TenantID:    tenantB.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenantB.TenantID, "agent@example.com"),
		InvitedBy:   "owner-b",
		Role:        "owner",
		Status:      "pending",
		CreatedAt:   tenantFixedTime().Add(-2 * time.Minute).Format(time.RFC3339),
		ExpiresAt:   tenantFixedTime().Add(3 * time.Hour).Format(time.RFC3339),
	}
	expired := model.TenantInviteItem{
		Token:       "expired",
		TenantID:    tenantB.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenantB.TenantID, "agent@example.com"),
		InvitedBy:   "owner-b",
		Role:        "admin",
		Status:      "pending",
		CreatedAt:   tenantFixedTime().Add(-4 * time.Hour).Format(time.RFC3339),
		ExpiresAt:   tenantFixedTime().Add(-time.Minute).Format(time.RFC3339),
	}
	other := model.TenantInviteItem{
		Token:       "other",
		TenantID:    tenantA.TenantID,
		Email:       "other@example.com",
		TenantEmail: model.TenantScopedPK(tenantA.TenantID, "other@example.com"),
		InvitedBy:   "owner-a",
		Role:        "admin",
		Status:      "pending",
		CreatedAt:   tenantFixedTime().Add(-time.Minute).Format(time.RFC3339),
		ExpiresAt:   tenantFixedTime().Add(2 * time.Hour).Format(time.RFC3339),
	}

	repo.invites[validSoon.Token] = validSoon
	repo.invites[validLater.Token] = validLater
	repo.invites[expired.Token] = expired
	repo.invites[other.Token] = other

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/tenant/invites/pending", nil)
	req.Header.Set("Authorization", bearer(t, invitee))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp dto.ListPendingInvitesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Invites) != 2 {
		t.Fatalf("expected 2 invites, got %d", len(resp.Invites))
	}
	if resp.Invites[0].Token != validSoon.Token || resp.Invites[1].Token != validLater.Token {
		t.Fatalf("expected ordered tokens [valid-soon, valid-later], got %+v", resp.Invites)
	}

	if resp.Invites[1].Role != "member" {
		t.Fatalf("expected sanitized role member, got %s", resp.Invites[1].Role)
	}

	if resp.Invites[0].Tenant.TenantID != tenantA.TenantID {
		t.Fatalf("expected tenant A in response, got %s", resp.Invites[0].Tenant.TenantID)
	}

	if repo.invites[expired.Token].Status != "expired" {
		t.Fatal("expected expired invite to be marked expired")
	}
}

func TestTenantListPendingInvitesRequiresAuth(t *testing.T) {
	repo := newTenantTestRepository()

	handler, cleanup := setupTenantHandler(t, repo)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/tenant/invites/pending", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}
