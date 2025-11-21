import type { TenantAPIKey, TenantSettings } from "./tenant"

export interface RegisterRequest {
  tenantName: string
  name: string
  email: string
  password: string
}

export interface LoginRequest {
  tenantId?: string
  email: string
  password: string
}

export interface RefreshTokenRequest {
  refreshToken: string
}

export interface RefreshTokenResponse {
  accessToken: string
  refreshToken?: string
}

export interface TenantResponse {
  tenantId: string
  name: string
  plan: string
  seats: number
  createdAt: string
  remainingSeats?: number
  settings?: TenantSettings
}

export interface UserResponse {
  userId: string
  tenantId: string
  email: string
  name: string
  role: string
  status: string
  createdAt: string
}

export interface TenantMembership {
  userId: string
  tenantId: string
  name: string
  plan: string
  seats: number
  role: string
  status: string
  isDefault: boolean
}

export interface AuthResponse {
  accessToken: string
  refreshToken?: string
  user: UserResponse
  tenant: TenantResponse
  apiKeys?: TenantAPIKey[]
  tenants?: TenantMembership[]
}

export interface AuthMeResponse {
  user: UserResponse
  tenant: TenantResponse
}
