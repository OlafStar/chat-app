import axios from "axios"

const DEFAULT_ERROR = "Something went wrong"

export const getAPIErrorMessage = (
  error: unknown,
  fallbackMessage = DEFAULT_ERROR
) => {
  if (axios.isAxiosError(error)) {
    const serverMessage =
      (error.response?.data as Record<string, unknown> | undefined)
        ?.message as string | undefined

    return serverMessage ?? error.message ?? fallbackMessage
  }

  return fallbackMessage
}
