package endpoints

import (
	"bytes"
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/middleware"
	"chat-app-backend/internal/dto"
	internaljwt "chat-app-backend/internal/jwt"
	"chat-app-backend/internal/model"
	"chat-app-backend/internal/queue"
	tenantservice "chat-app-backend/internal/service/tenant"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupWidgetHandler(t *testing.T, repo tenantservice.Repository) (http.Handler, func()) {
	t.Helper()

	internaljwt.RoleSecrets[internaljwt.RoleUser] = "test-secret"

	service := tenantservice.NewWithRepository(repo, tenantFixedTime)
	widgetEndpoints := NewWidgetEndpoints(service)

	queueManager := queue.NewRequestQueueManager(10, 1)
	server := api.NewAPIServer(":0", queueManager, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/tenant/widget", server.MakeHTTPHandleFunc(widgetEndpoints.TenantWidgetSettings, middleware.ValidateUserJWT))
	mux.HandleFunc("/api/widget", server.MakeHTTPHandleFunc(widgetEndpoints.PublicWidgetSettings))

	return mux, func() {
		queueManager.Shutdown()
	}
}

func TestWidgetTenantGetDefaults(t *testing.T) {
	repo := newTenantTestRepository()
	handler, cleanup := setupWidgetHandler(t, repo)
	t.Cleanup(cleanup)

	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Acme",
		Plan:     "starter",
		Seats:    1,
		Created:  tenantFixedTime().Format(time.RFC3339),
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    tenant.Created,
	}
	repo.CreateUser(context.Background(), owner)

	token, err := internaljwt.CreateToken(internaljwt.User{
		Id:           owner.UserID,
		TenantID:     owner.TenantID,
		Email:        owner.Email,
		PasswordHash: owner.PasswordHash,
	}, internaljwt.RoleUser, 0)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tenant/widget", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var resp dto.WidgetSettingsResultResponse
	if err := json.Unmarshal(res.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Widget.BubbleText != tenantservice.DefaultWidgetBubbleText {
		t.Fatalf("unexpected bubble text: %s", resp.Widget.BubbleText)
	}
}

func TestWidgetTenantUpdate(t *testing.T) {
	repo := newTenantTestRepository()
	handler, cleanup := setupWidgetHandler(t, repo)
	t.Cleanup(cleanup)

	now := tenantFixedTime().Format(time.RFC3339)
	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Acme",
		Plan:     "starter",
		Seats:    1,
		Created:  now,
	}
	repo.tenants[tenant.TenantID] = tenant

	owner := model.UserItem{
		PK:           model.TenantScopedPK(tenant.TenantID, "owner-1"),
		TenantID:     tenant.TenantID,
		UserID:       "owner-1",
		Email:        "owner@example.com",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
		PasswordHash: "hash",
		CreatedAt:    now,
	}
	repo.CreateUser(context.Background(), owner)

	token, err := internaljwt.CreateToken(internaljwt.User{
		Id:           owner.UserID,
		TenantID:     owner.TenantID,
		Email:        owner.Email,
		PasswordHash: owner.PasswordHash,
	}, internaljwt.RoleUser, 0)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	payload, _ := json.Marshal(dto.UpdateWidgetSettingsRequest{
		BubbleText: "Need help?",
		HeaderText: "Pingy Support",
		ThemeColor: "#123ABC",
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/tenant/widget", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var resp dto.WidgetSettingsResultResponse
	if err := json.Unmarshal(res.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Widget.BubbleText != "Need help?" || resp.Widget.HeaderText != "Pingy Support" || resp.Widget.ThemeColor != "#123ABC" {
		t.Fatalf("unexpected widget response: %+v", resp.Widget)
	}
}

func TestWidgetPublicSettings(t *testing.T) {
	repo := newTenantTestRepository()
	handler, cleanup := setupWidgetHandler(t, repo)
	t.Cleanup(cleanup)

	now := tenantFixedTime().Format(time.RFC3339)
	tenant := model.TenantItem{
		TenantID: "tenant-1",
		Name:     "Acme",
		Plan:     "starter",
		Seats:    1,
		Created:  now,
		Settings: map[string]interface{}{
			"widget": map[string]interface{}{
				"bubbleText": "Chat now",
				"headerText": "Acme team",
				"themeColor": "#ABCDEF",
			},
		},
	}
	repo.tenants[tenant.TenantID] = tenant
	repo.CreateTenantAPIKey(context.Background(), model.TenantAPIKeyItem{
		TenantID:  tenant.TenantID,
		KeyID:     "key-1",
		APIKey:    "pingy_test_key",
		CreatedAt: now,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/widget?tenantKey=pingy_test_key", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var resp dto.WidgetSettingsResultResponse
	if err := json.Unmarshal(res.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Widget.ThemeColor != "#ABCDEF" {
		t.Fatalf("expected custom color, got %+v", resp.Widget)
	}
}
