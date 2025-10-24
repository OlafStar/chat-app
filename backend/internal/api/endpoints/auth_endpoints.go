package endpoints

import (
	"chat-app-backend/internal/database"
	"chat-app-backend/internal/dto"
	"chat-app-backend/internal/model"
	authsvc "chat-app-backend/internal/service/auth"
	"encoding/json"
	"fmt"
	"net/http"
)

type AuthEndpoints interface {
	Register(http.ResponseWriter, *http.Request) error
	Login(http.ResponseWriter, *http.Request) error
	Me(http.ResponseWriter, *http.Request) error
	Switch(http.ResponseWriter, *http.Request) error
}

type authEndpoints struct {
	service *authsvc.Service
}

func NewAuthEndpoints(db *database.Database) AuthEndpoints {
	return &authEndpoints{
		service: authsvc.New(db),
	}
}

func (h *authEndpoints) Register(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodPost: h.handleRegister,
	})
}

func (h *authEndpoints) Login(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodPost: h.handleLogin,
	})
}

func (h *authEndpoints) Me(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodGet: h.handleMe,
	})
}

func (h *authEndpoints) Switch(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodPost: h.handleSwitch,
	})
}

func (h *authEndpoints) handleRegister(w http.ResponseWriter, r *http.Request) error {
	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode register request: %w", err),
		}
	}

	result, err := h.service.Register(r.Context(), authsvc.RegisterParams{
		TenantName: req.TenantName,
		OwnerName:  req.Name,
		OwnerEmail: req.Email,
		Password:   req.Password,
	})
	if err != nil {
		return h.serviceError(err)
	}

	return WriteJSON(w, http.StatusCreated, toAuthResponse(result))
}

func (h *authEndpoints) handleLogin(w http.ResponseWriter, r *http.Request) error {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode login request: %w", err),
		}
	}

	result, err := h.service.Login(r.Context(), authsvc.LoginParams{
		TenantID: req.TenantID,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return h.serviceError(err)
	}

	return WriteJSON(w, http.StatusOK, toAuthResponse(result))
}

func (h *authEndpoints) handleMe(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	profile, err := h.service.Me(r.Context(), identity)
	if err != nil {
		return h.serviceError(err)
	}

	return WriteJSON(w, http.StatusOK, toMeResponse(profile))
}

func (h *authEndpoints) handleSwitch(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	var req dto.SwitchTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode switch tenant request: %w", err),
		}
	}

	result, err := h.service.SwitchTenant(r.Context(), identity, req.TenantID)
	if err != nil {
		return h.serviceError(err)
	}

	return WriteJSON(w, http.StatusOK, toAuthResponse(result))
}

func (h *authEndpoints) serviceError(err error) error {
	if err == nil {
		return nil
	}

	svcErr, ok := err.(*authsvc.Error)
	if !ok {
		return &HTTPError{
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal server error",
			ErrorLog:   fmt.Errorf("auth service: %w", err),
		}
	}

	var errorLog error
	if svcErr.Err != nil {
		errorLog = fmt.Errorf("%s: %w", svcErr.Message, svcErr.Err)
	} else {
		errorLog = svcErr
	}

	switch svcErr.Code {
	case authsvc.ErrorCodeValidation:
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    svcErr.Message,
			ErrorLog:   errorLog,
		}
	case authsvc.ErrorCodeUnauthorized:
		return &HTTPError{
			StatusCode: http.StatusUnauthorized,
			Message:    svcErr.Message,
			ErrorLog:   errorLog,
		}
	case authsvc.ErrorCodeNotFound:
		return &HTTPError{
			StatusCode: http.StatusNotFound,
			Message:    svcErr.Message,
			ErrorLog:   errorLog,
		}
	default:
		return &HTTPError{
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal server error",
			ErrorLog:   errorLog,
		}
	}
}

func toAuthResponse(result authsvc.AuthResult) dto.AuthResponse {
	resp := dto.AuthResponse{
		AccessToken:  result.Tokens.AccessToken,
		RefreshToken: result.Tokens.RefreshToken,
		User:         toUserResponse(result.User),
		Tenant:       toTenantResponse(result.Tenant),
	}

	if len(result.Memberships) > 0 {
		resp.Tenants = toTenantMemberships(result.Memberships)
	}

	return resp
}

func toMeResponse(result authsvc.ProfileResult) dto.MeResponse {
	return dto.MeResponse{
		User:   toUserResponse(result.User),
		Tenant: toTenantResponse(result.Tenant),
	}
}

func toUserResponse(user model.UserItem) dto.UserResponse {
	return dto.UserResponse{
		UserID:    user.UserID,
		TenantID:  user.TenantID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
	}
}

func toTenantResponse(tenant model.TenantItem) dto.TenantResponse {
	return toTenantResponseWithRemaining(tenant, nil)
}

func toTenantResponseWithRemaining(tenant model.TenantItem, remaining *int) dto.TenantResponse {
	return dto.TenantResponse{
		TenantID:       tenant.TenantID,
		Name:           tenant.Name,
		Plan:           tenant.Plan,
		Seats:          tenant.Seats,
		CreatedAt:      tenant.Created,
		RemainingSeats: remaining,
	}
}

func toTenantMemberships(memberships []authsvc.Membership) []dto.TenantMembership {
	resp := make([]dto.TenantMembership, 0, len(memberships))
	for _, membership := range memberships {
		resp = append(resp, dto.TenantMembership{
			UserID:    membership.User.UserID,
			TenantID:  membership.Tenant.TenantID,
			Name:      membership.Tenant.Name,
			Plan:      membership.Tenant.Plan,
			Seats:     membership.Tenant.Seats,
			Role:      membership.User.Role,
			Status:    membership.User.Status,
			IsDefault: membership.IsDefault,
		})
	}
	return resp
}
