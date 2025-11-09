package dto

type UpdateTenantRequest struct {
	Name string `json:"name"`
}

type TenantSettingsResponse struct {
	Widget WidgetSettingsResponse `json:"widget"`
}

type WidgetSettingsResponse struct {
	BubbleText string `json:"bubbleText"`
	HeaderText string `json:"headerText"`
	ThemeColor string `json:"themeColor"`
}

type UpdateWidgetSettingsRequest struct {
	BubbleText string `json:"bubbleText"`
	HeaderText string `json:"headerText"`
	ThemeColor string `json:"themeColor"`
}

type WidgetSettingsResultResponse struct {
	Widget WidgetSettingsResponse `json:"widget"`
}

type AddTenantUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password,omitempty"`
	Role     string `json:"role,omitempty"`
}

type TenantInviteResponse struct {
	Token     string `json:"token"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expiresAt"`
}

type AddTenantUserResponse struct {
	User           *UserResponse         `json:"user,omitempty"`
	Invite         *TenantInviteResponse `json:"invite,omitempty"`
	RemainingSeats int                   `json:"remainingSeats"`
}

type AcceptInviteRequest struct {
	Token string `json:"token"`
	Name  string `json:"name,omitempty"`
}

type AcceptInviteResponse struct {
	User           UserResponse   `json:"user"`
	Tenant         TenantResponse `json:"tenant"`
	RemainingSeats int            `json:"remainingSeats"`
}

type PendingInviteResponse struct {
	Token     string         `json:"token"`
	Email     string         `json:"email"`
	Role      string         `json:"role"`
	InvitedBy string         `json:"invitedBy,omitempty"`
	CreatedAt string         `json:"createdAt,omitempty"`
	ExpiresAt string         `json:"expiresAt"`
	Tenant    TenantResponse `json:"tenant"`
}

type ListPendingInvitesResponse struct {
	Invites []PendingInviteResponse `json:"invites"`
}

type TenantAPIKey struct {
	KeyID     string `json:"keyId"`
	APIKey    string `json:"apiKey"`
	CreatedAt string `json:"createdAt"`
}

type TenantAPIKeyListResponse struct {
	Keys []TenantAPIKey `json:"keys"`
}

type CreateTenantAPIKeyResponse struct {
	Key TenantAPIKey `json:"key"`
}

type DeleteTenantAPIKeyRequest struct {
	KeyID string `json:"keyId"`
}
