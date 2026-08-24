/**
 * Resolve the API base URL at runtime.
 *
 * Priority:
 *   1. window.__env__.API_BASE_URL — set by docker-entrypoint.sh at container start (nginx/production)
 *   2. import.meta.env.VITE_API_BASE_URL — injected by Vite dev server (local dev via compose)
 *   3. '' — mock mode
 */
import { MOCK_IMAGE_COUNT } from '@/constants'

function resolveApiBaseUrl(): string {
  return window.__env__?.API_BASE_URL ?? import.meta.env.VITE_API_BASE_URL ?? ''
}

export const API_BASE_URL: string = resolveApiBaseUrl()

/** Returns true when a backend API is configured. */
export const isApiMode = (): boolean => API_BASE_URL.trim() !== ''

/**
 * Canonical API path segments.
 *
 * Written without a leading slash on purpose. These are handed either to
 * `apiClient`, which joins them onto its configured `baseURL`, or to
 * `getDataSourceUrl`, which re-roots them under /mock-data — and both treat
 * the value as a path relative to a base, never as one anchored at the site
 * root.
 */
export const API_PATHS = {
  catalogs: (): string => 'api/catalogs',
  catalogImages: (catalogKey: string, tags: string[] = []): string => {
    const path = `api/catalog/${catalogKey}/images`
    // Comma-separated, matching the server's own reading of the parameter. An
    // empty selection sends no parameter at all rather than `?tags=`, so that
    // "I cleared my filters" cannot be mistaken for a filter.
    return tags.length > 0 ? `${path}?tags=${encodeURIComponent(tags.join(','))}` : path
  },
  catalogTags: (catalogKey: string): string => `api/catalog/${catalogKey}/tags`,
  catalogToggleVisibility: (): string => 'api/catalog/selected/_toggle-visibility',
  devices: (): string => 'api/devices',
  deviceDeliveries: (displayKey: string): string => `api/device/${displayKey}/deliveries`,
}

/**
 * Returns the URL for an individual photo image.
 * In API mode: fetched from the backend.
 * In mock mode: a deterministic local placeholder (works offline).
 */
export function buildImageUrl(catalogKey: string, id: number): string {
  if (isApiMode()) {
    return `${API_BASE_URL.replace(/\/$/, '')}/api/catalog/${catalogKey}/image/${id}.jpg`
  }
  return `/mock-data/images/photo-${id % MOCK_IMAGE_COUNT}.svg`
}

/**
 * Returns the URL for a streaming data resource (NDJSON).
 * In API mode: remote API endpoint.
 * In mock mode: local file under /public/mock-data/.
 */
export function getDataSourceUrl(resourcePath: string): string {
  const cleanPath = resourcePath.startsWith('/') ? resourcePath.slice(1) : resourcePath
  if (isApiMode()) {
    return `${API_BASE_URL.replace(/\/$/, '')}/${cleanPath}`
  }
  return `/mock-data/${cleanPath}`
}
