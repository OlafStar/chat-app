package tenant

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

var ErrNotFound = errors.New("tenant repository: not found")

type Repository interface {
	GetTenant(ctx context.Context, tenantID string) (model.TenantItem, error)
	UpdateTenantName(ctx context.Context, tenantID, name string) (model.TenantItem, error)
	GetUser(ctx context.Context, tenantID, userID string) (model.UserItem, error)
	ListUsersByTenant(ctx context.Context, tenantID string) ([]model.UserItem, error)
	ListUsersByEmail(ctx context.Context, email string) ([]model.UserItem, error)
	CreateUser(ctx context.Context, user model.UserItem) error
	FindUserByEmail(ctx context.Context, tenantID, email string) (model.UserItem, error)
	CreateInvite(ctx context.Context, invite model.TenantInviteItem) error
	GetInvite(ctx context.Context, token string) (model.TenantInviteItem, error)
	ListInvitesByEmail(ctx context.Context, email string) ([]model.TenantInviteItem, error)
	FindActiveInvite(ctx context.Context, tenantID, email string) (model.TenantInviteItem, error)
	UpdateInviteStatus(ctx context.Context, token, status string) error
	DeleteInvite(ctx context.Context, token string) error
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
		if isNotFoundError(err) {
			return model.TenantItem{}, ErrNotFound
		}
		return model.TenantItem{}, err
	}
	return tenant, nil
}

func (r *DynamoRepository) UpdateTenantName(ctx context.Context, tenantID, name string) (model.TenantItem, error) {
	var updated model.TenantItem
	err := r.db.Client.UpdateItem(
		ctx,
		model.TenantsTable,
		map[string]types.AttributeValue{
			"tenantId": &types.AttributeValueMemberS{Value: tenantID},
		},
		"SET #name = :name",
		map[string]types.AttributeValue{
			":name": &types.AttributeValueMemberS{Value: name},
		},
		map[string]string{
			"#name": "name",
		},
		&updated,
	)
	if err != nil {
		return model.TenantItem{}, err
	}
	return updated, nil
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

func (r *DynamoRepository) ListUsersByTenant(ctx context.Context, tenantID string) ([]model.UserItem, error) {
	items, err := r.db.Client.QueryItems(
		ctx,
		model.UsersTable,
		aws.String("byTenant"),
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

func (r *DynamoRepository) CreateUser(ctx context.Context, user model.UserItem) error {
	return r.db.Client.PutItem(ctx, model.UsersTable, user)
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

func (r *DynamoRepository) CreateInvite(ctx context.Context, invite model.TenantInviteItem) error {
	return r.db.Client.PutItem(ctx, model.TenantInvitesTable, invite)
}

func (r *DynamoRepository) GetInvite(ctx context.Context, token string) (model.TenantInviteItem, error) {
	var invite model.TenantInviteItem
	err := r.db.Client.GetItem(
		ctx,
		model.TenantInvitesTable,
		map[string]types.AttributeValue{
			"token": &types.AttributeValueMemberS{Value: token},
		},
		&invite,
	)
	if err != nil {
		if isNotFoundError(err) {
			return model.TenantInviteItem{}, ErrNotFound
		}
		return model.TenantInviteItem{}, err
	}
	return invite, nil
}

func (r *DynamoRepository) ListInvitesByEmail(ctx context.Context, email string) ([]model.TenantInviteItem, error) {
	items, err := r.db.Client.ScanItems(
		ctx,
		model.TenantInvitesTable,
		"email = :email",
		map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
		nil,
	)
	if err != nil {
		return nil, err
	}

	invites := make([]model.TenantInviteItem, 0, len(items))
	for _, item := range items {
		var invite model.TenantInviteItem
		if err := attributevalue.UnmarshalMap(item, &invite); err != nil {
			return nil, err
		}
		invites = append(invites, invite)
	}

	return invites, nil
}

func (r *DynamoRepository) FindActiveInvite(ctx context.Context, tenantID, email string) (model.TenantInviteItem, error) {
	tenantEmail := model.TenantScopedPK(tenantID, email)
	filter := "#status = :status"
	items, err := r.db.Client.QueryItemsWithFilter(
		ctx,
		model.TenantInvitesTable,
		aws.String("byTenantEmail"),
		"tenantEmail = :tenantEmail",
		&filter,
		map[string]types.AttributeValue{
			":tenantEmail": &types.AttributeValueMemberS{Value: tenantEmail},
			":status":      &types.AttributeValueMemberS{Value: "pending"},
		},
		map[string]string{
			"#status": "status",
		},
	)
	if err != nil {
		return model.TenantInviteItem{}, err
	}
	if len(items) == 0 {
		return model.TenantInviteItem{}, ErrNotFound
	}

	var invite model.TenantInviteItem
	if err := attributevalue.UnmarshalMap(items[0], &invite); err != nil {
		return model.TenantInviteItem{}, err
	}

	return invite, nil
}

func (r *DynamoRepository) UpdateInviteStatus(ctx context.Context, token, status string) error {
	return r.db.Client.UpdateItem(
		ctx,
		model.TenantInvitesTable,
		map[string]types.AttributeValue{
			"token": &types.AttributeValueMemberS{Value: token},
		},
		"SET #status = :status",
		map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: status},
		},
		map[string]string{
			"#status": "status",
		},
		nil,
	)
}

func (r *DynamoRepository) DeleteInvite(ctx context.Context, token string) error {
	return r.db.Client.DeleteItem(
		ctx,
		model.TenantInvitesTable,
		map[string]types.AttributeValue{
			"token": &types.AttributeValueMemberS{Value: token},
		},
	)
}

func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "item not found")
}
