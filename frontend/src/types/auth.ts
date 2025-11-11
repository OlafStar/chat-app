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

export interface TenantResponse {
  tenantId: string
  name: string
  plan: string
  seats: number
  createdAt: string
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
  tenants?: TenantMembership[]
}

export interface AuthMeResponse {
  user: UserResponse
  tenant: TenantResponse & {
    settings: {
      widget: {
        bubbleText:string;
        headerText: string;
        themeColor: string
      }
    }
  }
}