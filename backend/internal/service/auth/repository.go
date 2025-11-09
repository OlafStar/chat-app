package auth

import (
	"chat-app-backend/internal/database"
	"chat-app-backend/internal/model"
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var ErrNotFound = errors.New("auth repository: not found")

type Repository interface {
	CreateTenant(ctx context.Context, tenant model.TenantItem) error
	CreateUser(ctx context.Context, user model.UserItem) error
	FindUserByEmail(ctx context.Context, tenantID, email string) (model.UserItem, error)
	ListUsersByEmail(ctx context.Context, email string) ([]model.UserItem, error)
	GetTenant(ctx context.Context, tenantID string) (model.TenantItem, error)
	GetUser(ctx context.Context, tenantID, userID string) (model.UserItem, error)
	CreateTenantAPIKey(ctx context.Context, item model.TenantAPIKeyItem) error
	ListTenantAPIKeys(ctx context.Context, tenantID string) ([]model.TenantAPIKeyItem, error)
}

type DynamoRepository struct {
	db *database.Database
}

func NewDynamoRepository(db *database.Database) Repository {
	return &DynamoRepository{db: db}
}

func (r *DynamoRepository) CreateTenant(ctx context.Context, tenant model.TenantItem) error {
	return r.db.Client.PutItem(ctx, model.TenantsTable, tenant)
}

func (r *DynamoRepository) CreateUser(ctx context.Context, user model.UserItem) error {
	return r.db.Client.PutItem(ctx, model.UsersTable, user)
}

func (r *DynamoRepository) FindUserByEmail(ctx context.Context, tenantID, email string) (model.UserItem, error) {
	items, err := r.db.Client.QueryItems(
		ctx,
		model.UsersTable,
		aws.String("byEmail"),
		"email = :email AND tenantId = :tenantId",
		map[string]types.AttributeValue{
			":email":    &types.AttributeValueMemberS{Value: email},
			":tenantId": &types.AttributeValueMemberS{Value: tenantID},
		},
		nil,
		nil,
	)
	if err != nil {
		return model.UserItem{}, err
	}

	if len(items) == 0 {
		return model.UserItem{}, ErrNotFound
	}

	var user model.UserItem
	if err := attributevalue.UnmarshalMap(items[0], &user); err != nil {
		return model.UserItem{}, err
	}

	return user, nil
}

func (r *DynamoRepository) ListUsersByEmail(ctx context.Context, email string) ([]model.UserItem, error) {
	items, err := r.db.Client.QueryItems(
		ctx,
		model.UsersTable,
		aws.String("byEmail"),
		"email = :email",
		map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}

	users := make([]model.UserItem, 0, len(items))
	for _, item := range items {
		var user model.UserItem
		if err := attributevalue.UnmarshalMap(item, &user); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
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
		if isNotFoundError(err) {
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
		if isNotFoundError(err) {
			return model.UserItem{}, ErrNotFound
		}
		return model.UserItem{}, err
	}

	return user, nil
}

func (r *DynamoRepository) CreateTenantAPIKey(ctx context.Context, item model.TenantAPIKeyItem) error {
	return r.db.Client.PutItem(ctx, model.TenantAPIKeysTable, item)
}

func (r *DynamoRepository) ListTenantAPIKeys(ctx context.Context, tenantID string) ([]model.TenantAPIKeyItem, error) {
	items, err := r.db.Client.QueryItems(
		ctx,
		model.TenantAPIKeysTable,
		nil,
		"tenantId = :tenantId",
		map[string]types.AttributeValue{
			":tenantId": &types.AttributeValueMemberS{Value: tenantID},
		},
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}

	keys := make([]model.TenantAPIKeyItem, 0, len(items))
	for _, item := range items {
		var key model.TenantAPIKeyItem
		if err := attributevalue.UnmarshalMap(item, &key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, nil
}

func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "item not found")
}
