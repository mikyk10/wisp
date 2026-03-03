/**
 * Resolve the API base URL at runtime.
 *
 * Priority:
 *   1. window.__env__.API_BASE_URL — set by docker-entrypoint.sh at container start (nginx/production)
 *   2. import.meta.env.VITE_API_BASE_URL — injected by Vite dev server (local dev via compose)
 *   3. '' — mock mode
 */
function resolveApiBaseUrl(): string {
  return window.__env__?.API_BASE_URL ?? import.meta.env.VITE_API_BASE_URL ?? ''
}

export const API_BASE_URL: string = resolveApiBaseUrl()

/** Returns true when a backend API is configured. */
export const isApiMode = (): boolean => API_BASE_URL.trim() !== ''

/** Build an absolute URL from a relative API path. Throws if API mode is not active. */
export function buildApiUrl(path: string): string {
  if (!isApiMode()) throw new Error('API base URL is not set')
  const cleanPath = path.startsWith('/') ? path.slice(1) : path
  return `${API_BASE_URL.replace(/\/$/, '')}/${cleanPath}`
}

/** Canonical API path segments. */
export const API_PATHS = {
  catalogs: (): string => 'api/catalogs',
  catalogImages: (catalogKey: string, tags: string[] = []): string => {
    const base = `api/catalog/${catalogKey}/images`
    return tags.length > 0 ? `${base}?tags=${tags.join(',')}` : base
  },
  catalogTags: (catalogKey: string): string => `api/catalog/${catalogKey}/tags`,
}

/**
 * Returns the URL for an individual photo image.
 * In API mode: fetched from the backend.
 * In mock mode: a deterministic picsum.photos placeholder.
 */
export function buildImageUrl(catalogKey: string, id: number): string {
  if (isApiMode()) {
    return `${API_BASE_URL.replace(/\/$/, '')}/api/catalog/${catalogKey}/image/${id}.jpg`
  }
  return `https://picsum.photos/240/240?random=${id}`
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
