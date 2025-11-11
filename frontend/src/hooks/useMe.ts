import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { AuthMeResponse } from "~/types/auth";

export function useMe() {
  return useQuery({queryFn: () => api.get<AuthMeResponse>('/auth/me'), queryKey: ['me']})
}