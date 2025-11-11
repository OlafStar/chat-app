import axios, { AxiosHeaders } from "axios"

import { getAccessToken } from "~/lib/auth"

const API_BASE_PATH = "http://localhost:8080/api/client/v1"

const api = axios.create({
  baseURL: API_BASE_PATH,
  headers: {
    "Content-Type": "application/json",
  },
})

api.interceptors.request.use((config) => {
  const accessToken = getAccessToken()
  if (accessToken) {
    const headers = new AxiosHeaders(config.headers)
    headers.setAuthorization(`Bearer ${accessToken}`)
    config.headers = headers
  }

  return config
})

export { api }
