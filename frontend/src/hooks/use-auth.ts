'use client'

import { useMutation } from "@tanstack/react-query"

import { api } from "~/lib/api"
import { persistAuthTokens } from "~/lib/auth"
import type {
  AuthResponse,
  LoginRequest,
  RegisterRequest,
} from "~/types/auth"

export function useLoginMutation() {
  return useMutation({
    mutationFn: (payload: LoginRequest) =>
      api.post<AuthResponse>("/auth/login", payload),
    onSuccess(response) {
      persistAuthTokens(response.data)
    },
  })
}

export function useSignupMutation() {
  return useMutation({
    mutationFn: (payload: RegisterRequest) =>
      api.post<AuthResponse>("/auth/register", payload),
    onSuccess(response) {
      persistAuthTokens(response.data)
    },
  })
}
