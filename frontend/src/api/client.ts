import axios from 'axios'
import { API_BASE_URL } from '@/config'

/**
 * Shared axios instance for all REST API calls.
 * Stream endpoints (NDJSON) use fetch() directly via NDJSONStreamReader.
 */
export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10_000,
  headers: { 'Content-Type': 'application/json' },
})

// Response interceptor — centralise error handling without duplicating try/catch.
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status
    if (status === 401 || status === 403) {
      // Future: redirect to login or surface an auth error
      console.warn('[apiClient] Auth error:', status)
    } else if (status >= 500) {
      console.error('[apiClient] Server error:', status, error.response?.data)
    }
    return Promise.reject(error)
  }
)
