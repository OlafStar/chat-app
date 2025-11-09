package conversation

import (
	"chat-app-backend/internal/database"
	"chat-app-backend/internal/model"
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var ErrNotFound = errors.New("conversation repository: not found")

type Repository interface {
	GetTenant(ctx context.Context, tenantID string) (model.TenantItem, error)
	GetTenantByAPIKey(ctx context.Context, apiKey string) (model.TenantItem, error)
	GetUser(ctx context.Context, tenantID, userID string) (model.UserItem, error)
	GetVisitor(ctx context.Context, tenantID, visitorID string) (model.VisitorItem, error)
	PutVisitor(ctx context.Context, visitor model.VisitorItem) error
	CreateConversation(ctx context.Context, conversation model.ConversationItem) error
	UpdateConversationActivity(ctx context.Context, tenantID, conversationID, updatedAt, lastMessageAt string, assignedUserID *string) error
	UpdateConversationVisitorEmail(ctx context.Context, tenantID, conversationID, visitorEmail, updatedAt string) error
	GetConversation(ctx context.Context, tenantID, conversationID string) (model.ConversationItem, error)
	ListConversations(ctx context.Context, tenantID string, limit int) ([]model.ConversationItem, error)
	CountConversationsStartedBetween(ctx context.Context, tenantID string, start, end time.Time) (int, error)
	CreateMessage(ctx context.Context, message model.MessageItem) error
	ListMessages(ctx context.Context, tenantID, conversationID string, limit int) ([]model.MessageItem, error)
}

type DynamoRepository struct {
	db *database.Database
}

func NewDynamoRepository(db *database.Database) Repository {
	return &DynamoRepository{db: db}
}

func (r *DynamoRepository) GetTenant(ctx context.Context, tenantID string) (model.TenantItem, error) {
	var tenant model.TenantItem
	err := r.db.Client.GetItem(
		ctx,
		model.TenantsTable,
		map[string]types.AttributeValue{
			"tenantId": &types.AttributeValueMemberS{Value: tenantID},
		},
		&tenant,
	)
	if err != nil {
		if isNotFound(err) {
			return model.TenantItem{}, ErrNotFound
		}
		return model.TenantItem{}, err
	}
	return tenant, nil
}

func (r *DynamoRepository) GetUser(ctx context.Context, tenantID, userID string) (model.UserItem, error) {
	var user model.UserItem
	err := r.db.Client.GetItem(
		ctx,
		model.UsersTable,
		map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: model.TenantScopedPK(tenantID, userID)},
		},
		&user,
	)
	if err != nil {
		if isNotFound(err) {
			return model.UserItem{}, ErrNotFound
		}
		return model.UserItem{}, err
	}
	return user, nil
}

func (r *DynamoRepository) GetTenantByAPIKey(ctx context.Context, apiKey string) (model.TenantItem, error) {
	items, err := r.db.Client.QueryItems(
		ctx,
		model.TenantAPIKeysTable,
		aws.String("byApiKey"),
		"apiKey = :apiKey",
		map[string]types.AttributeValue{
			":apiKey": &types.AttributeValueMemberS{Value: apiKey},
		},
		nil,
		nil,
	)
	if err != nil && !isIndexNotFound(err) {
		return model.TenantItem{}, err
	}

	if len(items) > 0 {
		var key model.TenantAPIKeyItem
		if err := attributevalue.UnmarshalMap(items[0], &key); err != nil {
			return model.TenantItem{}, err
		}
		return r.GetTenant(ctx, key.TenantID)
	}

	legacyItems, err := r.db.Client.ScanItems(
		ctx,
		model.TenantsTable,
		"#apiKey = :apiKey",
		map[string]types.AttributeValue{
			":apiKey": &types.AttributeValueMemberS{Value: apiKey},
		},
		map[string]string{
			"#apiKey": "apiKey",
		},
	)
	if err != nil {
		return model.TenantItem{}, err
	}
	if len(legacyItems) == 0 {
		return model.TenantItem{}, ErrNotFound
	}

	var tenant model.TenantItem
	if err := attributevalue.UnmarshalMap(legacyItems[0], &tenant); err != nil {
		return model.TenantItem{}, err
	}

	return tenant, nil
}

func (r *DynamoRepository) GetVisitor(ctx context.Context, tenantID, visitorID string) (model.VisitorItem, error) {
	var visitor model.VisitorItem
	err := r.db.Client.GetItem(
		ctx,
		model.VisitorsTable,
		map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: model.VisitorPK(tenantID, visitorID)},
		},
		&visitor,
	)
	if err != nil {
		if isNotFound(err) {
			return model.VisitorItem{}, ErrNotFound
		}
		return model.VisitorItem{}, err
	}
	return visitor, nil
}

func (r *DynamoRepository) PutVisitor(ctx context.Context, visitor model.VisitorItem) error {
	return r.db.Client.PutItem(ctx, model.VisitorsTable, visitor)
}

func (r *DynamoRepository) CreateConversation(ctx context.Context, conversation model.ConversationItem) error {
	return r.db.Client.PutItem(ctx, model.ConversationsTable, conversation)
}

func (r *DynamoRepository) UpdateConversationActivity(ctx context.Context, tenantID, conversationID, updatedAt, lastMessageAt string, assignedUserID *string) error {
	updateExpr := "SET #updatedAt = :updatedAt, #lastMessageAt = :lastMessageAt"
	exprValues := map[string]types.AttributeValue{
		":updatedAt":     &types.AttributeValueMemberS{Value: updatedAt},
		":lastMessageAt": &types.AttributeValueMemberS{Value: lastMessageAt},
	}
	attrNames := map[string]string{
		"#updatedAt":     "updatedAt",
		"#lastMessageAt": "lastMessageAt",
	}

	if assignedUserID != nil {
		updateExpr += ", #assignedUserId = :assignedUserId"
		exprValues[":assignedUserId"] = &types.AttributeValueMemberS{Value: *assignedUserID}
		attrNames["#assignedUserId"] = "assignedUserId"
	}

	return r.db.Client.UpdateItem(
		ctx,
		model.ConversationsTable,
		map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: model.ConversationPK(tenantID, conversationID)},
		},
		updateExpr,
		exprValues,
		attrNames,
		nil,
	)
}

func (r *DynamoRepository) UpdateConversationVisitorEmail(ctx context.Context, tenantID, conversationID, visitorEmail, updatedAt string) error {
	return r.db.Client.UpdateItem(
		ctx,
		model.ConversationsTable,
		map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: model.ConversationPK(tenantID, conversationID)},
		},
		"SET #visitorEmail = :visitorEmail, #updatedAt = :updatedAt",
		map[string]types.AttributeValue{
			":visitorEmail": &types.AttributeValueMemberS{Value: visitorEmail},
			":updatedAt":    &types.AttributeValueMemberS{Value: updatedAt},
		},
		map[string]string{
			"#visitorEmail": "visitorEmail",
			"#updatedAt":    "updatedAt",
		},
		nil,
	)
}

func (r *DynamoRepository) GetConversation(ctx context.Context, tenantID, conversationID string) (model.ConversationItem, error) {
	var conversation model.ConversationItem
	err := r.db.Client.GetItem(
		ctx,
		model.ConversationsTable,
		map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: model.ConversationPK(tenantID, conversationID)},
		},
		&conversation,
	)
	if err != nil {
		if isNotFound(err) {
			return model.ConversationItem{}, ErrNotFound
		}
		return model.ConversationItem{}, err
	}
	return conversation, nil
}

func (r *DynamoRepository) ListConversations(ctx context.Context, tenantID string, limit int) ([]model.ConversationItem, error) {
	scanForward := false
	items, err := r.db.Client.QueryItems(
		ctx,
		model.ConversationsTable,
		aws.String("byTenant"),
		"tenantId = :tenantId",
		map[string]types.AttributeValue{
			":tenantId": &types.AttributeValueMemberS{Value: tenantID},
		},
		nil,
		&scanForward,
	)
	if err != nil && !isIndexNotFound(err) {
		return nil, err
	}

	if (err != nil && isIndexNotFound(err)) || items == nil {
		items, err = r.db.Client.ScanItems(
			ctx,
			model.ConversationsTable,
			"tenantId = :tenantId",
			map[string]types.AttributeValue{
				":tenantId": &types.AttributeValueMemberS{Value: tenantID},
			},
			nil,
		)
		if err != nil {
			return nil, err
		}
	}

	conversations := make([]model.ConversationItem, 0, len(items))
	for _, item := range items {
		var conversation model.ConversationItem
		if err := attributevalue.UnmarshalMap(item, &conversation); err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}

	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].LastMessageAt > conversations[j].LastMessageAt
	})

	if limit > 0 && len(conversations) > limit {
		conversations = conversations[:limit]
	}

	return conversations, nil
}

func (r *DynamoRepository) CountConversationsStartedBetween(ctx context.Context, tenantID string, start, end time.Time) (int, error) {
	if tenantID == "" {
		return 0, errors.New("tenantID is required")
	}

	items, err := r.db.Client.QueryAll(
		ctx,
		model.ConversationsTable,
		aws.String("byTenant"),
		"tenantId = :tenantId",
		map[string]types.AttributeValue{
			":tenantId": &types.AttributeValueMemberS{Value: tenantID},
		},
	)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, item := range items {
		var conversation model.ConversationItem
		if err := attributevalue.UnmarshalMap(item, &conversation); err != nil {
			return 0, err
		}
		created := parseTime(conversation.CreatedAt)
		if created.IsZero() {
			continue
		}
		if (created.Equal(start) || created.After(start)) && created.Before(end) {
			count++
		}
	}

	return count, nil
}

func (r *DynamoRepository) CreateMessage(ctx context.Context, message model.MessageItem) error {
	return r.db.Client.PutItem(ctx, model.MessagesTable, message)
}

func (r *DynamoRepository) ListMessages(ctx context.Context, tenantID, conversationID string, limit int) ([]model.MessageItem, error) {
	scanForward := true
	items, err := r.db.Client.QueryItems(
		ctx,
		model.MessagesTable,
		aws.String("byConversation"),
		"conversationId = :conversationId",
		map[string]types.AttributeValue{
			":conversationId": &types.AttributeValueMemberS{Value: conversationID},
		},
		nil,
		&scanForward,
	)
	if err != nil && !isIndexNotFound(err) {
		return nil, err
	}

	if (err != nil && isIndexNotFound(err)) || items == nil {
		items, err = r.db.Client.ScanItems(
			ctx,
			model.MessagesTable,
			"conversationId = :conversationId",
			map[string]types.AttributeValue{
				":conversationId": &types.AttributeValueMemberS{Value: conversationID},
			},
			nil,
		)
		if err != nil {
			return nil, err
		}
	}

	messages := make([]model.MessageItem, 0, len(items))
	for _, item := range items {
		var message model.MessageItem
		if err := attributevalue.UnmarshalMap(item, &message); err != nil {
			return nil, err
		}
		if message.TenantID != "" && message.TenantID != tenantID {
			continue
		}
		messages = append(messages, message)
	}

	sort.Slice(messages, func(i, j int) bool {
		ti := parseTime(messages[i].CreatedAt)
		tj := parseTime(messages[j].CreatedAt)
		return ti.Before(tj)
	})

	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	return messages, nil
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "item not found")
}

func isIndexNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "index") && strings.Contains(msg, "not") && strings.Contains(msg, "found")
}

func parseTime(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}
	}
	return t
}
