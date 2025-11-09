import type { AuthResponse, TenantMembership } from "../api/auth";

export type AuthSession = {
  accessToken: string;
  refreshToken?: string;
  user: AuthResponse["user"];
  tenant: AuthResponse["tenant"];
  tenants?: TenantMembership[];
};

const STORAGE_KEY = "chat-app.session";

function getStorage(): Storage | null {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage;
}

export function persistAuthSession(payload: AuthResponse) {
  const storage = getStorage();
  if (!storage) {
    return;
  }

  const session: AuthSession = {
    accessToken: payload.accessToken,
    refreshToken: payload.refreshToken,
    user: payload.user,
    tenant: payload.tenant,
    tenants: payload.tenants,
  };

  storage.setItem(STORAGE_KEY, JSON.stringify(session));
  storage.setItem("accessToken", payload.accessToken);
  if (payload.refreshToken) {
    storage.setItem("refreshToken", payload.refreshToken);
  }
}

export function loadAuthSession(): AuthSession | null {
  const storage = getStorage();
  if (!storage) {
    return null;
  }

  const raw = storage.getItem(STORAGE_KEY);
  if (!raw) {
    return null;
  }

  try {
    return JSON.parse(raw) as AuthSession;
  } catch {
    storage.removeItem(STORAGE_KEY);
    return null;
  }
}

export function clearAuthSession() {
  const storage = getStorage();
  if (!storage) {
    return;
  }
  storage.removeItem(STORAGE_KEY);
  storage.removeItem("accessToken");
  storage.removeItem("refreshToken");
}
