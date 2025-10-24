package model

import "fmt"

const (
	TenantsTable           = "Tenants"
	UsersTable             = "Users"
	ConversationsTable     = "Conversations"
	MessagesTable          = "Messages"
	VisitorsTable          = "Visitors"
	WebhooksTable          = "Webhooks"
	WebhookDeliveriesTable = "WebhookDeliveries"
	AuditLogTable          = "AuditLog"
	AnalyticsDailyTable    = "AnalyticsDaily"
	TenantInvitesTable     = "TenantInvites"
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

func TenantScopedPK(tenantID, entityID string) string {
	return fmt.Sprintf("%s#%s", tenantID, entityID)
}
