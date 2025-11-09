package endpoints

import (
	"chat-app-backend/internal/dto"
	tenantservice "chat-app-backend/internal/service/tenant"
	"encoding/json"
	"fmt"
	"net/http"
)

type WidgetEndpoints interface {
	TenantWidgetSettings(http.ResponseWriter, *http.Request) error
	PublicWidgetSettings(http.ResponseWriter, *http.Request) error
}

type widgetEndpoints struct {
	service *tenantservice.Service
}

func NewWidgetEndpoints(service *tenantservice.Service) WidgetEndpoints {
	return &widgetEndpoints{
		service: service,
	}
}

func (h *widgetEndpoints) TenantWidgetSettings(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodGet:   h.handleGetTenantWidgetSettings,
		http.MethodPatch: h.handleUpdateTenantWidgetSettings,
	})
}

func (h *widgetEndpoints) PublicWidgetSettings(w http.ResponseWriter, r *http.Request) error {
	return MethodHandler(w, r, map[string]func(http.ResponseWriter, *http.Request) error{
		http.MethodGet: h.handlePublicWidgetSettings,
	})
}

func (h *widgetEndpoints) handleGetTenantWidgetSettings(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return mapTenantServiceError(err)
	}

	settings, err := h.service.GetWidgetSettings(r.Context(), identity, identity.TenantID)
	if err != nil {
		return mapTenantServiceError(err)
	}

	return WriteJSON(w, http.StatusOK, dto.WidgetSettingsResultResponse{
		Widget: widgetSettingsResult(settings),
	})
}

func (h *widgetEndpoints) handleUpdateTenantWidgetSettings(w http.ResponseWriter, r *http.Request) error {
	identity, err := h.service.IdentityFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return mapTenantServiceError(err)
	}

	var req dto.UpdateWidgetSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request payload",
			ErrorLog:   fmt.Errorf("decode widget settings request: %w", err),
		}
	}

	settings, err := h.service.UpdateWidgetSettings(r.Context(), identity, identity.TenantID, tenantservice.WidgetSettingsInput{
		BubbleText: req.BubbleText,
		HeaderText: req.HeaderText,
		ThemeColor: req.ThemeColor,
	})
	if err != nil {
		return mapTenantServiceError(err)
	}

	return WriteJSON(w, http.StatusOK, dto.WidgetSettingsResultResponse{
		Widget: widgetSettingsResult(settings),
	})
}

func (h *widgetEndpoints) handlePublicWidgetSettings(w http.ResponseWriter, r *http.Request) error {
	tenantKey := r.URL.Query().Get("tenantKey")
	settings, err := h.service.PublicWidgetSettings(r.Context(), tenantKey)
	if err != nil {
		return mapTenantServiceError(err)
	}

	return WriteJSON(w, http.StatusOK, dto.WidgetSettingsResultResponse{
		Widget: widgetSettingsResult(settings),
	})
}
