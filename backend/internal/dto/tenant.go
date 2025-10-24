package dto

type UpdateTenantRequest struct {
	Name string `json:"name"`
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
