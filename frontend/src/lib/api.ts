import axios, {
  AxiosError,
  type AxiosRequestConfig,
} from "axios"

import {
  clearAuthTokens,
  getAccessToken,
  getRefreshToken,
  persistAuthTokens,
} from "~/lib/auth"
import type { RefreshTokenResponse } from "~/types/auth"

const API_BASE_PATH = "http://localhost:8080/api/client/v1"

type ApiRequestConfig = AxiosRequestConfig & {
  _retry?: boolean
}

const api = axios.create({
  baseURL: API_BASE_PATH,
  headers: {
    "Content-Type": "application/json",
  },
})

api.interceptors.request.use((config) => {
  const accessToken = getAccessToken()
  if (accessToken && config.headers) {
    config.headers.Authorization = `Bearer ${accessToken}`
  }

  return config
})

api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as ApiRequestConfig | undefined
    const hasRefreshUrl =
      originalRequest?.url?.includes("/auth/refresh") ?? false

    if (
      error.response?.status === 401 &&
      !hasRefreshUrl &&
      originalRequest &&
      !originalRequest._retry
    ) {
      const refreshToken = getRefreshToken()
      if (!refreshToken) {
        return Promise.reject(error)
      }

      originalRequest._retry = true
      try {
        const refreshResponse = await api.post<RefreshTokenResponse>("/auth/refresh", {
          refreshToken,
        })

        persistAuthTokens(refreshResponse.data)

        if (originalRequest.headers) {
          originalRequest.headers.Authorization = `Bearer ${refreshResponse.data.accessToken}`
        }

        return api(originalRequest)
      } catch (refreshError) {
        clearAuthTokens()
        return Promise.reject(refreshError)
      }
    }

    return Promise.reject(error)
  }
)

export { api }