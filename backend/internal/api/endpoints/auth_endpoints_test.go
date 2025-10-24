package endpoints

import (
	"bytes"
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/middleware"
	"chat-app-backend/internal/dto"
	internaljwt "chat-app-backend/internal/jwt"
	"chat-app-backend/internal/model"
	"chat-app-backend/internal/queue"
	authsvc "chat-app-backend/internal/service/auth"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type testRepository struct {
	mu           sync.Mutex
	tenants      map[string]model.TenantItem
	users        map[string]model.UserItem
	usersByEmail map[string]map[string]string
}

func newTestRepository() *testRepository {
	return &testRepository{
		tenants:      make(map[string]model.TenantItem),
		users:        make(map[string]model.UserItem),
		usersByEmail: make(map[string]map[string]string),
	}
}

func (m *testRepository) CreateTenant(ctx context.Context, tenant model.TenantItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tenants[tenant.TenantID] = tenant
	return nil
}

func (m *testRepository) CreateUser(ctx context.Context, user model.UserItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.PK] = user
	if _, ok := m.usersByEmail[user.TenantID]; !ok {
		m.usersByEmail[user.TenantID] = make(map[string]string)
	}
	m.usersByEmail[user.TenantID][user.Email] = user.PK
	return nil
}

func (m *testRepository) FindUserByEmail(ctx context.Context, tenantID, email string) (model.UserItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantUsers, ok := m.usersByEmail[tenantID]
	if !ok {
		return model.UserItem{}, authsvc.ErrNotFound
	}

	pk, ok := tenantUsers[email]
	if !ok {
		return model.UserItem{}, authsvc.ErrNotFound
	}

	user, ok := m.users[pk]
	if !ok {
		return model.UserItem{}, authsvc.ErrNotFound
	}

	return user, nil
}

func (m *testRepository) ListUsersByEmail(ctx context.Context, email string) ([]model.UserItem, error) {
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

func (m *testRepository) GetTenant(ctx context.Context, tenantID string) (model.TenantItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenant, ok := m.tenants[tenantID]
	if !ok {
		return model.TenantItem{}, authsvc.ErrNotFound
	}
	return tenant, nil
}

func (m *testRepository) GetUser(ctx context.Context, tenantID, userID string) (model.UserItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := model.TenantScopedPK(tenantID, userID)
	user, ok := m.users[pk]
	if !ok {
		return model.UserItem{}, authsvc.ErrNotFound
	}
	return user, nil
}

func fixedTime() time.Time {
	return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
}

func setupTestJWT(t *testing.T) {
	t.Helper()
	internaljwt.RoleSecrets[internaljwt.RoleUser] = "test-secret"
	authsvc.SetTokenIssuer(func(user internaljwt.User, role internaljwt.Role, validUntil int64) (internaljwt.TokenResponse, error) {
		token, err := internaljwt.CreateToken(user, role, validUntil)
		if err != nil {
			return internaljwt.TokenResponse{}, err
		}
		return internaljwt.TokenResponse{
			AccessToken: token,
		}, nil
	})
	t.Cleanup(func() {
		authsvc.SetTokenIssuer(nil)
	})
}

func setupAuthHandler(t *testing.T, svc *authsvc.Service) (http.Handler, func()) {
	t.Helper()

	authEndpoints := &authEndpoints{service: svc}

	queueManager := queue.NewRequestQueueManager(10, 1)

	server := api.NewAPIServer(":0", queueManager, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/register", server.MakeHTTPHandleFunc(authEndpoints.Register))
	mux.HandleFunc("/api/auth/login", server.MakeHTTPHandleFunc(authEndpoints.Login))
	mux.HandleFunc("/api/auth/me", server.MakeHTTPHandleFunc(authEndpoints.Me, middleware.ValidateUserJWT))
	mux.HandleFunc("/api/auth/switch", server.MakeHTTPHandleFunc(authEndpoints.Switch, middleware.ValidateUserJWT))

	return mux, func() {
		queueManager.Shutdown()
	}
}

func doJSONRequest[T any](t *testing.T, handler http.Handler, method, target string, body interface{}, headers map[string]string, expectedStatus int) T {
	t.Helper()

	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		payload = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, target, payload)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, rec.Code, rec.Body.String())
	}

	var result T
	if expectedStatus != http.StatusNoContent {
		if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}

	return result
}

func TestAuthEndpointsEndToEnd(t *testing.T) {
	setupTestJWT(t)
	repo := newTestRepository()
	service := authsvc.NewWithRepository(repo, fixedTime)

	handler, cleanup := setupAuthHandler(t, service)
	defer cleanup()

	registerPayload := map[string]interface{}{
		"tenantName": "Acme Corp",
		"plan":       "pro",
		"seats":      5,
		"name":       "Jane Owner",
		"email":      "owner@example.com",
		"password":   "Sup3rS3cret!",
	}

	registerResp := doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/register", registerPayload, nil, http.StatusCreated)

	if registerResp.Tenant.Plan != "starter" {
		t.Fatalf("expected plan starter, got %s", registerResp.Tenant.Plan)
	}

	if registerResp.Tenant.Seats != 1 {
		t.Fatalf("expected seats 1, got %d", registerResp.Tenant.Seats)
	}

	if len(registerResp.Tenants) != 1 || !registerResp.Tenants[0].IsDefault {
		t.Fatalf("expected single default membership, got %#v", registerResp.Tenants)
	}

	loginPayload := map[string]interface{}{
		"tenantId": registerResp.Tenant.TenantID,
		"email":    registerResp.User.Email,
		"password": "Sup3rS3cret!",
	}

	loginResp := doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/login", loginPayload, nil, http.StatusOK)

	if loginResp.AccessToken == "" {
		t.Fatal("expected access token in login response")
	}

	if len(loginResp.Tenants) != 1 || !loginResp.Tenants[0].IsDefault {
		t.Fatalf("expected single default membership after login, got %#v", loginResp.Tenants)
	}

	meHeaders := map[string]string{
		"Authorization": "Bearer " + loginResp.AccessToken,
	}

	meResp := doJSONRequest[dto.MeResponse](t, handler, http.MethodGet, "/api/auth/me", nil, meHeaders, http.StatusOK)

	if meResp.User.Email != registerResp.User.Email {
		t.Fatalf("expected email %s, got %s", registerResp.User.Email, meResp.User.Email)
	}

	if meResp.Tenant.TenantID != registerResp.Tenant.TenantID {
		t.Fatalf("expected tenant ID %s, got %s", registerResp.Tenant.TenantID, meResp.Tenant.TenantID)
	}
}

func TestAuthRegisterUsesDefaultSeatsWhenProvided(t *testing.T) {
	setupTestJWT(t)
	repo := newTestRepository()
	service := authsvc.NewWithRepository(repo, fixedTime)

	handler, cleanup := setupAuthHandler(t, service)
	defer cleanup()

	payload := map[string]interface{}{
		"tenantName": "Acme Corp",
		"plan":       "starter",
		"seats":      10,
		"name":       "Jane Owner",
		"email":      "owner@example.com",
		"password":   "Sup3rS3cret!",
	}

	resp := doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/register", payload, nil, http.StatusCreated)

	if resp.Tenant.Seats != 1 {
		t.Fatalf("expected seats 1, got %d", resp.Tenant.Seats)
	}

	if resp.Tenant.Plan != "starter" {
		t.Fatalf("expected plan starter, got %s", resp.Tenant.Plan)
	}

	if len(resp.Tenants) != 1 || !resp.Tenants[0].IsDefault {
		t.Fatalf("expected single default membership, got %#v", resp.Tenants)
	}
}

func TestAuthLoginListsMultipleTenants(t *testing.T) {
	setupTestJWT(t)
	repo := newTestRepository()
	service := authsvc.NewWithRepository(repo, fixedTime)

	handler, cleanup := setupAuthHandler(t, service)
	defer cleanup()

	basePayload := map[string]interface{}{
		"tenantName": "Acme Corp",
		"name":       "Jane Owner",
		"email":      "owner@example.com",
		"password":   "Sup3rS3cret!",
	}

	doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/register", basePayload, nil, http.StatusCreated)

	secondPayload := map[string]interface{}{
		"tenantName": "Beta Corp",
		"name":       "Jane Owner",
		"email":      "owner@example.com",
		"password":   "Sup3rS3cret!",
	}

	doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/register", secondPayload, nil, http.StatusCreated)

	loginPayload := map[string]interface{}{
		"email":    "owner@example.com",
		"password": "Sup3rS3cret!",
	}

	loginResp := doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/login", loginPayload, nil, http.StatusOK)

	if len(loginResp.Tenants) != 2 {
		t.Fatalf("expected 2 tenant memberships, got %d", len(loginResp.Tenants))
	}

	defaultCount := 0
	for _, membership := range loginResp.Tenants {
		if membership.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly one default membership, got %d", defaultCount)
	}
}

func TestAuthSwitchTenant(t *testing.T) {
	setupTestJWT(t)
	repo := newTestRepository()
	service := authsvc.NewWithRepository(repo, fixedTime)

	handler, cleanup := setupAuthHandler(t, service)
	defer cleanup()

	firstPayload := map[string]interface{}{
		"tenantName": "Acme Corp",
		"name":       "Jane Owner",
		"email":      "owner@example.com",
		"password":   "Sup3rS3cret!",
	}

	firstResp := doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/register", firstPayload, nil, http.StatusCreated)

	secondPayload := map[string]interface{}{
		"tenantName": "Beta Corp",
		"name":       "Jane Owner",
		"email":      "owner@example.com",
		"password":   "Sup3rS3cret!",
	}

	doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/register", secondPayload, nil, http.StatusCreated)

	loginResp := doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/login", map[string]interface{}{
		"email":    "owner@example.com",
		"password": "Sup3rS3cret!",
	}, nil, http.StatusOK)

	if len(loginResp.Tenants) != 2 {
		t.Fatalf("expected 2 memberships, got %d", len(loginResp.Tenants))
	}

	var target dto.TenantMembership
	for _, membership := range loginResp.Tenants {
		if membership.TenantID != firstResp.Tenant.TenantID {
			target = membership
			break
		}
	}
	if target.TenantID == "" {
		t.Fatal("expected to find target tenant in memberships")
	}

	headers := map[string]string{
		"Authorization": "Bearer " + loginResp.AccessToken,
	}

	switchResp := doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/switch", map[string]interface{}{
		"tenantId": target.TenantID,
	}, headers, http.StatusOK)

	if switchResp.Tenant.TenantID != target.TenantID {
		t.Fatalf("expected tenant %s after switch, got %s", target.TenantID, switchResp.Tenant.TenantID)
	}

	defaultCount := 0
	for _, membership := range switchResp.Tenants {
		if membership.IsDefault {
			defaultCount++
			if membership.TenantID != target.TenantID {
				t.Fatalf("expected default tenant %s, got %s", target.TenantID, membership.TenantID)
			}
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected single default membership after switch, got %d", defaultCount)
	}

	foundFirst := false
	for _, membership := range switchResp.Tenants {
		if membership.TenantID == firstResp.Tenant.TenantID {
			foundFirst = true
			break
		}
	}
	if !foundFirst {
		t.Fatal("expected original tenant to remain in memberships after switch")
	}
}

func TestAuthSwitchTenantRejectsUnknownMembership(t *testing.T) {
	setupTestJWT(t)
	repo := newTestRepository()
	service := authsvc.NewWithRepository(repo, fixedTime)

	handler, cleanup := setupAuthHandler(t, service)
	defer cleanup()

	payload := map[string]interface{}{
		"tenantName": "Acme Corp",
		"name":       "Jane Owner",
		"email":      "owner@example.com",
		"password":   "Sup3rS3cret!",
	}

	doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/register", payload, nil, http.StatusCreated)

	loginResp := doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/login", map[string]interface{}{
		"email":    "owner@example.com",
		"password": "Sup3rS3cret!",
	}, nil, http.StatusOK)

	headers := map[string]string{
		"Authorization": "Bearer " + loginResp.AccessToken,
	}

	doJSONRequest[api.ApiError](t, handler, http.MethodPost, "/api/auth/switch", map[string]interface{}{
		"tenantId": "non-existent",
	}, headers, http.StatusUnauthorized)
}

func TestAuthSwitchTenantValidatesRequestBody(t *testing.T) {
	setupTestJWT(t)
	repo := newTestRepository()
	service := authsvc.NewWithRepository(repo, fixedTime)

	handler, cleanup := setupAuthHandler(t, service)
	defer cleanup()

	payload := map[string]interface{}{
		"tenantName": "Acme Corp",
		"name":       "Jane Owner",
		"email":      "owner@example.com",
		"password":   "Sup3rS3cret!",
	}

	doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/register", payload, nil, http.StatusCreated)

	loginResp := doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/login", map[string]interface{}{
		"email":    "owner@example.com",
		"password": "Sup3rS3cret!",
	}, nil, http.StatusOK)

	headers := map[string]string{
		"Authorization": "Bearer " + loginResp.AccessToken,
	}

	sameResp := doJSONRequest[dto.AuthResponse](t, handler, http.MethodPost, "/api/auth/switch", map[string]interface{}{}, headers, http.StatusOK)

	if sameResp.Tenant.TenantID != loginResp.Tenant.TenantID {
		t.Fatalf("expected tenant to remain %s, got %s", loginResp.Tenant.TenantID, sameResp.Tenant.TenantID)
	}
	if len(sameResp.Tenants) == 0 || !sameResp.Tenants[0].IsDefault {
		t.Fatal("expected default membership to remain unchanged")
	}
}
