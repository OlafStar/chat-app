package auth

import (
	internaljwt "chat-app-backend/internal/jwt"
	"chat-app-backend/internal/model"
)

type ErrorCode string

const (
	ErrorCodeValidation   ErrorCode = "validation_error"
	ErrorCodeUnauthorized ErrorCode = "unauthorized"
	ErrorCodeNotFound     ErrorCode = "not_found"
	ErrorCodeInternal     ErrorCode = "internal_error"
)

type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func newError(code ErrorCode, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

type RegisterParams struct {
	TenantName string
	OwnerName  string
	OwnerEmail string
	Password   string
}

type LoginParams struct {
	TenantID string
	Email    string
	Password string
}

type Identity struct {
	UserID   string
	TenantID string
	Email    string
}

type AuthResult struct {
	User        model.UserItem
	Tenant      model.TenantItem
	Tokens      internaljwt.TokenResponse
	Memberships []Membership
}

type ProfileResult struct {
	User   model.UserItem
	Tenant model.TenantItem
}

type Membership struct {
	User      model.UserItem
	Tenant    model.TenantItem
	IsDefault bool
}
