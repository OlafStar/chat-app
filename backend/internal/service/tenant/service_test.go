package tenant

import (
	internaljwt "chat-app-backend/internal/jwt"
	"chat-app-backend/internal/model"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryRepository struct {
	mu           sync.Mutex
	tenants      map[string]model.TenantItem
	users        map[string]model.UserItem
	usersByEmail map[string]map[string]string
	invites      map[string]model.TenantInviteItem
	keys         map[string]map[string]model.TenantAPIKeyItem
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		tenants:      make(map[string]model.TenantItem),
		users:        make(map[string]model.UserItem),
		usersByEmail: make(map[string]map[string]string),
		invites:      make(map[string]model.TenantInviteItem),
		keys:         make(map[string]map[string]model.TenantAPIKeyItem),
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

func (m *memoryRepository) UpdateTenantName(ctx context.Context, tenantID, name string) (model.TenantItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenant, ok := m.tenants[tenantID]
	if !ok {
		return model.TenantItem{}, ErrNotFound
	}

	tenant.Name = name
	m.tenants[tenantID] = tenant
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

func (m *memoryRepository) ListUsersByTenant(ctx context.Context, tenantID string) ([]model.UserItem, error) {
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

func (m *memoryRepository) CreateInvite(ctx context.Context, invite model.TenantInviteItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invites[invite.Token] = invite
	return nil
}

func (m *memoryRepository) GetInvite(ctx context.Context, token string) (model.TenantInviteItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	invite, ok := m.invites[token]
	if !ok {
		return model.TenantInviteItem{}, ErrNotFound
	}
	return invite, nil
}

func (m *memoryRepository) ListInvitesByEmail(ctx context.Context, email string) ([]model.TenantInviteItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.TenantInviteItem, 0)
	for _, invite := range m.invites {
		if invite.Email == email {
			out = append(out, invite)
		}
	}
	return out, nil
}

func (m *memoryRepository) FindActiveInvite(ctx context.Context, tenantID, email string) (model.TenantInviteItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	target := model.TenantScopedPK(tenantID, email)
	for _, invite := range m.invites {
		if invite.TenantEmail == target && invite.Status == inviteStatusPending {
			return invite, nil
		}
	}
	return model.TenantInviteItem{}, ErrNotFound
}

func (m *memoryRepository) UpdateInviteStatus(ctx context.Context, token, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	invite, ok := m.invites[token]
	if !ok {
		return ErrNotFound
	}
	invite.Status = status
	m.invites[token] = invite
	return nil
}

func (m *memoryRepository) DeleteInvite(ctx context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.invites, token)
	return nil
}

func (m *memoryRepository) ListTenantAPIKeys(ctx context.Context, tenantID string) ([]model.TenantAPIKeyItem, error) {
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

func (m *memoryRepository) CreateTenantAPIKey(ctx context.Context, item model.TenantAPIKeyItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.keys[item.TenantID]; !ok {
		m.keys[item.TenantID] = make(map[string]model.TenantAPIKeyItem)
	}
	m.keys[item.TenantID][item.KeyID] = item
	return nil
}

func (m *memoryRepository) DeleteTenantAPIKey(ctx context.Context, tenantID, keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tenantKeys, ok := m.keys[tenantID]; ok {
		delete(tenantKeys, keyID)
	}
	return nil
}

func (m *memoryRepository) GetTenantAPIKey(ctx context.Context, tenantID, keyID string) (model.TenantAPIKeyItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tenantKeys, ok := m.keys[tenantID]; ok {
		if key, ok := tenantKeys[keyID]; ok {
			return key, nil
		}
	}
	return model.TenantAPIKeyItem{}, ErrNotFound
}

func fixedNow() time.Time {
	return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
}

func newService(repo Repository) *Service {
	return NewWithRepository(repo, fixedNow)
}

func TestUpdateTenantName(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Old Name",
		Plan:     "starter",
		Seats:    1,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:        model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:  tenant.TenantID,
		UserID:    "owner-1",
		Email:     "owner@example.com",
		Name:      "Owner",
		Role:      "owner",
		Status:    "active",
		CreatedAt: fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: tenant.TenantID,
		Email:    owner.Email,
	}

	result, err := service.UpdateTenantName(context.Background(), identity, tenant.TenantID, "New Name")
	if err != nil {
		t.Fatalf("UpdateTenantName error: %v", err)
	}
	if result.Tenant.Name != "New Name" {
		t.Fatalf("expected name to be updated, got %s", result.Tenant.Name)
	}
	if repo.tenants[tenant.TenantID].Name != "New Name" {
		t.Fatalf("repository not updated, got %s", repo.tenants[tenant.TenantID].Name)
	}
	if result.RemainingSeats != 1 {
		t.Fatalf("expected remaining seats 1, got %d", result.RemainingSeats)
	}
}

func TestUpdateTenantNameRequiresOwner(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Old Name",
		Plan:     "starter",
		Seats:    1,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	member := model.UserItem{
		PK:        model.TenantScopedPK(tenant.TenantID, "member-1"),
		TenantID:  tenant.TenantID,
		UserID:    "member-1",
		Email:     "user@example.com",
		Name:      "User",
		Role:      "member",
		Status:    "active",
		CreatedAt: fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), member)

	identity := Identity{
		UserID:   member.UserID,
		TenantID: tenant.TenantID,
		Email:    member.Email,
	}

	_, err := service.UpdateTenantName(context.Background(), identity, tenant.TenantID, "New Name")
	if err == nil {
		t.Fatal("expected error for non-owner")
	}
	svcErr, ok := err.(*Error)
	if !ok || svcErr.Code != ErrorCodeForbidden {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestListTenantAPIKeys(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  fixedNow().Format(time.RFC3339),
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
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	repo.keys[tenant.TenantID] = map[string]model.TenantAPIKeyItem{
		"key-1": {
			TenantID:  tenant.TenantID,
			KeyID:     "key-1",
			APIKey:    "pingy_OLD",
			CreatedAt: fixedNow().Add(-time.Hour).Format(time.RFC3339),
		},
		"key-2": {
			TenantID:  tenant.TenantID,
			KeyID:     "key-2",
			APIKey:    "pingy_NEW",
			CreatedAt: fixedNow().Format(time.RFC3339),
		},
	}

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: tenant.TenantID,
		Email:    owner.Email,
	}

	keys, err := service.ListTenantAPIKeys(context.Background(), identity, tenant.TenantID)
	if err != nil {
		t.Fatalf("ListTenantAPIKeys error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0].APIKey != "pingy_NEW" {
		t.Fatalf("expected newest key first, got %s", keys[0].APIKey)
	}
	if keys[1].APIKey != "pingy_OLD" {
		t.Fatalf("expected oldest key second, got %s", keys[1].APIKey)
	}
}

func TestCreateTenantAPIKey(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  fixedNow().Format(time.RFC3339),
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
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: tenant.TenantID,
		Email:    owner.Email,
	}

	key, err := service.CreateTenantAPIKey(context.Background(), identity, tenant.TenantID)
	if err != nil {
		t.Fatalf("CreateTenantAPIKey error: %v", err)
	}
	if key.APIKey == "" {
		t.Fatalf("expected api key value")
	}
	if !strings.HasPrefix(key.APIKey, "pingy_") {
		t.Fatalf("expected pingy_ prefix, got %s", key.APIKey)
	}
	if key.KeyID == "" {
		t.Fatalf("expected key id")
	}

	stored, err := repo.GetTenantAPIKey(context.Background(), tenant.TenantID, key.KeyID)
	if err != nil {
		t.Fatalf("expected key stored: %v", err)
	}
	if stored.APIKey != key.APIKey {
		t.Fatalf("expected stored api key to match, got %s", stored.APIKey)
	}
}

func TestDeleteTenantAPIKey(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  fixedNow().Format(time.RFC3339),
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
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	key := model.TenantAPIKeyItem{
		TenantID:  tenant.TenantID,
		KeyID:     "key-1",
		APIKey:    "pingy_KEY",
		CreatedAt: fixedNow().Format(time.RFC3339),
	}
	repo.CreateTenantAPIKey(context.Background(), key)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: tenant.TenantID,
		Email:    owner.Email,
	}

	if err := service.DeleteTenantAPIKey(context.Background(), identity, tenant.TenantID, key.KeyID); err != nil {
		t.Fatalf("DeleteTenantAPIKey error: %v", err)
	}

	if _, err := repo.GetTenantAPIKey(context.Background(), tenant.TenantID, key.KeyID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected key to be removed, got %v", err)
	}
}

func TestAddUserHonoursSeatLimit(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  fixedNow().Format(time.RFC3339),
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
		CreatedAt:    fixedNow().Format(time.RFC3339),
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
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), member)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: tenant.TenantID,
		Email:    owner.Email,
	}

	_, err := service.AddUser(context.Background(), identity, tenant.TenantID, AddUserParams{
		Name:     "Another User",
		Email:    "new@example.com",
		Password: "password",
	})
	if err == nil {
		t.Fatal("expected seat limit error")
	}
	svcErr, ok := err.(*Error)
	if !ok || svcErr.Code != ErrorCodeSeatLimit {
		t.Fatalf("expected seat limit error, got %v", err)
	}
}

func TestAddUserSuccess(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	passwordUser, err := internaljwt.NewUser(internaljwt.RegisterUser{
		Email:    "owner@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("prepare owner: %v", err)
	}

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: passwordUser.PasswordHash,
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: owner.TenantID,
		Email:    owner.Email,
	}

	result, err := service.AddUser(context.Background(), identity, tenant.TenantID, AddUserParams{
		Name:     "Agent Smith",
		Email:    "agent@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("AddUser error: %v", err)
	}

	if result.User == nil {
		t.Fatal("expected user to be created")
	}
	if result.User.UserID == "" {
		t.Fatal("expected user id to be set")
	}
	if result.User.Role != "member" {
		t.Fatalf("expected role member, got %s", result.User.Role)
	}
	if result.User.Status != "active" {
		t.Fatalf("expected active status, got %s", result.User.Status)
	}
	if result.RemainingSeats != 1 {
		t.Fatalf("expected remaining seats 1, got %d", result.RemainingSeats)
	}

	stored, err := repo.FindUserByEmail(context.Background(), tenant.TenantID, "agent@example.com")
	if err != nil {
		t.Fatalf("failed to lookup stored user: %v", err)
	}
	if stored.UserID != result.User.UserID {
		t.Fatalf("expected stored user to match, got %s", stored.UserID)
	}
}

func TestAddUserRejectsInvalidRole(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:        model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:  tenant.TenantID,
		UserID:    "owner-1",
		Email:     "owner@example.com",
		Name:      "Owner",
		Role:      "owner",
		Status:    "active",
		CreatedAt: fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: owner.TenantID,
		Email:    owner.Email,
	}

	_, err := service.AddUser(context.Background(), identity, tenant.TenantID, AddUserParams{
		Name:     "Agent",
		Email:    "agent@example.com",
		Password: "password",
		Role:     "owner",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid role")
	}
	svcErr, ok := err.(*Error)
	if !ok || svcErr.Code != ErrorCodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestAddUserReturnsZeroRemainingWhenTenantFull(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	passwordUser, err := internaljwt.NewUser(internaljwt.RegisterUser{
		Email:    "owner@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("prepare owner: %v", err)
	}

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: passwordUser.PasswordHash,
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: owner.TenantID,
		Email:    owner.Email,
	}

	result, err := service.AddUser(context.Background(), identity, tenant.TenantID, AddUserParams{
		Name:     "First Agent",
		Email:    "agent@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("AddUser error: %v", err)
	}

	if result.RemainingSeats != 0 {
		t.Fatalf("expected remaining seats 0, got %d", result.RemainingSeats)
	}
}

func TestAddUserCreatesInviteForExistingAccount(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	ownerUser, err := internaljwt.NewUser(internaljwt.RegisterUser{
		Email:    "owner@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("prepare owner: %v", err)
	}

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: ownerUser.PasswordHash,
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	other := model.UserItem{
		PK:           model.TenantScopedPK("tenant-2", "existing"),
		TenantID:     "tenant-2",
		UserID:       "existing",
		Email:        "agent@example.com",
		Name:         "Agent",
		Role:         "member",
		Status:       "active",
		PasswordHash: ownerUser.PasswordHash,
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), other)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: owner.TenantID,
		Email:    owner.Email,
	}

	result, err := service.AddUser(context.Background(), identity, tenant.TenantID, AddUserParams{
		Name:  "Agent",
		Email: "agent@example.com",
	})
	if err != nil {
		t.Fatalf("AddUser invite error: %v", err)
	}

	if result.User != nil {
		t.Fatal("expected invite instead of user creation")
	}
	if result.Invite == nil {
		t.Fatal("expected invite to be created")
	}
	if result.Invite.Role != "member" {
		t.Fatalf("expected invite role member, got %s", result.Invite.Role)
	}

	if _, err := repo.FindActiveInvite(context.Background(), tenant.TenantID, "agent@example.com"); err != nil {
		t.Fatalf("expected active invite in repository: %v", err)
	}
}

func TestAddUserReturnsExistingInviteToken(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	ownerUser, err := internaljwt.NewUser(internaljwt.RegisterUser{
		Email:    "owner@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("prepare owner: %v", err)
	}

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: ownerUser.PasswordHash,
		CreatedAt:    fixedNow().Format(time.RFC3339),
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
		PasswordHash: ownerUser.PasswordHash,
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), existing)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: owner.TenantID,
		Email:    owner.Email,
	}

	first, err := service.AddUser(context.Background(), identity, tenant.TenantID, AddUserParams{
		Name:  "Agent",
		Email: "agent@example.com",
	})
	if err != nil {
		t.Fatalf("first invite error: %v", err)
	}
	second, err := service.AddUser(context.Background(), identity, tenant.TenantID, AddUserParams{
		Name:  "Agent",
		Email: "agent@example.com",
	})
	if err != nil {
		t.Fatalf("second invite error: %v", err)
	}
	if first.Invite == nil || second.Invite == nil {
		t.Fatalf("expected invites in both responses: %+v %+v", first, second)
	}
	if first.Invite.Token != second.Invite.Token {
		t.Fatalf("expected same invite token, got %s and %s", first.Invite.Token, second.Invite.Token)
	}
}

func TestAcceptInviteCreatesMembership(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	ownerUser, err := internaljwt.NewUser(internaljwt.RegisterUser{
		Email:    "owner@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("prepare owner: %v", err)
	}

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: ownerUser.PasswordHash,
		CreatedAt:    fixedNow().Format(time.RFC3339),
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
		PasswordHash: ownerUser.PasswordHash,
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), existing)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: owner.TenantID,
		Email:    owner.Email,
	}

	inviteResult, err := service.AddUser(context.Background(), identity, tenant.TenantID, AddUserParams{
		Name:  "Agent",
		Email: "agent@example.com",
	})
	if err != nil {
		t.Fatalf("AddUser invite error: %v", err)
	}
	if inviteResult.Invite == nil {
		t.Fatal("expected invite to be generated")
	}

	acceptResult, err := service.AcceptInvite(context.Background(), inviteResult.Invite.Token, "")
	if err != nil {
		t.Fatalf("AcceptInvite error: %v", err)
	}
	if acceptResult.User == nil {
		t.Fatal("expected user to be created on accept")
	}
	if acceptResult.User.Email != "agent@example.com" {
		t.Fatalf("unexpected user email %s", acceptResult.User.Email)
	}
	if acceptResult.User.Role != "member" {
		t.Fatalf("expected role member, got %s", acceptResult.User.Role)
	}

	stored, err := repo.FindUserByEmail(context.Background(), tenant.TenantID, "agent@example.com")
	if err != nil {
		t.Fatalf("expected user persisted: %v", err)
	}
	if stored.UserID != acceptResult.User.UserID {
		t.Fatalf("expected stored user to match, got %s", stored.UserID)
	}

	invite, err := repo.GetInvite(context.Background(), inviteResult.Invite.Token)
	if err != nil {
		t.Fatalf("expected invite to remain accessible: %v", err)
	}
	if invite.Status != inviteStatusAccepted {
		t.Fatalf("expected invite status accepted, got %s", invite.Status)
	}
}

func TestAcceptInviteRequiresPendingStatus(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	invite := model.TenantInviteItem{
		Token:       "token",
		TenantID:    tenant.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenant.TenantID, "agent@example.com"),
		InvitedBy:   "owner",
		Role:        "member",
		Status:      inviteStatusAccepted,
		CreatedAt:   fixedNow().Format(time.RFC3339),
		ExpiresAt:   fixedNow().Add(time.Hour).Format(time.RFC3339),
	}
	repo.CreateInvite(context.Background(), invite)

	_, err := service.AcceptInvite(context.Background(), invite.Token, "")
	if err == nil {
		t.Fatal("expected conflict for non-pending invite")
	}
	svcErr, ok := err.(*Error)
	if !ok || svcErr.Code != ErrorCodeConflict {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestAcceptInviteFailsWhenExpired(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	existingUser := model.UserItem{
		PK:           model.TenantScopedPK("tenant-2", "existing"),
		TenantID:     "tenant-2",
		UserID:       "existing",
		Email:        "agent@example.com",
		Name:         "Agent",
		Role:         "member",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), existingUser)

	expiredInvite := model.TenantInviteItem{
		Token:       "invite-token",
		TenantID:    tenant.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenant.TenantID, "agent@example.com"),
		InvitedBy:   "owner",
		Role:        "member",
		Status:      inviteStatusPending,
		CreatedAt:   fixedNow().Add(-time.Hour).Format(time.RFC3339),
		ExpiresAt:   fixedNow().Add(-time.Minute).Format(time.RFC3339),
	}
	repo.CreateInvite(context.Background(), expiredInvite)

	_, err := service.AcceptInvite(context.Background(), expiredInvite.Token, "")
	if err == nil {
		t.Fatal("expected error for expired invite")
	}
	svcErr, ok := err.(*Error)
	if !ok || svcErr.Code != ErrorCodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestAcceptInviteSecondUseFails(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	existing := model.UserItem{
		PK:           model.TenantScopedPK("tenant-2", "existing"),
		TenantID:     "tenant-2",
		UserID:       "existing",
		Email:        "agent@example.com",
		Name:         "Agent",
		Role:         "member",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), existing)

	invite := model.TenantInviteItem{
		Token:       "token",
		TenantID:    tenant.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenant.TenantID, "agent@example.com"),
		InvitedBy:   "owner",
		Role:        "member",
		Status:      inviteStatusPending,
		CreatedAt:   fixedNow().Format(time.RFC3339),
		ExpiresAt:   fixedNow().Add(time.Hour).Format(time.RFC3339),
	}
	repo.CreateInvite(context.Background(), invite)

	if _, err := service.AcceptInvite(context.Background(), invite.Token, ""); err != nil {
		t.Fatalf("first accept error: %v", err)
	}
	_, err := service.AcceptInvite(context.Background(), invite.Token, "")
	if err == nil {
		t.Fatal("expected conflict on second acceptance")
	}
	svcErr, ok := err.(*Error)
	if !ok || svcErr.Code != ErrorCodeConflict {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestAcceptInviteHonoursSeatLimit(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  fixedNow().Format(time.RFC3339),
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
		CreatedAt:    fixedNow().Format(time.RFC3339),
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
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), existing)

	member := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "member-1"),
		TenantID:     tenant.TenantID,
		UserID:       "member-1",
		Email:        "member@example.com",
		Name:         "Member",
		Role:         "member",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), member)

	invite := model.TenantInviteItem{
		Token:       "invite-token",
		TenantID:    tenant.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenant.TenantID, "agent@example.com"),
		InvitedBy:   owner.UserID,
		Role:        "member",
		Status:      inviteStatusPending,
		CreatedAt:   fixedNow().Format(time.RFC3339),
		ExpiresAt:   fixedNow().Add(time.Hour).Format(time.RFC3339),
	}
	repo.CreateInvite(context.Background(), invite)

	_, err := service.AcceptInvite(context.Background(), invite.Token, "")
	if err == nil {
		t.Fatal("expected seat limit error on acceptance")
	}
	svcErr, ok := err.(*Error)
	if !ok || svcErr.Code != ErrorCodeSeatLimit {
		t.Fatalf("expected seat limit error, got %v", err)
	}
}

func TestListPendingInvitesFiltersAndSorts(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenantA := model.TenantItem{
		TenantID: "tenant-a",
		Name:     "Tenant A",
		Plan:     "starter",
		Seats:    3,
		Created:  fixedNow().Format(time.RFC3339),
	}
	tenantB := model.TenantItem{
		TenantID: "tenant-b",
		Name:     "Tenant B",
		Plan:     "starter",
		Seats:    5,
		Created:  fixedNow().Add(-time.Hour).Format(time.RFC3339),
	}
	repo.tenants[tenantA.TenantID] = tenantA
	repo.tenants[tenantB.TenantID] = tenantB

	validSoon := model.TenantInviteItem{
		Token:       "valid-soon",
		TenantID:    tenantA.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenantA.TenantID, "agent@example.com"),
		InvitedBy:   "owner-a",
		Role:        "admin",
		Status:      inviteStatusPending,
		CreatedAt:   fixedNow().Add(-time.Minute).Format(time.RFC3339),
		ExpiresAt:   fixedNow().Add(2 * time.Hour).Format(time.RFC3339),
	}
	validLater := model.TenantInviteItem{
		Token:       "valid-later",
		TenantID:    tenantB.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenantB.TenantID, "agent@example.com"),
		InvitedBy:   "owner-b",
		Role:        "owner",
		Status:      inviteStatusPending,
		CreatedAt:   fixedNow().Add(-2 * time.Minute).Format(time.RFC3339),
		ExpiresAt:   fixedNow().Add(3 * time.Hour).Format(time.RFC3339),
	}
	expired := model.TenantInviteItem{
		Token:       "expired",
		TenantID:    tenantA.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenantA.TenantID, "agent@example.com"),
		InvitedBy:   "owner-a",
		Role:        "member",
		Status:      inviteStatusPending,
		CreatedAt:   fixedNow().Add(-4 * time.Hour).Format(time.RFC3339),
		ExpiresAt:   fixedNow().Add(-time.Minute).Format(time.RFC3339),
	}
	otherEmail := model.TenantInviteItem{
		Token:       "other-email",
		TenantID:    tenantA.TenantID,
		Email:       "other@example.com",
		TenantEmail: model.TenantScopedPK(tenantA.TenantID, "other@example.com"),
		InvitedBy:   "owner-a",
		Role:        "admin",
		Status:      inviteStatusPending,
		CreatedAt:   fixedNow().Format(time.RFC3339),
		ExpiresAt:   fixedNow().Add(2 * time.Hour).Format(time.RFC3339),
	}
	alreadyUsed := model.TenantInviteItem{
		Token:       "already-used",
		TenantID:    tenantB.TenantID,
		Email:       "agent@example.com",
		TenantEmail: model.TenantScopedPK(tenantB.TenantID, "agent@example.com"),
		InvitedBy:   "owner-b",
		Role:        "admin",
		Status:      inviteStatusAccepted,
		CreatedAt:   fixedNow().Add(-30 * time.Minute).Format(time.RFC3339),
		ExpiresAt:   fixedNow().Add(time.Hour).Format(time.RFC3339),
	}

	repo.invites[validSoon.Token] = validSoon
	repo.invites[validLater.Token] = validLater
	repo.invites[expired.Token] = expired
	repo.invites[otherEmail.Token] = otherEmail
	repo.invites[alreadyUsed.Token] = alreadyUsed

	identity := Identity{
		UserID:   "user-1",
		TenantID: "tenant-a",
		Email:    "Agent@example.com",
	}

	invites, err := service.ListPendingInvites(context.Background(), identity)
	if err != nil {
		t.Fatalf("list pending invites: %v", err)
	}

	if len(invites) != 2 {
		t.Fatalf("expected 2 invites, got %d", len(invites))
	}

	if invites[0].Token != validSoon.Token {
		t.Fatalf("expected first invite %s, got %s", validSoon.Token, invites[0].Token)
	}
	if invites[1].Token != validLater.Token {
		t.Fatalf("expected second invite %s, got %s", validLater.Token, invites[1].Token)
	}

	if invites[1].Role != "member" {
		t.Fatalf("expected sanitized role member, got %s", invites[1].Role)
	}

	if invites[0].Tenant.TenantID != tenantA.TenantID {
		t.Fatalf("expected tenant A, got %s", invites[0].Tenant.TenantID)
	}
	if invites[1].Tenant.TenantID != tenantB.TenantID {
		t.Fatalf("expected tenant B, got %s", invites[1].Tenant.TenantID)
	}

	if status := repo.invites[expired.Token].Status; status != inviteStatusExpired {
		t.Fatalf("expected expired invite to be marked expired, got %s", status)
	}
}

func TestListPendingInvitesRequiresIdentityEmail(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	identity := Identity{
		UserID:   "user-1",
		TenantID: "tenant-a",
		Email:    "",
	}

	_, err := service.ListPendingInvites(context.Background(), identity)
	if err == nil {
		t.Fatal("expected error for missing email")
	}

	svcErr, ok := err.(*Error)
	if !ok || svcErr.Code != ErrorCodeUnauthorized {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestAddUserIgnoresDisabledMembers(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    1,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	passwordUser, err := internaljwt.NewUser(internaljwt.RegisterUser{
		Email:    "owner@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("prepare owner: %v", err)
	}

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: passwordUser.PasswordHash,
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), owner)

	disabled := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "disabled-1"),
		TenantID:     tenant.TenantID,
		UserID:       "disabled-1",
		Email:        "disabled@example.com",
		Name:         "Disabled",
		Role:         "member",
		Status:       "DISABLED",
		PasswordHash: "hash",
		CreatedAt:    fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), disabled)

	identity := Identity{
		UserID:   owner.UserID,
		TenantID: owner.TenantID,
		Email:    owner.Email,
	}

	result, err := service.AddUser(context.Background(), identity, tenant.TenantID, AddUserParams{
		Name:     "Replacement",
		Email:    "replacement@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("AddUser error: %v", err)
	}

	if result.User == nil {
		t.Fatal("expected user to be created")
	}
	if result.RemainingSeats != 0 {
		t.Fatalf("expected remaining seats 0, got %d", result.RemainingSeats)
	}
}

func TestAddUserRequiresOwner(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Tenant",
		Plan:     "starter",
		Seats:    2,
		Created:  fixedNow().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	member := model.UserItem{
		PK:        model.TenantScopedPK(tenant.TenantID, "member-1"),
		TenantID:  tenant.TenantID,
		UserID:    "member-1",
		Email:     "member@example.com",
		Name:      "Member",
		Role:      "member",
		Status:    "active",
		CreatedAt: fixedNow().Format(time.RFC3339),
	}
	repo.CreateUser(context.Background(), member)

	identity := Identity{
		UserID:   member.UserID,
		TenantID: member.TenantID,
		Email:    member.Email,
	}

	_, err := service.AddUser(context.Background(), identity, tenant.TenantID, AddUserParams{
		Name:     "Agent Smith",
		Email:    "agent@example.com",
		Password: "password",
	})
	if err == nil {
		t.Fatal("expected forbidden error")
	}
	svcErr, ok := err.(*Error)
	if !ok || svcErr.Code != ErrorCodeForbidden {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}
