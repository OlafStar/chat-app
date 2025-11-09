package conversation

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"chat-app-backend/internal/database"
	"chat-app-backend/internal/env"
	internaljwt "chat-app-backend/internal/jwt"
	"chat-app-backend/internal/model"

	"github.com/google/uuid"
)

type ErrorCode string

const (
	ErrorCodeValidation   ErrorCode = "validation_error"
	ErrorCodeUnauthorized ErrorCode = "unauthorized"
	ErrorCodeForbidden    ErrorCode = "forbidden"
	ErrorCodeNotFound     ErrorCode = "not_found"
	ErrorCodeConflict     ErrorCode = "conflict"
	ErrorCodeInternal     ErrorCode = "internal_error"
)

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

type CreateConversationParams struct {
	TenantID     string
	TenantAPIKey string
	Visitor      VisitorParams
	Message      string
	Metadata     map[string]string
	Origin       string
}

type VisitorParams struct {
	VisitorID string
	Name      string
	Email     string
	Metadata  map[string]string
}

type ConversationResult struct {
	Conversation model.ConversationItem
	VisitorToken string
	Message      model.MessageItem
}

type MessageResult struct {
	Conversation model.ConversationItem
	Message      model.MessageItem
}

type ListConversationsResult struct {
	Conversations []model.ConversationItem
}

type ListMessagesResult struct {
	Conversation model.ConversationItem
	Messages     []model.MessageItem
}

type ConversationUsageResult struct {
	TenantID     string
	PeriodStart  time.Time
	PeriodEnd    time.Time
	StartedCount int
}

type VisitorAccess struct {
	TenantID       string
	ConversationID string
	VisitorID      string
}

type Service struct {
	repo Repository
	now  func() time.Time
}

var (
	visitorTokenSecret = []byte(env.MustGet(env.UserSecretKey))
	visitorTokenTTL    = 7 * 24 * time.Hour
)

type visitorTokenClaims struct {
	TenantID       string `json:"tenantId"`
	ConversationID string `json:"conversationId"`
	VisitorID      string `json:"visitorId"`
	IssuedAt       int64  `json:"iat"`
	ExpiresAt      int64  `json:"exp"`
}

func SetVisitorTokenSecret(secret []byte) {
	if len(secret) == 0 {
		return
	}
	visitorTokenSecret = make([]byte, len(secret))
	copy(visitorTokenSecret, secret)
}

func SetVisitorTokenTTL(ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	visitorTokenTTL = ttl
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

func (s *Service) CreateConversation(ctx context.Context, params CreateConversationParams) (ConversationResult, error) {
	tenantID := strings.TrimSpace(params.TenantID)
	tenantKey := strings.TrimSpace(params.TenantAPIKey)
	messageBody := strings.TrimSpace(params.Message)

	if messageBody == "" {
		return ConversationResult{}, newError(ErrorCodeValidation, "message body is required", nil)
	}

	var tenant model.TenantItem
	var err error

	if tenantKey != "" {
		tenant, err = s.repo.GetTenantByAPIKey(ctx, tenantKey)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ConversationResult{}, newError(ErrorCodeNotFound, "tenant not found", err)
			}
			return ConversationResult{}, newError(ErrorCodeInternal, "failed to load tenant", err)
		}
		tenantID = tenant.TenantID
	} else {
		if tenantID == "" {
			return ConversationResult{}, newError(ErrorCodeValidation, "tenantId or api key is required", nil)
		}
		tenant, err = s.repo.GetTenant(ctx, tenantID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ConversationResult{}, newError(ErrorCodeNotFound, "tenant not found", err)
			}
			return ConversationResult{}, newError(ErrorCodeInternal, "failed to load tenant", err)
		}
	}

	visitorID := strings.TrimSpace(params.Visitor.VisitorID)
	if visitorID == "" {
		visitorID = uuid.NewString()
	}

	now := s.now().UTC()
	nowStr := now.Format(time.RFC3339)
	conversationID := uuid.NewString()

	visitor := model.VisitorItem{
		PK:         model.VisitorPK(tenantID, visitorID),
		TenantID:   tenantID,
		VisitorID:  visitorID,
		Name:       strings.TrimSpace(params.Visitor.Name),
		Email:      normalizeEmail(params.Visitor.Email),
		Metadata:   cloneStringMap(params.Visitor.Metadata),
		CreatedAt:  nowStr,
		LastSeenAt: nowStr,
	}

	if existing, err := s.repo.GetVisitor(ctx, tenantID, visitorID); err == nil {
		if visitor.Name == "" {
			visitor.Name = existing.Name
		}
		if visitor.Email == "" {
			visitor.Email = existing.Email
		}
		if len(visitor.Metadata) == 0 && len(existing.Metadata) > 0 {
			visitor.Metadata = cloneStringMap(existing.Metadata)
		}
		visitor.CreatedAt = existing.CreatedAt
	} else if !errors.Is(err, ErrNotFound) {
		return ConversationResult{}, newError(ErrorCodeInternal, "failed to lookup visitor", err)
	}

	if err := s.repo.PutVisitor(ctx, visitor); err != nil {
		return ConversationResult{}, newError(ErrorCodeInternal, "failed to persist visitor", err)
	}

	conversation := model.ConversationItem{
		PK:             model.ConversationPK(tenantID, conversationID),
		ConversationID: conversationID,
		TenantID:       tenantID,
		VisitorID:      visitorID,
		VisitorName:    visitor.Name,
		VisitorEmail:   visitor.Email,
		Status:         model.ConversationStatusOpen,
		Metadata:       cloneStringMap(params.Metadata),
		OriginURL:      strings.TrimSpace(params.Origin),
		CreatedAt:      nowStr,
		UpdatedAt:      nowStr,
		LastMessageAt:  nowStr,
	}

	if err := s.repo.CreateConversation(ctx, conversation); err != nil {
		return ConversationResult{}, newError(ErrorCodeInternal, "failed to create conversation", err)
	}

	messageID := uuid.NewString()
	message := model.MessageItem{
		PK:             model.MessagePK(conversationID, messageID),
		TenantID:       tenantID,
		ConversationID: conversationID,
		MessageID:      messageID,
		SenderType:     "visitor",
		SenderID:       visitorID,
		Body:           messageBody,
		CreatedAt:      nowStr,
	}
	if err := s.repo.CreateMessage(ctx, message); err != nil {
		return ConversationResult{}, newError(ErrorCodeInternal, "failed to store message", err)
	}

	token, err := signVisitorToken(visitorTokenClaims{
		TenantID:       tenantID,
		ConversationID: conversationID,
		VisitorID:      visitorID,
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(visitorTokenTTL).Unix(),
	})
	if err != nil {
		return ConversationResult{}, newError(ErrorCodeInternal, "failed to issue visitor token", err)
	}

	return ConversationResult{
		Conversation: conversation,
		VisitorToken: token,
		Message:      message,
	}, nil
}

func (s *Service) PostVisitorMessage(ctx context.Context, token, body string) (MessageResult, error) {
	body = strings.TrimSpace(body)

	if body == "" {
		return MessageResult{}, newError(ErrorCodeValidation, "message body is required", nil)
	}

	access, err := s.ValidateVisitorAccess(token)
	if err != nil {
		return MessageResult{}, err
	}

	conversation, err := s.repo.GetConversation(ctx, access.TenantID, access.ConversationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return MessageResult{}, newError(ErrorCodeNotFound, "conversation not found", err)
		}
		return MessageResult{}, newError(ErrorCodeInternal, "failed to fetch conversation", err)
	}

	if conversation.Status == model.ConversationStatusClosed {
		return MessageResult{}, newError(ErrorCodeConflict, "conversation is closed", nil)
	}

	if conversation.VisitorID != access.VisitorID {
		return MessageResult{}, newError(ErrorCodeForbidden, "token does not match conversation", nil)
	}

	now := s.now().UTC()
	nowStr := now.Format(time.RFC3339)

	messageID := uuid.NewString()
	message := model.MessageItem{
		PK:             model.MessagePK(conversation.ConversationID, messageID),
		TenantID:       conversation.TenantID,
		ConversationID: conversation.ConversationID,
		MessageID:      messageID,
		SenderType:     "visitor",
		SenderID:       access.VisitorID,
		Body:           body,
		CreatedAt:      nowStr,
	}

	if err := s.repo.CreateMessage(ctx, message); err != nil {
		return MessageResult{}, newError(ErrorCodeInternal, "failed to store message", err)
	}

	if err := s.repo.UpdateConversationActivity(ctx, conversation.TenantID, conversation.ConversationID, nowStr, nowStr, nil); err != nil {
		return MessageResult{}, newError(ErrorCodeInternal, "failed to update conversation", err)
	}

	if existingVisitor, err := s.repo.GetVisitor(ctx, conversation.TenantID, access.VisitorID); err == nil {
		existingVisitor.LastSeenAt = nowStr
		if err := s.repo.PutVisitor(ctx, existingVisitor); err != nil {
			return MessageResult{}, newError(ErrorCodeInternal, "failed to update visitor", err)
		}
	} else if errors.Is(err, ErrNotFound) {
		visitor := model.VisitorItem{
			PK:         model.VisitorPK(conversation.TenantID, access.VisitorID),
			TenantID:   conversation.TenantID,
			VisitorID:  access.VisitorID,
			CreatedAt:  nowStr,
			LastSeenAt: nowStr,
		}
		if err := s.repo.PutVisitor(ctx, visitor); err != nil {
			return MessageResult{}, newError(ErrorCodeInternal, "failed to store visitor", err)
		}
	} else {
		return MessageResult{}, newError(ErrorCodeInternal, "failed to update visitor", err)
	}

	conversation.LastMessageAt = nowStr
	conversation.UpdatedAt = nowStr

	return MessageResult{
		Conversation: conversation,
		Message:      message,
	}, nil
}

func (s *Service) AssignVisitorEmail(ctx context.Context, token, email string) (model.ConversationItem, error) {
	email = normalizeEmail(email)
	if !isValidEmail(email) {
		return model.ConversationItem{}, newError(ErrorCodeValidation, "a valid email is required", nil)
	}

	access, err := s.ValidateVisitorAccess(token)
	if err != nil {
		return model.ConversationItem{}, err
	}

	conversation, err := s.repo.GetConversation(ctx, access.TenantID, access.ConversationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return model.ConversationItem{}, newError(ErrorCodeNotFound, "conversation not found", err)
		}
		return model.ConversationItem{}, newError(ErrorCodeInternal, "failed to fetch conversation", err)
	}

	if conversation.VisitorID != access.VisitorID {
		return model.ConversationItem{}, newError(ErrorCodeForbidden, "token does not match conversation", nil)
	}

	now := s.now().UTC()
	nowStr := now.Format(time.RFC3339)

	if err := s.repo.UpdateConversationVisitorEmail(ctx, conversation.TenantID, conversation.ConversationID, email, nowStr); err != nil {
		if errors.Is(err, ErrNotFound) {
			return model.ConversationItem{}, newError(ErrorCodeNotFound, "conversation not found", err)
		}
		return model.ConversationItem{}, newError(ErrorCodeInternal, "failed to update conversation", err)
	}

	visitor, err := s.repo.GetVisitor(ctx, conversation.TenantID, conversation.VisitorID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			visitor = model.VisitorItem{
				PK:         model.VisitorPK(conversation.TenantID, conversation.VisitorID),
				TenantID:   conversation.TenantID,
				VisitorID:  conversation.VisitorID,
				CreatedAt:  nowStr,
				LastSeenAt: nowStr,
			}
		} else {
			return model.ConversationItem{}, newError(ErrorCodeInternal, "failed to load visitor", err)
		}
	} else {
		visitor.Email = email
		visitor.LastSeenAt = nowStr
	}

	if visitor.PK == "" {
		visitor.PK = model.VisitorPK(conversation.TenantID, conversation.VisitorID)
	}
	if visitor.TenantID == "" {
		visitor.TenantID = conversation.TenantID
	}
	if visitor.VisitorID == "" {
		visitor.VisitorID = conversation.VisitorID
	}
	if visitor.CreatedAt == "" {
		visitor.CreatedAt = nowStr
	}
	visitor.Email = email

	if err := s.repo.PutVisitor(ctx, visitor); err != nil {
		return model.ConversationItem{}, newError(ErrorCodeInternal, "failed to persist visitor", err)
	}

	conversation.VisitorEmail = email
	conversation.UpdatedAt = nowStr

	return conversation, nil
}

func (s *Service) PostAgentMessage(ctx context.Context, identity Identity, conversationID, body string) (MessageResult, error) {
	conversationID = strings.TrimSpace(conversationID)
	body = strings.TrimSpace(body)

	if identity.UserID == "" || identity.TenantID == "" {
		return MessageResult{}, newError(ErrorCodeUnauthorized, "invalid user identity", nil)
	}
	if conversationID == "" {
		return MessageResult{}, newError(ErrorCodeValidation, "conversationId is required", nil)
	}
	if body == "" {
		return MessageResult{}, newError(ErrorCodeValidation, "message body is required", nil)
	}

	if _, err := s.repo.GetUser(ctx, identity.TenantID, identity.UserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return MessageResult{}, newError(ErrorCodeUnauthorized, "user not found", err)
		}
		return MessageResult{}, newError(ErrorCodeInternal, "failed to verify user", err)
	}

	conversation, err := s.repo.GetConversation(ctx, identity.TenantID, conversationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return MessageResult{}, newError(ErrorCodeNotFound, "conversation not found", err)
		}
		return MessageResult{}, newError(ErrorCodeInternal, "failed to fetch conversation", err)
	}

	now := s.now().UTC()
	nowStr := now.Format(time.RFC3339)

	messageID := uuid.NewString()
	message := model.MessageItem{
		PK:             model.MessagePK(conversation.ConversationID, messageID),
		TenantID:       identity.TenantID,
		ConversationID: conversation.ConversationID,
		MessageID:      messageID,
		SenderType:     "agent",
		SenderID:       identity.UserID,
		Body:           body,
		CreatedAt:      nowStr,
	}

	if err := s.repo.CreateMessage(ctx, message); err != nil {
		return MessageResult{}, newError(ErrorCodeInternal, "failed to store message", err)
	}

	if conversation.TenantStartedAt == "" {
		if err := s.repo.MarkConversationTenantStart(ctx, identity.TenantID, conversation.ConversationID, nowStr, identity.UserID); err != nil {
			return MessageResult{}, newError(ErrorCodeInternal, "failed to mark conversation start", err)
		}
		conversation.TenantStartedAt = nowStr
		conversation.TenantStartedBy = identity.UserID
	}

	var assigned *string
	if conversation.AssignedUserID == "" {
		assigned = &identity.UserID
	}

	if err := s.repo.UpdateConversationActivity(ctx, identity.TenantID, conversation.ConversationID, nowStr, nowStr, assigned); err != nil {
		return MessageResult{}, newError(ErrorCodeInternal, "failed to update conversation", err)
	}

	if assigned != nil {
		conversation.AssignedUserID = *assigned
	}
	conversation.LastMessageAt = nowStr
	conversation.UpdatedAt = nowStr

	return MessageResult{
		Conversation: conversation,
		Message:      message,
	}, nil
}

func (s *Service) ListConversations(ctx context.Context, identity Identity, limit int) (ListConversationsResult, error) {
	if identity.UserID == "" || identity.TenantID == "" {
		return ListConversationsResult{}, newError(ErrorCodeUnauthorized, "invalid user identity", nil)
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	if _, err := s.repo.GetUser(ctx, identity.TenantID, identity.UserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ListConversationsResult{}, newError(ErrorCodeUnauthorized, "user not found", err)
		}
		return ListConversationsResult{}, newError(ErrorCodeInternal, "failed to verify user", err)
	}

	conversations, err := s.repo.ListConversations(ctx, identity.TenantID, limit)
	if err != nil {
		return ListConversationsResult{}, newError(ErrorCodeInternal, "failed to list conversations", err)
	}

	return ListConversationsResult{Conversations: conversations}, nil
}

func (s *Service) GetConversationUsage(ctx context.Context, identity Identity, start, end time.Time) (ConversationUsageResult, error) {
	if identity.UserID == "" || identity.TenantID == "" {
		return ConversationUsageResult{}, newError(ErrorCodeUnauthorized, "invalid user identity", nil)
	}
	if start.IsZero() || end.IsZero() {
		return ConversationUsageResult{}, newError(ErrorCodeValidation, "period start and end are required", nil)
	}
	start = start.UTC()
	end = end.UTC()
	if !start.Before(end) {
		return ConversationUsageResult{}, newError(ErrorCodeValidation, "period start must be before period end", nil)
	}

	if _, err := s.repo.GetUser(ctx, identity.TenantID, identity.UserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ConversationUsageResult{}, newError(ErrorCodeUnauthorized, "user not found", err)
		}
		return ConversationUsageResult{}, newError(ErrorCodeInternal, "failed to verify user", err)
	}

	count, err := s.repo.CountConversationsStartedBetween(ctx, identity.TenantID, start, end)
	if err != nil {
		return ConversationUsageResult{}, newError(ErrorCodeInternal, "failed to load usage", err)
	}

	return ConversationUsageResult{
		TenantID:     identity.TenantID,
		PeriodStart:  start,
		PeriodEnd:    end,
		StartedCount: count,
	}, nil
}

func (s *Service) ListMessages(ctx context.Context, identity Identity, conversationID string, limit int) (ListMessagesResult, error) {
	if identity.UserID == "" || identity.TenantID == "" {
		return ListMessagesResult{}, newError(ErrorCodeUnauthorized, "invalid user identity", nil)
	}

	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ListMessagesResult{}, newError(ErrorCodeValidation, "conversationId is required", nil)
	}

	if limit <= 0 || limit > 200 {
		limit = 100
	}

	conversation, err := s.repo.GetConversation(ctx, identity.TenantID, conversationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ListMessagesResult{}, newError(ErrorCodeNotFound, "conversation not found", err)
		}
		return ListMessagesResult{}, newError(ErrorCodeInternal, "failed to fetch conversation", err)
	}

	messages, err := s.repo.ListMessages(ctx, identity.TenantID, conversationID, limit)
	if err != nil {
		return ListMessagesResult{}, newError(ErrorCodeInternal, "failed to list messages", err)
	}

	return ListMessagesResult{
		Conversation: conversation,
		Messages:     messages,
	}, nil
}

func (s *Service) ListVisitorMessages(ctx context.Context, token, conversationID string, limit int) (ListMessagesResult, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ListMessagesResult{}, newError(ErrorCodeValidation, "conversationId is required", nil)
	}

	if limit <= 0 || limit > 200 {
		limit = 100
	}

	access, err := s.ValidateVisitorAccess(token)
	if err != nil {
		return ListMessagesResult{}, err
	}

	if access.ConversationID != conversationID {
		return ListMessagesResult{}, newError(ErrorCodeForbidden, "token does not match conversation", nil)
	}

	conversation, err := s.repo.GetConversation(ctx, access.TenantID, conversationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ListMessagesResult{}, newError(ErrorCodeNotFound, "conversation not found", err)
		}
		return ListMessagesResult{}, newError(ErrorCodeInternal, "failed to fetch conversation", err)
	}

	if conversation.VisitorID != access.VisitorID {
		return ListMessagesResult{}, newError(ErrorCodeForbidden, "token does not match conversation", nil)
	}

	messages, err := s.repo.ListMessages(ctx, access.TenantID, conversationID, limit)
	if err != nil {
		return ListMessagesResult{}, newError(ErrorCodeInternal, "failed to list messages", err)
	}

	return ListMessagesResult{
		Conversation: conversation,
		Messages:     messages,
	}, nil
}

func (s *Service) ValidateVisitorAccess(token string) (VisitorAccess, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return VisitorAccess{}, newError(ErrorCodeUnauthorized, "visitor token required", nil)
	}

	claims, err := verifyVisitorToken(token, s.now)
	if err != nil {
		return VisitorAccess{}, newError(ErrorCodeUnauthorized, "invalid visitor token", err)
	}

	return VisitorAccess{
		TenantID:       claims.TenantID,
		ConversationID: claims.ConversationID,
		VisitorID:      claims.VisitorID,
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

func (s *Service) IdentityFromToken(token string) (Identity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Identity{}, newError(ErrorCodeUnauthorized, "empty token", nil)
	}
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

func signVisitorToken(claims visitorTokenClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, visitorTokenSecret)
	if _, err := mac.Write(payload); err != nil {
		return "", err
	}
	signature := mac.Sum(nil)

	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	sigPart := base64.RawURLEncoding.EncodeToString(signature)

	return fmt.Sprintf("%s.%s", payloadPart, sigPart), nil
}

func verifyVisitorToken(token string, now func() time.Time) (visitorTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return visitorTokenClaims{}, errors.New("invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return visitorTokenClaims{}, fmt.Errorf("decode payload: %w", err)
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return visitorTokenClaims{}, fmt.Errorf("decode signature: %w", err)
	}

	mac := hmac.New(sha256.New, visitorTokenSecret)
	if _, err := mac.Write(payload); err != nil {
		return visitorTokenClaims{}, fmt.Errorf("sign payload: %w", err)
	}

	if !hmac.Equal(sig, mac.Sum(nil)) {
		return visitorTokenClaims{}, errors.New("signature mismatch")
	}

	var claims visitorTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return visitorTokenClaims{}, fmt.Errorf("unmarshal claims: %w", err)
	}

	nowTime := now().UTC()
	if claims.ExpiresAt != 0 && nowTime.Unix() > claims.ExpiresAt {
		return visitorTokenClaims{}, errors.New("token expired")
	}

	return claims, nil
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func normalizeEmail(email string) string {
	email = strings.TrimSpace(email)
	return strings.ToLower(email)
}

func isValidEmail(email string) bool {
	if email == "" {
		return false
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	local, domain := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if local == "" || domain == "" {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
}
