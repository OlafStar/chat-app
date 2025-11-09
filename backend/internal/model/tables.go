package model

import "fmt"

const (
	TenantsTable       = "Tenants"
	UsersTable         = "Users"
	ConversationsTable = "Conversations"
	MessagesTable      = "Messages"
	VisitorsTable      = "Visitors"
	TenantInvitesTable = "TenantInvites"
	TenantAPIKeysTable = "TenantAPIKeys"
)

type TenantItem struct {
	TenantID string                 `dynamodbav:"tenantId"`
	Name     string                 `dynamodbav:"name"`
	Plan     string                 `dynamodbav:"plan"`
	Seats    int                    `dynamodbav:"seats"`
	Settings map[string]interface{} `dynamodbav:"settings,omitempty"`
	Created  string                 `dynamodbav:"createdAt"`
}

type UserItem struct {
	PK           string `dynamodbav:"pk"`
	TenantID     string `dynamodbav:"tenantId"`
	UserID       string `dynamodbav:"userId"`
	Email        string `dynamodbav:"email"`
	Name         string `dynamodbav:"name"`
	Role         string `dynamodbav:"role"`
	Status       string `dynamodbav:"status"`
	PasswordHash string `dynamodbav:"passwordHash"`
	CreatedAt    string `dynamodbav:"createdAt"`
}

type TenantInviteItem struct {
	Token       string `dynamodbav:"token"`
	TenantID    string `dynamodbav:"tenantId"`
	Email       string `dynamodbav:"email"`
	TenantEmail string `dynamodbav:"tenantEmail"`
	InvitedBy   string `dynamodbav:"invitedBy"`
	Role        string `dynamodbav:"role"`
	Status      string `dynamodbav:"status"`
	ExpiresAt   string `dynamodbav:"expiresAt"`
	CreatedAt   string `dynamodbav:"createdAt"`
}

type TenantAPIKeyItem struct {
	TenantID   string `dynamodbav:"tenantId"`
	KeyID      string `dynamodbav:"keyId"`
	APIKey     string `dynamodbav:"apiKey"`
	CreatedAt  string `dynamodbav:"createdAt"`
	LastUsedAt string `dynamodbav:"lastUsedAt,omitempty"`
}

func TenantScopedPK(tenantID, entityID string) string {
	return fmt.Sprintf("%s#%s", tenantID, entityID)
}
