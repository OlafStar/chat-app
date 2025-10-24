package auth

import (
	internaljwt "chat-app-backend/internal/jwt"
	"chat-app-backend/internal/model"
	"context"
	"sync"
	"testing"
	"time"
)

type memoryRepository struct {
	mu           sync.Mutex
	tenants      map[string]model.TenantItem
	users        map[string]model.UserItem
	usersByEmail map[string]map[string]string
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		tenants:      make(map[string]model.TenantItem),
		users:        make(map[string]model.UserItem),
		usersByEmail: make(map[string]map[string]string),
	}
}

func (m *memoryRepository) CreateTenant(ctx context.Context, tenant model.TenantItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tenants[tenant.TenantID] = tenant
	return nil
}

func (m *memoryRepository) CreateUser(ctx context.Context, user model.UserItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.PK] = user
	if _, ok := m.usersByEmail[user.TenantID]; !ok {
		m.usersByEmail[user.TenantID] = make(map[string]string)
	}
	m.usersByEmail[user.TenantID][user.Email] = user.PK
	return nil
}

func (m *memoryRepository) FindUserByEmail(ctx context.Context, tenantID, email string) (model.UserItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantUsers, ok := m.usersByEmail[tenantID]
	if !ok {
		return model.UserItem{}, ErrNotFound
	}

	pk, ok := tenantUsers[email]
	if !ok {
		return model.UserItem{}, ErrNotFound
	}

	user, ok := m.users[pk]
	if !ok {
		return model.UserItem{}, ErrNotFound
	}

	return user, nil
}

func (m *memoryRepository) ListUsersByEmail(ctx context.Context, email string) ([]model.UserItem, error) {
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

func (m *memoryRepository) GetTenant(ctx context.Context, tenantID string) (model.TenantItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenant, ok := m.tenants[tenantID]
	if !ok {
		return model.TenantItem{}, ErrNotFound
	}
	return tenant, nil
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

func setupJWT(t *testing.T) {
	t.Helper()

	original := createTokenWithRefresh
	internaljwt.RoleSecrets[internaljwt.RoleUser] = "test-secret"
	SetTokenIssuer(func(user internaljwt.User, role internaljwt.Role, validUntil int64) (internaljwt.TokenResponse, error) {
		token, err := internaljwt.CreateToken(user, role, validUntil)
		if err != nil {
			return internaljwt.TokenResponse{}, err
		}
		return internaljwt.TokenResponse{
			AccessToken: token,
		}, nil
	})

	t.Cleanup(func() {
		SetTokenIssuer(original)
	})
}

func fixedNow() time.Time {
	return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
}

func TestRegisterUsesDefaultPlanAndSeats(t *testing.T) {
	setupJWT(t)
	repo := newMemoryRepository()
	svc := NewWithRepository(repo, fixedNow)

	result, err := svc.Register(context.Background(), RegisterParams{
		TenantName: "Acme",
		OwnerName:  "Owner",
		OwnerEmail: "owner@example.com",
		Password:   "secret",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Tenant.Plan != defaultPlan {
		t.Fatalf("expected plan %s, got %s", defaultPlan, result.Tenant.Plan)
	}

	if result.Tenant.Seats != defaultPlanSeats {
		t.Fatalf("expected seats %d, got %d", defaultPlanSeats, result.Tenant.Seats)
	}

	if len(result.Memberships) != 1 || !result.Memberships[0].IsDefault {
		t.Fatalf("expected single default membership, got %#v", result.Memberships)
	}
}

func TestRegisterValidatesRequiredFields(t *testing.T) {
	setupJWT(t)
	repo := newMemoryRepository()
	svc := NewWithRepository(repo, fixedNow)

	_, err := svc.Register(context.Background(), RegisterParams{
		TenantName: "Acme",
		OwnerName:  "Owner",
		OwnerEmail: "owner@example.com",
		Password:   "",
	})

	if err == nil {
		t.Fatal("expected validation error for missing password")
	}

	svcErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected service error, got %T", err)
	}

	if svcErr.Code != ErrorCodeValidation {
		t.Fatalf("expected validation error, got %s", svcErr.Code)
	}
}

func TestLoginWithoutTenantSucceedsForSingleAssociation(t *testing.T) {
	setupJWT(t)
	repo := newMemoryRepository()
	svc := NewWithRepository(repo, fixedNow)

	_, err := svc.Register(context.Background(), RegisterParams{
		TenantName: "Acme",
		OwnerName:  "Owner",
		OwnerEmail: "owner@example.com",
		Password:   "secret",
	})
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	result, err := svc.Login(context.Background(), LoginParams{
		Email:    "owner@example.com",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("login error: %v", err)
	}

	if result.User.Email != "owner@example.com" {
		t.Fatalf("expected email owner@example.com, got %s", result.User.Email)
	}

	if len(result.Memberships) != 1 || !result.Memberships[0].IsDefault {
		t.Fatalf("expected single default membership, got %#v", result.Memberships)
	}
}

func TestLoginWithoutTenantSelectsDefaultWhenMultiple(t *testing.T) {
	setupJWT(t)
	repo := newMemoryRepository()
	svc := NewWithRepository(repo, fixedNow)

	_, err := svc.Register(context.Background(), RegisterParams{
		TenantName: "Acme",
		OwnerName:  "Owner",
		OwnerEmail: "owner@example.com",
		Password:   "secret",
	})
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	_, err = svc.Register(context.Background(), RegisterParams{
		TenantName: "Beta",
		OwnerName:  "Owner Two",
		OwnerEmail: "owner@example.com",
		Password:   "secret",
	})
	if err != nil {
		t.Fatalf("second register error: %v", err)
	}

	loginResult, err := svc.Login(context.Background(), LoginParams{
		Email:    "owner@example.com",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	if len(loginResult.Memberships) != 2 {
		t.Fatalf("expected 2 memberships, got %d", len(loginResult.Memberships))
	}

	defaultCount := 0
	for _, membership := range loginResult.Memberships {
		if membership.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly one default membership, got %d", defaultCount)
	}
}

func TestSwitchTenantChangesDefault(t *testing.T) {
	setupJWT(t)
	repo := newMemoryRepository()
	svc := NewWithRepository(repo, fixedNow)

	first, err := svc.Register(context.Background(), RegisterParams{
		TenantName: "Acme",
		OwnerName:  "Owner",
		OwnerEmail: "owner@example.com",
		Password:   "secret",
	})
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	second, err := svc.Register(context.Background(), RegisterParams{
		TenantName: "Beta",
		OwnerName:  "Owner",
		OwnerEmail: "owner@example.com",
		Password:   "secret",
	})
	if err != nil {
		t.Fatalf("second register error: %v", err)
	}

	loginResult, err := svc.Login(context.Background(), LoginParams{
		Email:    "owner@example.com",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("login error: %v", err)
	}
	if len(loginResult.Memberships) != 2 {
		t.Fatalf("expected 2 memberships, got %d", len(loginResult.Memberships))
	}

	identity := Identity{
		UserID:   loginResult.User.UserID,
		TenantID: loginResult.Tenant.TenantID,
		Email:    loginResult.User.Email,
	}

	var target Membership
	for _, member := range loginResult.Memberships {
		if member.Tenant.TenantID == second.Tenant.TenantID {
			target = member
			break
		}
	}
	if target.Tenant.TenantID == "" {
		t.Fatal("expected to find membership for target tenant")
	}

	switchResult, err := svc.SwitchTenant(context.Background(), identity, target.Tenant.TenantID)
	if err != nil {
		t.Fatalf("switch error: %v", err)
	}

	if switchResult.Tenant.TenantID != target.Tenant.TenantID {
		t.Fatalf("expected tenant %s, got %s", target.Tenant.TenantID, switchResult.Tenant.TenantID)
	}

	defaultCount := 0
	for _, member := range switchResult.Memberships {
		if member.IsDefault {
			defaultCount++
			if member.Tenant.TenantID != target.Tenant.TenantID {
				t.Fatalf("expected default tenant %s, got %s", target.Tenant.TenantID, member.Tenant.TenantID)
			}
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly one default membership, got %d", defaultCount)
	}

	// ensure original tenant still present
	foundOriginal := false
	for _, member := range switchResult.Memberships {
		if member.Tenant.TenantID == first.Tenant.TenantID {
			foundOriginal = true
			break
		}
	}
	if !foundOriginal {
		t.Fatal("expected original tenant to remain in membership list")
	}
}

func TestSwitchTenantValidatesMembership(t *testing.T) {
	setupJWT(t)
	repo := newMemoryRepository()
	svc := NewWithRepository(repo, fixedNow)

	res, err := svc.Register(context.Background(), RegisterParams{
		TenantName: "Acme",
		OwnerName:  "Owner",
		OwnerEmail: "owner@example.com",
		Password:   "secret",
	})
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	identity := Identity{
		UserID:   res.User.UserID,
		TenantID: res.Tenant.TenantID,
		Email:    res.User.Email,
	}

	_, err = svc.SwitchTenant(context.Background(), identity, "non-existent")
	if err == nil {
		t.Fatal("expected error for missing membership")
	}

	if svcErr, ok := err.(*Error); !ok || svcErr.Code != ErrorCodeUnauthorized {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestLoginRejectsInvalidPassword(t *testing.T) {
	setupJWT(t)
	repo := newMemoryRepository()
	svc := NewWithRepository(repo, fixedNow)

	_, err := svc.Register(context.Background(), RegisterParams{
		TenantName: "Acme",
		OwnerName:  "Owner",
		OwnerEmail: "owner@example.com",
		Password:   "secret",
	})
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	_, err = svc.Login(context.Background(), LoginParams{
		Email:    "owner@example.com",
		Password: "wrong",
	})
	if err == nil {
		t.Fatal("expected login to fail with invalid password")
	}
	if svcErr, ok := err.(*Error); !ok || svcErr.Code != ErrorCodeUnauthorized {
		t.Fatalf("expected unauthorized error for wrong password, got %v", err)
	}
}

func TestLoginSkipsInactiveMemberships(t *testing.T) {
	setupJWT(t)
	repo := newMemoryRepository()
	svc := NewWithRepository(repo, fixedNow)

	_, err := svc.Register(context.Background(), RegisterParams{
		TenantName: "Acme",
		OwnerName:  "Owner",
		OwnerEmail: "owner@example.com",
		Password:   "secret",
	})
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	secondTenant := model.TenantItem{
		TenantID: "tenant-two",
		Name:     "Beta",
		Plan:     "starter",
		Seats:    1,
		Created:  fixedNow().Format(time.RFC3339),
	}
	if err := repo.CreateTenant(context.Background(), secondTenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	hashed := ""
	for _, existing := range repo.users {
		hashed = existing.PasswordHash
		break
	}

	inactive := model.UserItem{
		PK:           model.TenantScopedPK(secondTenant.TenantID, "user-two"),
		TenantID:     secondTenant.TenantID,
		UserID:       "user-two",
		Email:        "owner@example.com",
		Name:         "Inactive",
		Role:         "member",
		Status:       "disabled",
		PasswordHash: hashed,
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	if err := repo.CreateUser(context.Background(), inactive); err != nil {
		t.Fatalf("create inactive user: %v", err)
	}

	result, err := svc.Login(context.Background(), LoginParams{
		Email:    "owner@example.com",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("login error: %v", err)
	}
	if len(result.Memberships) == 0 {
		t.Fatal("expected at least one membership from login")
	}
	for _, membership := range result.Memberships {
		if membership.User.Status != "active" {
			t.Fatalf("expected inactive memberships to be excluded, got %+v", membership)
		}
	}
}
