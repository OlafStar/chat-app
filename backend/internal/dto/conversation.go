package dto

type ConversationMetadata struct {
	ConversationID string            `json:"conversationId"`
	VisitorID      string            `json:"visitorId"`
	VisitorName    string            `json:"visitorName,omitempty"`
	VisitorEmail   string            `json:"visitorEmail,omitempty"`
	Status         string            `json:"status"`
	AssignedUserID string            `json:"assignedUserId,omitempty"`
	CreatedAt      string            `json:"createdAt"`
	UpdatedAt      string            `json:"updatedAt"`
	LastMessageAt  string            `json:"lastMessageAt"`
	OriginURL      string            `json:"originUrl,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type MessageResponse struct {
	MessageID      string `json:"messageId"`
	ConversationID string `json:"conversationId"`
	SenderType     string `json:"senderType"`
	SenderID       string `json:"senderId"`
	Body           string `json:"body"`
	CreatedAt      string `json:"createdAt"`
}

type CreateConversationRequest struct {
	TenantID string                   `json:"tenantId"`
	Visitor  CreateVisitorPayload     `json:"visitor"`
	Message  CreateMessagePayload     `json:"message"`
	Metadata map[string]string        `json:"metadata,omitempty"`
	Origin   CreateConversationOrigin `json:"origin,omitempty"`
}

type CreateVisitorPayload struct {
	VisitorID string            `json:"visitorId,omitempty"`
	Name      string            `json:"name,omitempty"`
	Email     string            `json:"email,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type CreateMessagePayload struct {
	Body string `json:"body"`
}

type CreateConversationOrigin struct {
	URL string `json:"url,omitempty"`
}

type CreateConversationResponse struct {
	Conversation ConversationMetadata `json:"conversation"`
	VisitorToken string               `json:"visitorToken"`
	VisitorID    string               `json:"visitorId"`
	Message      MessageResponse      `json:"message"`
}

type PostVisitorMessageRequest struct {
	Body         string `json:"body"`
	VisitorToken string `json:"visitorToken"`
}

type AssignConversationEmailRequest struct {
	Email        string `json:"email"`
	VisitorToken string `json:"visitorToken"`
}

type AssignConversationEmailResponse struct {
	Conversation ConversationMetadata `json:"conversation"`
}

type PostAgentMessageRequest struct {
	Body string `json:"body"`
}

type ListConversationsResponse struct {
	Conversations []ConversationMetadata `json:"conversations"`
}

type ListMessagesResponse struct {
	Messages []MessageResponse `json:"messages"`
}
