package endpoints

import (
	"chat-app-backend/internal/dto"
	tenantservice "chat-app-backend/internal/service/tenant"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TenantEndpoints interface {
	UpdateTenant(http.ResponseWriter, *http.Request) error
	AddTenantUser(http.ResponseWriter, *http.Request) error
	AcceptInvite(http.ResponseWriter, *http.Request) error
	ListPendingInvites(http.ResponseWriter, *http.Request) error
	TenantAPIKeys(http.ResponseWriter, *http.Request) error
}

type tenantEndpoints struct {
	service *tenantservice.Service
}

func NewTenantEndpoints(service *tenantservice.Service) TenantEndpoints {
	return &tenantEndpoints{
		service: service,
	}
}

func (h *tenantEndpoints) UpdateTenant(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodPatch: h.handleUpdateTenant,
	})
}

func (h *tenantEndpoints) AddTenantUser(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodPost: h.handleAddTenantUser,
	})
}

func (h *tenantEndpoints) AcceptInvite(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodPost: h.handleAcceptInvite,
	})
}

func (h *tenantEndpoints) ListPendingInvites(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodGet: h.handleListPendingInvites,
	})
}

func (h *tenantEndpoints) TenantAPIKeys(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodGet:    h.handleListTenantAPIKeys,
		http.MethodPost:   h.handleCreateTenantAPIKey,
		http.MethodDelete: h.handleDeleteTenantAPIKey,
	})
}

func (h *tenantEndpoints) handleUpdateTenant(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	var req dto.UpdateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode update tenant request: %w", err),
		}
	}

	result, err := h.service.UpdateTenantName(r.Context(), identity, identity.TenantID, req.Name)
	if err != nil {
		return h.serviceError(err)
	}

	remaining := result.RemainingSeats
	return WriteJSON(w, http.StatusOK, toTenantResponseWithRemaining(result.Tenant, &remaining))
}

func (h *tenantEndpoints) handleAddTenantUser(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	var req dto.AddTenantUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode add tenant user request: %w", err),
		}
	}

	result, err := h.service.AddUser(r.Context(), identity, identity.TenantID, tenantservice.AddUserParams{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		return h.serviceError(err)
	}

	resp := dto.AddTenantUserResponse{
		RemainingSeats: result.RemainingSeats,
	}

	if result.User != nil {
		userResp := toUserResponse(*result.User)
		resp.User = &userResp
	}
	if result.Invite != nil {
		resp.Invite = toInviteResponse(result.Invite)
	}

	return WriteJSON(w, http.StatusCreated, resp)
}

func (h *tenantEndpoints) handleListTenantAPIKeys(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	keys, err := h.service.ListTenantAPIKeys(r.Context(), identity, identity.TenantID)
	if err != nil {
		return h.serviceError(err)
	}

	return WriteJSON(w, http.StatusOK, dto.TenantAPIKeyListResponse{
		Keys: toTenantAPIKeyResponse(keys),
	})
}

func (h *tenantEndpoints) handleCreateTenantAPIKey(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	key, err := h.service.CreateTenantAPIKey(r.Context(), identity, identity.TenantID)
	if err != nil {
		return h.serviceError(err)
	}

	return WriteJSON(w, http.StatusCreated, dto.CreateTenantAPIKeyResponse{
		Key: toTenantAPIKeyDTO(key),
	})
}

func (h *tenantEndpoints) handleDeleteTenantAPIKey(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	var req dto.DeleteTenantAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode delete tenant api key request: %w", err),
		}
	}

	if err := h.service.DeleteTenantAPIKey(r.Context(), identity, identity.TenantID, req.KeyID); err != nil {
		return h.serviceError(err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func toTenantAPIKeyResponse(keys []tenantservice.TenantAPIKey) []dto.TenantAPIKey {
	resp := make([]dto.TenantAPIKey, 0, len(keys))
	for _, key := range keys {
		resp = append(resp, toTenantAPIKeyDTO(key))
	}
	return resp
}

func toTenantAPIKeyDTO(key tenantservice.TenantAPIKey) dto.TenantAPIKey {
	createdAt := ""
	if !key.CreatedAt.IsZero() {
		createdAt = key.CreatedAt.Format(time.RFC3339)
	}
	return dto.TenantAPIKey{
		KeyID:     key.KeyID,
		APIKey:    key.APIKey,
		CreatedAt: createdAt,
	}
}

func toInviteResponse(invite *tenantservice.InviteResult) *dto.TenantInviteResponse {
	if invite == nil {
		return nil
	}
	resp := dto.TenantInviteResponse{
		Token:     invite.Token,
		Email:     invite.Email,
		Role:      invite.Role,
		ExpiresAt: invite.ExpiresAt.Format(time.RFC3339),
	}
	return &resp
}

func toPendingInviteResponse(invite tenantservice.PendingInvite) dto.PendingInviteResponse {
	resp := dto.PendingInviteResponse{
		Token:     invite.Token,
		Email:     invite.Email,
		Role:      invite.Role,
		InvitedBy: invite.InvitedBy,
		ExpiresAt: invite.ExpiresAt.Format(time.RFC3339),
		Tenant:    toTenantResponse(invite.Tenant),
	}
	if !invite.CreatedAt.IsZero() {
		resp.CreatedAt = invite.CreatedAt.Format(time.RFC3339)
	}
	return resp
}

func (h *tenantEndpoints) handleAcceptInvite(w http.ResponseWriter, r *http.Request) error {
	var req dto.AcceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode accept invite request: %w", err),
		}
	}

	result, err := h.service.AcceptInvite(r.Context(), req.Token, req.Name)
	if err != nil {
		return h.serviceError(err)
	}
	if result.User == nil || result.Tenant == nil {
		return &HTTPError{
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal server error",
			ErrorLog:   fmt.Errorf("accept invite returned empty result"),
		}
	}

	remaining := result.RemainingSeats
	userResp := toUserResponse(*result.User)
	return WriteJSON(w, http.StatusCreated, dto.AcceptInviteResponse{
		User:           userResp,
		Tenant:         toTenantResponseWithRemaining(*result.Tenant, &remaining),
		RemainingSeats: remaining,
	})
}

func (h *tenantEndpoints) handleListPendingInvites(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return h.serviceError(err)
	}

	invites, err := h.service.ListPendingInvites(r.Context(), identity)
	if err != nil {
		return h.serviceError(err)
	}

	resp := dto.ListPendingInvitesResponse{
		Invites: make([]dto.PendingInviteResponse, 0, len(invites)),
	}
	for _, invite := range invites {
		resp.Invites = append(resp.Invites, toPendingInviteResponse(invite))
	}

	return WriteJSON(w, http.StatusOK, resp)
}

func (h *tenantEndpoints) serviceError(err error) error {
	return mapTenantServiceError(err)
}

func mapTenantServiceError(err error) error {
	if err == nil {
		return nil
	}

	svcErr, ok := err.(*tenantservice.Error)
	if !ok {
		return &HTTPError{
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal server error",
			ErrorLog:   fmt.Errorf("tenant service: %w", err),
		}
	}

	var errorLog error
	if svcErr.Err != nil {
		errorLog = fmt.Errorf("%s: %w", svcErr.Message, svcErr.Err)
	} else {
		errorLog = svcErr
	}

	switch svcErr.Code {
	case tenantservice.ErrorCodeValidation:
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    svcErr.Message,
			ErrorLog:   errorLog,
		}
	case tenantservice.ErrorCodeUnauthorized:
		return &HTTPError{
			StatusCode: http.StatusUnauthorized,
			Message:    svcErr.Message,
			ErrorLog:   errorLog,
		}
	case tenantservice.ErrorCodeForbidden:
		return &HTTPError{
			StatusCode: http.StatusForbidden,
			Message:    svcErr.Message,
			ErrorLog:   errorLog,
		}
	case tenantservice.ErrorCodeNotFound:
		return &HTTPError{
			StatusCode: http.StatusNotFound,
			Message:    svcErr.Message,
			ErrorLog:   errorLog,
		}
	case tenantservice.ErrorCodeConflict:
		return &HTTPError{
			StatusCode: http.StatusConflict,
			Message:    svcErr.Message,
			ErrorLog:   errorLog,
		}
	case tenantservice.ErrorCodeSeatLimit:
		return &HTTPError{
			StatusCode: http.StatusConflict,
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
