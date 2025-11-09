package tenant

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"chat-app-backend/internal/model"
)

const (
	DefaultWidgetBubbleText = "Chat with us"
	DefaultWidgetHeaderText = "Need a hand?"
	DefaultWidgetThemeColor = "#7F56D9"
)

var hexColorPattern = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

type WidgetSettings struct {
	BubbleText string
	HeaderText string
	ThemeColor string
}

type WidgetSettingsInput struct {
	BubbleText string
	HeaderText string
	ThemeColor string
}

func defaultWidgetSettings() WidgetSettings {
	return WidgetSettings{
		BubbleText: DefaultWidgetBubbleText,
		HeaderText: DefaultWidgetHeaderText,
		ThemeColor: DefaultWidgetThemeColor,
	}
}

func WidgetSettingsFromTenant(tenant model.TenantItem) WidgetSettings {
	return widgetSettingsFromMap(tenant.Settings)
}

func widgetSettingsFromMap(settings map[string]interface{}) WidgetSettings {
	result := defaultWidgetSettings()
	if settings == nil {
		return result
	}

	widgetRaw, ok := settings["widget"]
	if !ok {
		return result
	}

	widgetMap, ok := widgetRaw.(map[string]interface{})
	if !ok {
		return result
	}

	if val, ok := widgetMap["bubbleText"].(string); ok && strings.TrimSpace(val) != "" {
		result.BubbleText = val
	}
	if val, ok := widgetMap["headerText"].(string); ok && strings.TrimSpace(val) != "" {
		result.HeaderText = val
	}
	if val, ok := widgetMap["themeColor"].(string); ok && strings.TrimSpace(val) != "" {
		result.ThemeColor = val
	}

	return result
}

func (w WidgetSettings) toMap() map[string]interface{} {
	return map[string]interface{}{
		"bubbleText": w.BubbleText,
		"headerText": w.HeaderText,
		"themeColor": w.ThemeColor,
	}
}

func normalizeWidgetSettings(input WidgetSettingsInput) (WidgetSettings, error) {
	settings := defaultWidgetSettings()

	if trimmed := strings.TrimSpace(input.BubbleText); trimmed != "" {
		settings.BubbleText = trimmed
	}

	if trimmed := strings.TrimSpace(input.HeaderText); trimmed != "" {
		settings.HeaderText = trimmed
	}

	if trimmed := strings.TrimSpace(input.ThemeColor); trimmed != "" {
		if !hexColorPattern.MatchString(trimmed) {
			return WidgetSettings{}, newError(ErrorCodeValidation, "themeColor must be a valid hex color (e.g. #7F56D9)", nil)
		}
		settings.ThemeColor = strings.ToUpper(trimmed)
	}

	return settings, nil
}

func cloneSettings(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return make(map[string]interface{})
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (s *Service) GetWidgetSettings(ctx context.Context, identity Identity, tenantID string) (WidgetSettings, error) {
	_, tenant, err := s.ensureOwnerAccess(ctx, identity, tenantID)
	if err != nil {
		return WidgetSettings{}, err
	}
	return widgetSettingsFromMap(tenant.Settings), nil
}

func (s *Service) UpdateWidgetSettings(ctx context.Context, identity Identity, tenantID string, params WidgetSettingsInput) (WidgetSettings, error) {
	_, tenant, err := s.ensureOwnerAccess(ctx, identity, tenantID)
	if err != nil {
		return WidgetSettings{}, err
	}

	normalized, err := normalizeWidgetSettings(params)
	if err != nil {
		return WidgetSettings{}, err
	}

	nextSettings := cloneSettings(tenant.Settings)
	nextSettings["widget"] = normalized.toMap()

	if _, err := s.repo.UpdateTenantSettings(ctx, tenant.TenantID, nextSettings); err != nil {
		if errors.Is(err, ErrNotFound) {
			return WidgetSettings{}, newError(ErrorCodeNotFound, "tenant not found", err)
		}
		return WidgetSettings{}, newError(ErrorCodeInternal, "failed to update widget settings", err)
	}

	return normalized, nil
}

func (s *Service) PublicWidgetSettings(ctx context.Context, tenantKey string) (WidgetSettings, error) {
	tenantKey = strings.TrimSpace(tenantKey)
	if tenantKey == "" {
		return WidgetSettings{}, newError(ErrorCodeValidation, "tenantKey is required", nil)
	}

	tenant, err := s.repo.GetTenantByAPIKey(ctx, tenantKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return WidgetSettings{}, newError(ErrorCodeNotFound, "tenant not found", err)
		}
		return WidgetSettings{}, newError(ErrorCodeInternal, "failed to load tenant", err)
	}

	return widgetSettingsFromMap(tenant.Settings), nil
}
