package dto

type RegisterRequest struct {
	TenantName string `json:"tenantName"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

type LoginRequest struct {
	TenantID string `json:"tenantId,omitempty"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SwitchTenantRequest struct {
	TenantID string `json:"tenantId"`
}

type TenantResponse struct {
	TenantID  string `json:"tenantId"`
	Name      string `json:"name"`
	Plan      string `json:"plan"`
	Seats     int    `json:"seats"`
	CreatedAt string `json:"createdAt"`
	// RemainingSeats surfaces the number of additional active users that can be
	// provisioned within the tenant (owner included in the seat calculation).
	// It is omitted when not computed by the backing service.
	RemainingSeats *int `json:"remainingSeats,omitempty"`
}

type UserResponse struct {
	UserID    string `json:"userId"`
	TenantID  string `json:"tenantId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type AuthResponse struct {
	AccessToken  string             `json:"accessToken"`
	RefreshToken string             `json:"refreshToken,omitempty"`
	User         UserResponse       `json:"user"`
	Tenant       TenantResponse     `json:"tenant"`
	APIKeys      []TenantAPIKey     `json:"apiKeys,omitempty"`
	Tenants      []TenantMembership `json:"tenants,omitempty"`
}

type MeResponse struct {
	User   UserResponse   `json:"user"`
	Tenant TenantResponse `json:"tenant"`
}

type TenantMembership struct {
	UserID    string `json:"userId"`
	TenantID  string `json:"tenantId"`
	Name      string `json:"name"`
	Plan      string `json:"plan"`
	Seats     int    `json:"seats"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	IsDefault bool   `json:"isDefault"`
}
