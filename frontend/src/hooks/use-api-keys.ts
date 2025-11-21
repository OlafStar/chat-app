'use client'

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { api } from "~/lib/api"
import type {
  CreateTenantAPIKeyResponse,
  DeleteTenantAPIKeyRequest,
  TenantAPIKey,
  TenantAPIKeyListResponse,
} from "~/types/tenant"

const API_KEYS_QUERY_KEY = ["tenant", "api-keys"]

const sortKeysByCreatedAt = (keys: TenantAPIKey[]) =>
  [...keys].sort((a, b) => {
    const first = Date.parse(a.createdAt || "")
    const second = Date.parse(b.createdAt || "")
    return (isNaN(second) ? 0 : second) - (isNaN(first) ? 0 : first)
  })

export function useTenantApiKeys() {
  return useQuery({
    queryKey: API_KEYS_QUERY_KEY,
    queryFn: async () => {
      const response = await api.get<TenantAPIKeyListResponse>("/tenant/api-keys")
      return sortKeysByCreatedAt(response.data.keys ?? [])
    },
  })
}

export function useCreateTenantApiKeyMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      const response = await api.post<CreateTenantAPIKeyResponse>("/tenant/api-keys")
      return response.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: API_KEYS_QUERY_KEY })
    },
  })
}

export function useDeleteTenantApiKeyMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (keyId: DeleteTenantAPIKeyRequest["keyId"]) => {
      await api.delete("/tenant/api-keys", { data: { keyId } })
      return keyId
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: API_KEYS_QUERY_KEY })
    },
  })
}

export { API_KEYS_QUERY_KEY }
