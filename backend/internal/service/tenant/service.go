package tenant

import (
	"chat-app-backend/internal/database"
	internaljwt "chat-app-backend/internal/jwt"
	"chat-app-backend/internal/model"
	"chat-app-backend/utils"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ErrorCode string

const (
	ErrorCodeValidation   ErrorCode = "validation_error"
	ErrorCodeUnauthorized ErrorCode = "unauthorized"
	ErrorCodeForbidden    ErrorCode = "forbidden"
	ErrorCodeNotFound     ErrorCode = "not_found"
	ErrorCodeConflict     ErrorCode = "conflict"
	ErrorCodeSeatLimit    ErrorCode = "seat_limit"
	ErrorCodeInternal     ErrorCode = "internal_error"
)

const (
	inviteTTL            = 48 * time.Hour
	inviteStatusPending  = "pending"
	inviteStatusAccepted = "accepted"
	inviteStatusExpired  = "expired"
)

var allowedTenantRoles = map[string]bool{
	"member": true,
	"admin":  true,
}

type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func newError(code ErrorCode, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

type Identity struct {
	UserID   string
	TenantID string
	Email    string
}

type AddUserParams struct {
	Name     string
	Email    string
	Password string
	Role     string
}

type TenantResult struct {
	Tenant         model.TenantItem
	RemainingSeats int
}

type TenantAPIKey struct {
	KeyID     string
	APIKey    string
	CreatedAt time.Time
}

type InviteResult struct {
	Token     string
	Email     string
	TenantID  string
	Role      string
	ExpiresAt time.Time
}

type AddUserResult struct {
	User           *model.UserItem
	Tenant         *model.TenantItem
	Invite         *InviteResult
	RemainingSeats int
}

type PendingInvite struct {
	Token     string
	Email     string
	Role      string
	InvitedBy string
	CreatedAt time.Time
	ExpiresAt time.Time
	Tenant    model.TenantItem
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func New(db *database.Database) *Service {
	return &Service{
		repo: NewDynamoRepository(db),
		now:  time.Now,
	}
}

func NewWithRepository(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		repo: repo,
		now:  now,
	}
}

func (s *Service) UpdateTenantName(ctx context.Context, identity Identity, tenantID, name string) (TenantResult, error) {
	name = strings.TrimSpace(name)
	tenantID = strings.TrimSpace(tenantID)

	if name == "" {
		return TenantResult{}, newError(ErrorCodeValidation, "tenant name is required", nil)
	}
	if identity.UserID == "" || identity.TenantID == "" {
		return TenantResult{}, newError(ErrorCodeUnauthorized, "invalid user identity", nil)
	}
	if tenantID == "" {
		tenantID = identity.TenantID
	}

	user, tenant, err := s.ensureOwnerAccess(ctx, identity, tenantID)
	if err != nil {
		return TenantResult{}, err
	}

	if tenant.Name == name {
		remaining, err := s.fetchRemainingSeats(ctx, tenant)
		if err != nil {
			return TenantResult{}, err
		}
		return TenantResult{
			Tenant:         tenant,
			RemainingSeats: remaining,
		}, nil
	}

	updated, err := s.repo.UpdateTenantName(ctx, tenant.TenantID, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return TenantResult{}, newError(ErrorCodeNotFound, "tenant not found", err)
		}
		return TenantResult{}, newError(ErrorCodeInternal, "failed to update tenant", err)
	}

	if updated.TenantID == "" {
		updated = tenant
		updated.Name = name
	}

	// ensure we retain up-to-date metadata in case repo returned partial data
	updated.TenantID = tenant.TenantID
	updated.Plan = tenant.Plan
	updated.Seats = tenant.Seats
	updated.Created = tenant.Created

	_ = user // owner access already validated

	remaining, err := s.fetchRemainingSeats(ctx, updated)
	if err != nil {
		return TenantResult{}, err
	}

	return TenantResult{
		Tenant:         updated,
		RemainingSeats: remaining,
	}, nil
}

func (s *Service) ListTenantAPIKeys(ctx context.Context, identity Identity, tenantID string) ([]TenantAPIKey, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(identity.TenantID)
	}

	_, tenant, err := s.ensureOwnerAccess(ctx, identity, tenantID)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.ListTenantAPIKeys(ctx, tenant.TenantID)
	if err != nil {
		return nil, newError(ErrorCodeInternal, "failed to list tenant api keys", err)
	}

	keys := make([]TenantAPIKey, 0, len(items))
	for _, item := range items {
		key, err := toTenantAPIKey(item)
		if err != nil {
			return nil, newError(ErrorCodeInternal, "invalid api key record", err)
		}
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].CreatedAt.After(keys[j].CreatedAt)
	})

	return keys, nil
}

func (s *Service) CreateTenantAPIKey(ctx context.Context, identity Identity, tenantID string) (TenantAPIKey, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(identity.TenantID)
	}

	_, tenant, err := s.ensureOwnerAccess(ctx, identity, tenantID)
	if err != nil {
		return TenantAPIKey{}, err
	}

	now := s.now().UTC()
	item := model.TenantAPIKeyItem{
		TenantID:  tenant.TenantID,
		KeyID:     uuid.NewString(),
		APIKey:    utils.GenerateAPIKey(),
		CreatedAt: now.Format(time.RFC3339),
	}

	if err := s.repo.CreateTenantAPIKey(ctx, item); err != nil {
		return TenantAPIKey{}, newError(ErrorCodeInternal, "failed to create tenant api key", err)
	}

	key, err := toTenantAPIKey(item)
	if err != nil {
		return TenantAPIKey{}, newError(ErrorCodeInternal, "failed to prepare api key response", err)
	}

	return key, nil
}

func (s *Service) DeleteTenantAPIKey(ctx context.Context, identity Identity, tenantID, keyID string) error {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(identity.TenantID)
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return newError(ErrorCodeValidation, "keyId is required", nil)
	}

	_, tenant, err := s.ensureOwnerAccess(ctx, identity, tenantID)
	if err != nil {
		return err
	}

	if _, err := s.repo.GetTenantAPIKey(ctx, tenant.TenantID, keyID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return newError(ErrorCodeNotFound, "api key not found", err)
		}
		return newError(ErrorCodeInternal, "failed to load api key", err)
	}

	if err := s.repo.DeleteTenantAPIKey(ctx, tenant.TenantID, keyID); err != nil {
		return newError(ErrorCodeInternal, "failed to delete api key", err)
	}
	return nil
}

func toTenantAPIKey(item model.TenantAPIKeyItem) (TenantAPIKey, error) {
	createdAt, err := time.Parse(time.RFC3339, item.CreatedAt)
	if err != nil {
		return TenantAPIKey{}, err
	}
	return TenantAPIKey{
		KeyID:     item.KeyID,
		APIKey:    item.APIKey,
		CreatedAt: createdAt,
	}, nil
}

func (s *Service) AddUser(ctx context.Context, identity Identity, tenantID string, params AddUserParams) (AddUserResult, error) {
	name := strings.TrimSpace(params.Name)
	email := normalizeEmail(params.Email)
	password := strings.TrimSpace(params.Password)
	tenantID = strings.TrimSpace(tenantID)
	role := strings.TrimSpace(params.Role)
	if role == "" {
		role = "member"
	}
	if !allowedTenantRoles[role] {
		return AddUserResult{}, newError(ErrorCodeValidation, "invalid role", nil)
	}

	if name == "" || email == "" {
		return AddUserResult{}, newError(ErrorCodeValidation, "name and email are required", nil)
	}
	if identity.UserID == "" || identity.TenantID == "" {
		return AddUserResult{}, newError(ErrorCodeUnauthorized, "invalid user identity", nil)
	}
	if tenantID == "" {
		tenantID = identity.TenantID
	}

	_, tenant, err := s.ensureOwnerAccess(ctx, identity, tenantID)
	if err != nil {
		return AddUserResult{}, err
	}

	if _, err := s.repo.FindUserByEmail(ctx, tenantID, email); err == nil {
		return AddUserResult{}, newError(ErrorCodeConflict, "user with this email already exists", nil)
	} else if !errors.Is(err, ErrNotFound) {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to check existing user", err)
	}

	users, err := s.repo.ListUsersByTenant(ctx, tenantID)
	if err != nil {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to list users", err)
	}

	activeCount := countActiveUsers(users)
	capacity := tenant.Seats + 1
	if activeCount >= capacity {
		return AddUserResult{}, newError(ErrorCodeSeatLimit, fmt.Sprintf("no seats available on plan %s", tenant.Plan), nil)
	}

	existingUsers, err := s.repo.ListUsersByEmail(ctx, email)
	if err != nil {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to check existing users", err)
	}
	if len(existingUsers) > 0 {
		return s.createInvite(ctx, identity, tenant, email, role, activeCount, capacity)
	}

	if password == "" {
		return AddUserResult{}, newError(ErrorCodeValidation, "password is required for new users", nil)
	}

	newUser, err := internaljwt.NewUser(internaljwt.RegisterUser{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to prepare user", err)
	}

	userID := uuid.NewString()
	now := s.now().UTC().Format(time.RFC3339)

	item := model.UserItem{
		PK:           model.TenantScopedPK(tenantID, userID),
		TenantID:     tenantID,
		UserID:       userID,
		Email:        email,
		Name:         name,
		Role:         role,
		Status:       "active",
		PasswordHash: newUser.PasswordHash,
		CreatedAt:    now,
	}

	if err := s.repo.CreateUser(ctx, item); err != nil {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to create user", err)
	}

	remaining := capacity - (activeCount + 1)
	if remaining < 0 {
		remaining = 0
	}

	tenantCopy := tenant
	return AddUserResult{
		User:           &item,
		Tenant:         &tenantCopy,
		RemainingSeats: remaining,
	}, nil
}

func (s *Service) IdentityFromAuthorizationHeader(header string) (Identity, error) {
	authHeader := strings.TrimSpace(header)
	if authHeader == "" {
		return Identity{}, newError(ErrorCodeUnauthorized, "missing authorization header", nil)
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return Identity{}, newError(ErrorCodeUnauthorized, "invalid authorization header format", nil)
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	return s.identityFromToken(token)
}

func (s *Service) identityFromToken(token string) (Identity, error) {
	if token == "" {
		return Identity{}, newError(ErrorCodeUnauthorized, "empty token", nil)
	}

	claims, err := internaljwt.ParseToken(token, internaljwt.RoleUser)
	if err != nil {
		return Identity{}, newError(ErrorCodeUnauthorized, "invalid token", err)
	}

	userID, _ := claims["id"].(string)
	email, _ := claims["email"].(string)
	tenantID, _ := claims["tenantId"].(string)

	if userID == "" || tenantID == "" {
		return Identity{}, newError(ErrorCodeUnauthorized, "token missing identifiers", nil)
	}

	return Identity{
		UserID:   userID,
		TenantID: tenantID,
		Email:    email,
	}, nil
}

func (s *Service) ensureOwnerAccess(ctx context.Context, identity Identity, tenantID string) (model.UserItem, model.TenantItem, error) {
	if tenantID != identity.TenantID {
		return model.UserItem{}, model.TenantItem{}, newError(ErrorCodeForbidden, "action restricted to tenant owners", nil)
	}

	user, err := s.repo.GetUser(ctx, tenantID, identity.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return model.UserItem{}, model.TenantItem{}, newError(ErrorCodeUnauthorized, "user not found for tenant", err)
		}
		return model.UserItem{}, model.TenantItem{}, newError(ErrorCodeInternal, "failed to fetch user", err)
	}

	if user.Status != "active" {
		return model.UserItem{}, model.TenantItem{}, newError(ErrorCodeForbidden, "user is not active", nil)
	}

	if user.Role != "owner" {
		return model.UserItem{}, model.TenantItem{}, newError(ErrorCodeForbidden, "only tenant owners can perform this action", nil)
	}

	tenant, err := s.repo.GetTenant(ctx, tenantID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return model.UserItem{}, model.TenantItem{}, newError(ErrorCodeNotFound, "tenant not found", err)
		}
		return model.UserItem{}, model.TenantItem{}, newError(ErrorCodeInternal, "failed to fetch tenant", err)
	}

	return user, tenant, nil
}

func countActiveUsers(users []model.UserItem) int {
	count := 0
	for _, user := range users {
		if strings.EqualFold(user.Status, "disabled") {
			continue
		}
		count++
	}
	return count
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *Service) fetchRemainingSeats(ctx context.Context, tenant model.TenantItem) (int, error) {
	users, err := s.repo.ListUsersByTenant(ctx, tenant.TenantID)
	if err != nil {
		return 0, newError(ErrorCodeInternal, "failed to list users", err)
	}
	capacity := tenant.Seats + 1
	remaining := capacity - countActiveUsers(users)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

func (s *Service) createInvite(
	ctx context.Context,
	identity Identity,
	tenant model.TenantItem,
	email string,
	role string,
	activeCount, capacity int,
) (AddUserResult, error) {
	if invite, err := s.repo.FindActiveInvite(ctx, tenant.TenantID, email); err == nil {
		expiresAt, parseErr := time.Parse(time.RFC3339, invite.ExpiresAt)
		if parseErr != nil {
			expiresAt = s.now().Add(inviteTTL)
		}
		tenantCopy := tenant
		return AddUserResult{
			Invite: &InviteResult{
				Token:     invite.Token,
				Email:     invite.Email,
				TenantID:  invite.TenantID,
				Role:      invite.Role,
				ExpiresAt: expiresAt,
			},
			Tenant:         &tenantCopy,
			RemainingSeats: capacity - activeCount,
		}, nil
	} else if !errors.Is(err, ErrNotFound) {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to lookup invite", err)
	}

	now := s.now().UTC()
	expiresAt := now.Add(inviteTTL)

	invite := model.TenantInviteItem{
		Token:       uuid.NewString(),
		TenantID:    tenant.TenantID,
		Email:       email,
		TenantEmail: model.TenantScopedPK(tenant.TenantID, email),
		InvitedBy:   identity.UserID,
		Role:        role,
		Status:      inviteStatusPending,
		CreatedAt:   now.Format(time.RFC3339),
		ExpiresAt:   expiresAt.Format(time.RFC3339),
	}

	if err := s.repo.CreateInvite(ctx, invite); err != nil {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to create invite", err)
	}

	tenantCopy := tenant
	return AddUserResult{
		Invite: &InviteResult{
			Token:     invite.Token,
			Email:     invite.Email,
			TenantID:  invite.TenantID,
			Role:      invite.Role,
			ExpiresAt: expiresAt,
		},
		Tenant:         &tenantCopy,
		RemainingSeats: capacity - activeCount,
	}, nil
}

func (s *Service) ListPendingInvites(ctx context.Context, identity Identity) ([]PendingInvite, error) {
	if identity.UserID == "" {
		return nil, newError(ErrorCodeUnauthorized, "invalid user identity", nil)
	}

	email := normalizeEmail(identity.Email)
	if email == "" {
		return nil, newError(ErrorCodeUnauthorized, "missing email on identity", nil)
	}

	invites, err := s.repo.ListInvitesByEmail(ctx, email)
	if err != nil {
		return nil, newError(ErrorCodeInternal, "failed to list invites", err)
	}

	now := s.now().UTC()
	results := make([]PendingInvite, 0, len(invites))
	tenantCache := make(map[string]model.TenantItem)

	for _, invite := range invites {
		if !strings.EqualFold(invite.Status, inviteStatusPending) {
			continue
		}

		expiresAt, parseErr := time.Parse(time.RFC3339, invite.ExpiresAt)
		if parseErr != nil || now.After(expiresAt) {
			_ = s.repo.UpdateInviteStatus(ctx, invite.Token, inviteStatusExpired)
			continue
		}

		tenant, ok := tenantCache[invite.TenantID]
		if !ok {
			var getErr error
			tenant, getErr = s.repo.GetTenant(ctx, invite.TenantID)
			if getErr != nil {
				if errors.Is(getErr, ErrNotFound) {
					_ = s.repo.UpdateInviteStatus(ctx, invite.Token, inviteStatusExpired)
					continue
				}
				return nil, newError(ErrorCodeInternal, "failed to fetch tenant for invite", getErr)
			}
			tenantCache[invite.TenantID] = tenant
		}

		createdAt, parseCreatedErr := time.Parse(time.RFC3339, invite.CreatedAt)
		if parseCreatedErr != nil {
			createdAt = time.Time{}
		}

		role := invite.Role
		if !allowedTenantRoles[role] {
			role = "member"
		}

		tenantCopy := tenant
		results = append(results, PendingInvite{
			Token:     invite.Token,
			Email:     invite.Email,
			Role:      role,
			InvitedBy: invite.InvitedBy,
			CreatedAt: createdAt.UTC(),
			ExpiresAt: expiresAt.UTC(),
			Tenant:    tenantCopy,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].ExpiresAt.Equal(results[j].ExpiresAt) {
			return results[i].Token < results[j].Token
		}
		return results[i].ExpiresAt.Before(results[j].ExpiresAt)
	})

	return results, nil
}

func (s *Service) AcceptInvite(ctx context.Context, token, name string) (AddUserResult, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return AddUserResult{}, newError(ErrorCodeValidation, "invite token is required", nil)
	}

	invite, err := s.repo.GetInvite(ctx, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AddUserResult{}, newError(ErrorCodeNotFound, "invite not found", err)
		}
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to fetch invite", err)
	}

	if invite.Status != inviteStatusPending {
		return AddUserResult{}, newError(ErrorCodeConflict, "invite already used", nil)
	}

	expiresAt, err := time.Parse(time.RFC3339, invite.ExpiresAt)
	if err != nil {
		expiresAt = s.now()
	}
	if s.now().After(expiresAt) {
		_ = s.repo.UpdateInviteStatus(ctx, token, inviteStatusExpired)
		return AddUserResult{}, newError(ErrorCodeValidation, "invite expired", nil)
	}

	tenant, err := s.repo.GetTenant(ctx, invite.TenantID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AddUserResult{}, newError(ErrorCodeNotFound, "tenant not found", err)
		}
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to fetch tenant", err)
	}

	users, err := s.repo.ListUsersByTenant(ctx, tenant.TenantID)
	if err != nil {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to list users", err)
	}

	activeCount := countActiveUsers(users)
	capacity := tenant.Seats + 1
	if activeCount >= capacity {
		return AddUserResult{}, newError(ErrorCodeSeatLimit, fmt.Sprintf("no seats available on plan %s", tenant.Plan), nil)
	}

	if _, err := s.repo.FindUserByEmail(ctx, tenant.TenantID, invite.Email); err == nil {
		return AddUserResult{}, newError(ErrorCodeConflict, "user already belongs to tenant", nil)
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to verify existing membership", err)
	}

	existingUsers, err := s.repo.ListUsersByEmail(ctx, invite.Email)
	if err != nil {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to fetch existing accounts", err)
	}
	if len(existingUsers) == 0 {
		return AddUserResult{}, newError(ErrorCodeValidation, "no account registered with this email", nil)
	}

	source := existingUsers[0]
	for _, candidate := range existingUsers {
		if strings.EqualFold(candidate.Status, "active") {
			source = candidate
			break
		}
	}
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = source.Name
	}
	if displayName == "" {
		displayName = invite.Email
	}
	if source.PasswordHash == "" {
		return AddUserResult{}, newError(ErrorCodeInternal, "existing account missing credentials", nil)
	}

	userID := uuid.NewString()
	now := s.now().UTC().Format(time.RFC3339)
	userRole := invite.Role
	if !allowedTenantRoles[userRole] {
		userRole = "member"
	}

	item := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, userID),
		TenantID:     tenant.TenantID,
		UserID:       userID,
		Email:        invite.Email,
		Name:         displayName,
		Role:         userRole,
		Status:       "active",
		PasswordHash: source.PasswordHash,
		CreatedAt:    now,
	}

	if err := s.repo.CreateUser(ctx, item); err != nil {
		return AddUserResult{}, newError(ErrorCodeInternal, "failed to create user", err)
	}

	_ = s.repo.UpdateInviteStatus(ctx, token, inviteStatusAccepted)

	remaining := capacity - (activeCount + 1)
	if remaining < 0 {
		remaining = 0
	}

	tenantCopy := tenant
	return AddUserResult{
		User:           &item,
		Tenant:         &tenantCopy,
		RemainingSeats: remaining,
	}, nil
}
