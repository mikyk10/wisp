import { describe, it, expect, beforeEach, afterEach } from 'vitest'

// We re-import the module under test after each test so that the
// module-level `API_BASE_URL` constant is re-evaluated with the
// current window.__env__ / import.meta.env values.
//
// Vitest's module cache is cleared with vi.resetModules() in beforeEach.
import { vi } from 'vitest'

// ---------- helpers ----------

function setWindowEnv(url: string | undefined) {
  if (url === undefined) {
    delete (window as Window & { __env__?: Record<string, string> }).__env__
  } else {
    ;(window as Window & { __env__?: Record<string, string> }).__env__ = {
      API_BASE_URL: url,
    }
  }
}

// ---------- buildImageUrl ----------

describe('buildImageUrl', () => {
  beforeEach(() => vi.resetModules())
  afterEach(() => setWindowEnv(undefined))

  it('returns an API URL in API mode', async () => {
    setWindowEnv('http://api.example.com')
    const { buildImageUrl } = await import('../config')
    expect(buildImageUrl('mycat', 42)).toBe(
      'http://api.example.com/api/catalog/mycat/image/42.jpg'
    )
  })

  it('returns a deterministic local placeholder in mock mode', async () => {
    setWindowEnv('')
    const { buildImageUrl } = await import('../config')
    expect(buildImageUrl('mycat', 7)).toBe('/mock-data/images/photo-7.svg')
    // ids map onto the fixed placeholder set
    expect(buildImageUrl('mycat', 19)).toBe('/mock-data/images/photo-7.svg')
  })
})

// ---------- getDataSourceUrl ----------

describe('getDataSourceUrl', () => {
  beforeEach(() => vi.resetModules())
  afterEach(() => setWindowEnv(undefined))

  it('returns the API path in API mode', async () => {
    setWindowEnv('http://api.example.com')
    const { getDataSourceUrl } = await import('../config')
    expect(getDataSourceUrl('photos.ndjson')).toBe(
      'http://api.example.com/photos.ndjson'
    )
  })

  it('returns the local mock-data path in mock mode', async () => {
    setWindowEnv('')
    const { getDataSourceUrl } = await import('../config')
    expect(getDataSourceUrl('photos.ndjson')).toBe('/mock-data/photos.ndjson')
  })

  it('handles a leading slash in the resource path', async () => {
    setWindowEnv('')
    const { getDataSourceUrl } = await import('../config')
    expect(getDataSourceUrl('/photos.ndjson')).toBe('/mock-data/photos.ndjson')
  })
})

// ---------- isApiMode ----------

describe('isApiMode', () => {
  beforeEach(() => vi.resetModules())
  afterEach(() => setWindowEnv(undefined))

  it('returns true when API_BASE_URL is set', async () => {
    setWindowEnv('http://api.example.com')
    const { isApiMode } = await import('../config')
    expect(isApiMode()).toBe(true)
  })

  it('returns false when API_BASE_URL is empty', async () => {
    setWindowEnv('')
    const { isApiMode } = await import('../config')
    expect(isApiMode()).toBe(false)
  })

  it('returns false when __env__ is absent', async () => {
    setWindowEnv(undefined)
    const { isApiMode } = await import('../config')
    expect(isApiMode()).toBe(false)
  })
})

describe('API_PATHS.catalogImages tag filter', () => {
  // Filtering happens on the server. The grid holds one catalogue's worth of
  // rows, so narrowing in the browser would still have streamed all of them —
  // which is the cost the filter exists to avoid.
  it('carries the tags as one comma-separated parameter', async () => {
    const { API_PATHS } = await import('../config')
    expect(API_PATHS.catalogImages('photos', ['sakura', 'night'])).toBe(
      'api/catalog/photos/images?tags=sakura%2Cnight'
    )
  })

  it('sends no parameter at all when nothing is filtered', async () => {
    // `?tags=` would reach the server as a filter it has to decide how to read;
    // sending nothing says "no filter" without relying on that reading.
    const { API_PATHS } = await import('../config')
    expect(API_PATHS.catalogImages('photos')).toBe('api/catalog/photos/images')
    expect(API_PATHS.catalogImages('photos', [])).toBe('api/catalog/photos/images')
  })

  it('escapes a tag that would otherwise break the query', async () => {
    const { API_PATHS } = await import('../config')
    expect(API_PATHS.catalogImages('photos', ['a&b'])).toBe('api/catalog/photos/images?tags=a%26b')
  })

  it('points at the catalogue tag list', async () => {
    const { API_PATHS } = await import('../config')
    expect(API_PATHS.catalogTags('photos')).toBe('api/catalog/photos/tags')
  })
})
