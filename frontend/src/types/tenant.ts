export interface TenantAPIKey {
  keyId: string
  apiKey: string
  createdAt: string
}

export interface WidgetSettings {
  bubbleText: string
  headerText: string
  themeColor: string
}

export interface TenantSettings {
  widget: WidgetSettings
}

export interface TenantAPIKeyListResponse {
  keys: TenantAPIKey[]
}

export interface CreateTenantAPIKeyResponse {
  key: TenantAPIKey
}

export interface DeleteTenantAPIKeyRequest {
  keyId: string
}
