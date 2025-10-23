package jwt

type Role int

type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

type RegisterUser struct {
	Email string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	Id string `json:"id"`
	Email string `json:"email"`
	PasswordHash string `json:"password"`
}

type UserType string

type UserRegisterer interface {
	GetEmail() string
	GetPassword() string
}