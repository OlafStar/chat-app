package model

import "fmt"

type ConversationStatus string

const (
	ConversationStatusOpen   ConversationStatus = "open"
	ConversationStatusClosed ConversationStatus = "closed"
)

func ConversationPK(tenantID, conversationID string) string {
	return fmt.Sprintf("%s#%s", tenantID, conversationID)
}

func MessagePK(conversationID, messageID string) string {
	return fmt.Sprintf("%s#%s", conversationID, messageID)
}

func VisitorPK(tenantID, visitorID string) string {
	return fmt.Sprintf("%s#%s", tenantID, visitorID)
}

type ConversationItem struct {
	PK             string             `dynamodbav:"pk"`
	ConversationID string             `dynamodbav:"conversationId"`
	TenantID       string             `dynamodbav:"tenantId"`
	VisitorID      string             `dynamodbav:"visitorId"`
	VisitorName    string             `dynamodbav:"visitorName,omitempty"`
	VisitorEmail   string             `dynamodbav:"visitorEmail,omitempty"`
	Status         ConversationStatus `dynamodbav:"status"`
	AssignedUserID string             `dynamodbav:"assignedUserId,omitempty"`
	Metadata       map[string]string  `dynamodbav:"metadata,omitempty"`
	OriginURL      string             `dynamodbav:"originUrl,omitempty"`
	CreatedAt      string             `dynamodbav:"createdAt"`
	UpdatedAt      string             `dynamodbav:"updatedAt"`
	LastMessageAt  string             `dynamodbav:"lastMessageAt"`
}

type MessageItem struct {
	PK             string `dynamodbav:"pk"`
	TenantID       string `dynamodbav:"tenantId"`
	ConversationID string `dynamodbav:"conversationId"`
	MessageID      string `dynamodbav:"messageId"`
	SenderType     string `dynamodbav:"senderType"`
	SenderID       string `dynamodbav:"senderId"`
	Body           string `dynamodbav:"body"`
	CreatedAt      string `dynamodbav:"createdAt"`
}

type VisitorItem struct {
	PK         string            `dynamodbav:"pk"`
	TenantID   string            `dynamodbav:"tenantId"`
	VisitorID  string            `dynamodbav:"visitorId"`
	Name       string            `dynamodbav:"name,omitempty"`
	Email      string            `dynamodbav:"email,omitempty"`
	Metadata   map[string]string `dynamodbav:"metadata,omitempty"`
	CreatedAt  string            `dynamodbav:"createdAt"`
	LastSeenAt string            `dynamodbav:"lastSeenAt"`
}
