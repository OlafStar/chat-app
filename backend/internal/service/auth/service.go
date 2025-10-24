package auth

import (
	"chat-app-backend/internal/database"
	internaljwt "chat-app-backend/internal/jwt"
	"chat-app-backend/internal/model"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultPlan      = "starter"
	defaultPlanSeats = 1
)

type Service struct {
	repo Repository
	now  func() time.Time
}

var createTokenWithRefresh = internaljwt.CreateTokenWithRefresh

func SetTokenIssuer(issuer func(internaljwt.User, internaljwt.Role, int64) (internaljwt.TokenResponse, error)) {
	if issuer == nil {
		createTokenWithRefresh = internaljwt.CreateTokenWithRefresh
		return
	}
	createTokenWithRefresh = issuer
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

func (s *Service) Register(ctx context.Context, params RegisterParams) (AuthResult, error) {
	email := normalizeEmail(params.OwnerEmail)
	password := strings.TrimSpace(params.Password)
	name := strings.TrimSpace(params.OwnerName)
	tenantName := strings.TrimSpace(params.TenantName)

	if email == "" || password == "" || name == "" || tenantName == "" {
		return AuthResult{}, newError(ErrorCodeValidation, "missing required fields", nil)
	}

	plan := defaultPlan
	seats := defaultPlanSeats

	now := s.now().UTC().Format(time.RFC3339)
	tenantID := uuid.NewString()
	userID := uuid.NewString()

	tenant := model.TenantItem{
		TenantID: tenantID,
		Name:     tenantName,
		Plan:     plan,
		Seats:    seats,
		Settings: map[string]interface{}{},
		Created:  now,
	}

	if err := s.repo.CreateTenant(ctx, tenant); err != nil {
		return AuthResult{}, newError(ErrorCodeInternal, "failed to create tenant", err)
	}

	newUser, err := internaljwt.NewUser(internaljwt.RegisterUser{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return AuthResult{}, newError(ErrorCodeInternal, "failed to prepare user", err)
	}

	newUser.Id = userID
	newUser.TenantID = tenantID

	user := model.UserItem{
		PK:           model.TenantScopedPK(tenantID, userID),
		TenantID:     tenantID,
		UserID:       userID,
		Email:        email,
		Name:         name,
		Role:         "owner",
		Status:       "active",
		PasswordHash: newUser.PasswordHash,
		CreatedAt:    now,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return AuthResult{}, newError(ErrorCodeInternal, "failed to save user", err)
	}

	tokens, err := createTokenWithRefresh(newUser, internaljwt.RoleUser, 0)
	if err != nil {
		return AuthResult{}, newError(ErrorCodeInternal, "failed to issue tokens", err)
	}

	return AuthResult{
		User:   user,
		Tenant: tenant,
		Tokens: tokens,
		Memberships: []Membership{
			{
				User:      user,
				Tenant:    tenant,
				IsDefault: true,
			},
		},
	}, nil
}

func (s *Service) Login(ctx context.Context, params LoginParams) (AuthResult, error) {
	email := normalizeEmail(params.Email)
	password := strings.TrimSpace(params.Password)
	tenantID := strings.TrimSpace(params.TenantID)

	if email == "" || password == "" {
		return AuthResult{}, newError(ErrorCodeValidation, "missing required fields", nil)
	}

	matches, err := s.resolveUserTenants(ctx, email, tenantID, password)
	if err != nil {
		return AuthResult{}, err
	}

	defaultIdx := s.selectDefaultMembership(matches, tenantID)
	defaultMatch := matches[defaultIdx]

	jwtUser := internaljwt.User{
		Id:           defaultMatch.User.UserID,
		TenantID:     defaultMatch.User.TenantID,
		Email:        defaultMatch.User.Email,
		PasswordHash: defaultMatch.User.PasswordHash,
	}

	tokens, err := createTokenWithRefresh(jwtUser, internaljwt.RoleUser, 0)
	if err != nil {
		return AuthResult{}, newError(ErrorCodeInternal, "failed to issue tokens", err)
	}

	memberships := make([]Membership, len(matches))
	for i, match := range matches {
		memberships[i] = Membership{
			User:      match.User,
			Tenant:    match.Tenant,
			IsDefault: i == defaultIdx,
		}
	}

	return AuthResult{
		User:        defaultMatch.User,
		Tenant:      defaultMatch.Tenant,
		Tokens:      tokens,
		Memberships: memberships,
	}, nil
}

func (s *Service) SwitchTenant(ctx context.Context, identity Identity, tenantID string) (AuthResult, error) {
	email := normalizeEmail(identity.Email)
	tenantID = strings.TrimSpace(tenantID)

	if email == "" {
		return AuthResult{}, newError(ErrorCodeUnauthorized, "invalid user identity", nil)
	}
	if tenantID == "" {
		tenantID = strings.TrimSpace(identity.TenantID)
	}
	matches, err := s.fetchMemberships(ctx, email)
	if err != nil {
		return AuthResult{}, err
	}
	if len(matches) == 0 {
		return AuthResult{}, newError(ErrorCodeUnauthorized, "memberships not found", nil)
	}

	defaultIdx := -1
	for i, match := range matches {
		if match.Tenant.TenantID == tenantID {
			defaultIdx = i
			break
		}
	}
	if defaultIdx == -1 {
		return AuthResult{}, newError(ErrorCodeUnauthorized, "membership not found", nil)
	}

	defaultMatch := matches[defaultIdx]

	jwtUser := internaljwt.User{
		Id:           defaultMatch.User.UserID,
		TenantID:     defaultMatch.User.TenantID,
		Email:        defaultMatch.User.Email,
		PasswordHash: defaultMatch.User.PasswordHash,
	}

	tokens, err := createTokenWithRefresh(jwtUser, internaljwt.RoleUser, 0)
	if err != nil {
		return AuthResult{}, newError(ErrorCodeInternal, "failed to issue tokens", err)
	}

	memberships := make([]Membership, len(matches))
	for i, match := range matches {
		memberships[i] = Membership{
			User:      match.User,
			Tenant:    match.Tenant,
			IsDefault: i == defaultIdx,
		}
	}

	return AuthResult{
		User:        defaultMatch.User,
		Tenant:      defaultMatch.Tenant,
		Tokens:      tokens,
		Memberships: memberships,
	}, nil
}

func (s *Service) Me(ctx context.Context, identity Identity) (ProfileResult, error) {
	userID := strings.TrimSpace(identity.UserID)
	tenantID := strings.TrimSpace(identity.TenantID)

	if userID == "" || tenantID == "" {
		return ProfileResult{}, newError(ErrorCodeUnauthorized, "invalid user identity", nil)
	}

	user, err := s.repo.GetUser(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ProfileResult{}, newError(ErrorCodeNotFound, "user not found", err)
		}
		return ProfileResult{}, newError(ErrorCodeInternal, "failed to fetch user", err)
	}

	tenant, err := s.repo.GetTenant(ctx, tenantID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ProfileResult{}, newError(ErrorCodeNotFound, "tenant not found", err)
		}
		return ProfileResult{}, newError(ErrorCodeInternal, "failed to fetch tenant", err)
	}

	return ProfileResult{
		User:   user,
		Tenant: tenant,
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

type userTenantMatch struct {
	User   model.UserItem
	Tenant model.TenantItem
}

func (s *Service) resolveUserTenants(ctx context.Context, email, tenantID, password string) ([]userTenantMatch, error) {
	memberships, err := s.fetchMemberships(ctx, email)
	if err != nil {
		return nil, err
	}
	if len(memberships) == 0 {
		return nil, newError(ErrorCodeUnauthorized, "invalid credentials", nil)
	}

	if tenantID != "" {
		for _, match := range memberships {
			if match.Tenant.TenantID == tenantID {
				if !internaljwt.ValidatePassword(match.User.PasswordHash, password) {
					return nil, newError(ErrorCodeUnauthorized, "invalid credentials", nil)
				}
				return []userTenantMatch{match}, nil
			}
		}
		return nil, newError(ErrorCodeUnauthorized, "invalid credentials", nil)
	}

	filtered := make([]userTenantMatch, 0, len(memberships))
	for _, match := range memberships {
		if internaljwt.ValidatePassword(match.User.PasswordHash, password) {
			filtered = append(filtered, match)
		}
	}

	if len(filtered) == 0 {
		return nil, newError(ErrorCodeUnauthorized, "invalid credentials", nil)
	}

	return filtered, nil
}

func (s *Service) fetchMemberships(ctx context.Context, email string) ([]userTenantMatch, error) {
	users, err := s.repo.ListUsersByEmail(ctx, email)
	if err != nil {
		return nil, newError(ErrorCodeInternal, "failed to fetch user", err)
	}

	matches := make([]userTenantMatch, 0, len(users))
	for _, user := range users {
		if user.Status != "active" {
			continue
		}

		tenant, err := s.repo.GetTenant(ctx, user.TenantID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, newError(ErrorCodeInternal, "failed to fetch tenant", err)
		}

		matches = append(matches, userTenantMatch{User: user, Tenant: tenant})
	}

	return matches, nil
}

func (s *Service) selectDefaultMembership(matches []userTenantMatch, tenantID string) int {
	if tenantID != "" {
		return 0
	}

	for i, match := range matches {
		if match.User.Role == "owner" {
			return i
		}
	}

	return 0
}
