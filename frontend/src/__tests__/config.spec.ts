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

// ---------- buildApiUrl ----------

describe('buildApiUrl', () => {
  beforeEach(() => vi.resetModules())
  afterEach(() => setWindowEnv(undefined))

  it('builds a URL from base + path', async () => {
    setWindowEnv('http://api.example.com')
    const { buildApiUrl } = await import('../config')
    expect(buildApiUrl('catalogs')).toBe('http://api.example.com/catalogs')
  })

  it('strips trailing slash from base and leading slash from path', async () => {
    setWindowEnv('http://api.example.com/')
    const { buildApiUrl } = await import('../config')
    expect(buildApiUrl('/catalog/foo/images')).toBe(
      'http://api.example.com/catalog/foo/images'
    )
  })

  it('throws when API_BASE_URL is empty', async () => {
    setWindowEnv('')
    const { buildApiUrl } = await import('../config')
    expect(() => buildApiUrl('catalogs')).toThrow('API base URL is not set')
  })
})

// ---------- buildImageUrl ----------

describe('buildImageUrl', () => {
  beforeEach(() => vi.resetModules())
  afterEach(() => setWindowEnv(undefined))

  it('returns an API URL in API mode', async () => {
    setWindowEnv('http://api.example.com')
    const { buildImageUrl } = await import('../config')
    expect(buildImageUrl('mycat', 42)).toBe(
      'http://api.example.com/catalog/mycat/image/42.jpg'
    )
  })

  it('returns a picsum placeholder in mock mode', async () => {
    setWindowEnv('')
    const { buildImageUrl } = await import('../config')
    const url = buildImageUrl('mycat', 7)
    expect(url).toContain('picsum.photos')
    expect(url).toContain('7')
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
