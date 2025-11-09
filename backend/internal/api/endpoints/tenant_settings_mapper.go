package endpoints

import (
	"chat-app-backend/internal/dto"
	"chat-app-backend/internal/model"
	tenantservice "chat-app-backend/internal/service/tenant"
)

func tenantSettingsResponse(tenant model.TenantItem) *dto.TenantSettingsResponse {
	widget := tenantservice.WidgetSettingsFromTenant(tenant)
	return &dto.TenantSettingsResponse{
		Widget: dto.WidgetSettingsResponse{
			BubbleText: widget.BubbleText,
			HeaderText: widget.HeaderText,
			ThemeColor: widget.ThemeColor,
		},
	}
}

func widgetSettingsResult(settings tenantservice.WidgetSettings) dto.WidgetSettingsResponse {
	return dto.WidgetSettingsResponse{
		BubbleText: settings.BubbleText,
		HeaderText: settings.HeaderText,
		ThemeColor: settings.ThemeColor,
	}
}
