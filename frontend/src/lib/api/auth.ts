import { apiRequest } from "./client";

export type TenantResponse = {
  tenantId: string;
  name: string;
  plan: string;
  seats: number;
  createdAt: string;
  remainingSeats?: number;
};

export type UserResponse = {
  userId: string;
  tenantId: string;
  email: string;
  name: string;
  role: string;
  status: string;
  createdAt: string;
};

export type TenantMembership = {
  userId: string;
  tenantId: string;
  name: string;
  plan: string;
  seats: number;
  role: string;
  status: string;
  isDefault: boolean;
};

export type AuthResponse = {
  accessToken: string;
  refreshToken?: string;
  user: UserResponse;
  tenant: TenantResponse;
  apiKeys?: Array<{
    keyId: string;
    apiKey: string;
    createdAt: string;
  }>;
  tenants?: TenantMembership[];
};

export type RegisterRequest = {
  tenantName: string;
  name: string;
  email: string;
  password: string;
};

export type LoginRequest = {
  tenantId?: string;
  email: string;
  password: string;
};

export async function registerTenant(payload: RegisterRequest) {
  return apiRequest<AuthResponse>("/auth/register", {
    method: "POST",
    body: payload,
  });
}

export async function login(payload: LoginRequest) {
  return apiRequest<AuthResponse>("/auth/login", {
    method: "POST",
    body: payload,
  });
}
