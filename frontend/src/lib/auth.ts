import type { AuthResponse } from "~/types/auth"

const ACCESS_TOKEN_KEY = "pingy:access-token"
const REFRESH_TOKEN_KEY = "pingy:refresh-token"

const getCookieOptions = () => {
  const options = ["path=/", "sameSite=lax"]
  if (typeof window !== "undefined" && window.location.protocol === "https:") {
    options.push("secure")
  }
  return options.join("; ")
}

const setCookie = (name: string, value: string) => {
  if (typeof document === "undefined") {
    return
  }

  document.cookie = `${name}=${encodeURIComponent(value)}; ${getCookieOptions()}`
}

const clearCookie = (name: string) => {
  if (typeof document === "undefined") {
    return
  }

  document.cookie = `${name}=; Max-Age=0; ${getCookieOptions()}`
}

export const getAccessToken = () => {
  if (typeof window === "undefined") {
    return null
  }

  return window.localStorage.getItem(ACCESS_TOKEN_KEY)
}

export const persistAuthTokens = (response: AuthResponse) => {
  if (typeof window === "undefined") {
    return
  }

  window.localStorage.setItem(ACCESS_TOKEN_KEY, response.accessToken)
  setCookie(ACCESS_TOKEN_KEY, response.accessToken)
  if (response.refreshToken) {
    window.localStorage.setItem(REFRESH_TOKEN_KEY, response.refreshToken)
    setCookie(REFRESH_TOKEN_KEY, response.refreshToken)
  }
}

export const clearAuthTokens = () => {
  if (typeof window === "undefined") {
    return
  }

  window.localStorage.removeItem(ACCESS_TOKEN_KEY)
  window.localStorage.removeItem(REFRESH_TOKEN_KEY)
  clearCookie(ACCESS_TOKEN_KEY)
  clearCookie(REFRESH_TOKEN_KEY)
}
