import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useCatalogsStore } from '../catalogs'
import { usePhotosStore } from '../photos'

// ---------- module mocks ----------

vi.mock('@/api/catalogs', () => ({
  catalogsApi: {
    fetchAll: vi.fn().mockResolvedValue(['album-a', 'album-b']),
  },
}))

vi.mock('@/api/photos', () => ({
  photosApi: { toggleVisibility: vi.fn().mockResolvedValue(undefined) },
}))

// isApiMode is mocked as vi.fn() so individual tests can override its return value.
vi.mock('@/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/config')>()
  return {
    ...actual,
    API_BASE_URL: '',
    isApiMode: vi.fn().mockReturnValue(false),
    buildImageUrl: (_: string, id: number) => `https://picsum.photos/240/240?random=${id}`,
    getDataSourceUrl: (p: string) => `/mock-data/${p}`,
  }
})

// ---------- helpers ----------

function makeStream(...chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder()
  return new ReadableStream({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk))
      controller.close()
    },
  })
}

let fetchMock: ReturnType<typeof vi.fn>

function stubFetch(body = '') {
  fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, body: makeStream(body) })
  vi.stubGlobal('fetch', fetchMock)
}

// ---------- tests ----------

describe('useCatalogsStore', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
    stubFetch() // default: empty NDJSON stream
    // Reset isApiMode to mock mode before each test
    const { isApiMode } = await import('@/config')
    vi.mocked(isApiMode).mockReturnValue(false)
    // Reset fetchAll mock to default success
    const { catalogsApi } = await import('@/api/catalogs')
    vi.mocked(catalogsApi.fetchAll).mockResolvedValue(['album-a', 'album-b'])
  })

  // ── initial state ────────────────────────────────────────────────────────

  describe('initial state', () => {
    it('starts with empty catalogs and no current catalog', () => {
      const store = useCatalogsStore()
      expect(store.catalogs).toHaveLength(0)
      expect(store.currentCatalog).toBe('')
      expect(store.error).toBeNull()
    })
  })

  // ── initCatalogs — mock mode ─────────────────────────────────────────────

  describe('initCatalogs — mock mode', () => {
    it('sets catalogs to ["default"] and currentCatalog to "default"', async () => {
      const store = useCatalogsStore()
      await store.initCatalogs()
      expect(store.catalogs).toEqual(['default'])
      expect(store.currentCatalog).toBe('default')
      expect(store.error).toBeNull()
    })

    it('marks the photos stream as completed after loading', async () => {
      const store = useCatalogsStore()
      await store.initCatalogs()
      const photosStore = usePhotosStore()
      expect(photosStore.streamCompleted).toBe(true)
    })

    it('does not call catalogsApi.fetchAll in mock mode', async () => {
      const { catalogsApi } = await import('@/api/catalogs')
      const store = useCatalogsStore()
      await store.initCatalogs()
      expect(catalogsApi.fetchAll).not.toHaveBeenCalled()
    })
  })

  // ── initCatalogs — API mode ──────────────────────────────────────────────

  describe('initCatalogs — API mode', () => {
    beforeEach(async () => {
      const { isApiMode } = await import('@/config')
      vi.mocked(isApiMode).mockReturnValue(true)
    })

    it('fetches catalog list and sets the first catalog as current', async () => {
      const store = useCatalogsStore()
      await store.initCatalogs()
      expect(store.catalogs).toEqual(['album-a', 'album-b'])
      expect(store.currentCatalog).toBe('album-a')
      expect(store.error).toBeNull()
    })

    it('does not load photos when the catalog list is empty', async () => {
      const { catalogsApi } = await import('@/api/catalogs')
      vi.mocked(catalogsApi.fetchAll).mockResolvedValueOnce([])

      const store = useCatalogsStore()
      await store.initCatalogs()

      expect(store.catalogs).toHaveLength(0)
      expect(store.currentCatalog).toBe('')
      // fetch for the NDJSON stream must not have been triggered
      expect(fetchMock).not.toHaveBeenCalled()
    })

    it('sets error state when the API call fails', async () => {
      const { catalogsApi } = await import('@/api/catalogs')
      vi.mocked(catalogsApi.fetchAll).mockRejectedValueOnce(new Error('network failure'))

      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useCatalogsStore()
      await store.initCatalogs()
      consoleSpy.mockRestore()

      expect(store.error).toBe('network failure')
      expect(store.catalogs).toHaveLength(0)
    })
  })

  // ── setCurrentCatalog ────────────────────────────────────────────────────

  describe('setCurrentCatalog', () => {
    it('updates currentCatalog and reloads the photos stream', async () => {
      const store = useCatalogsStore()
      await store.initCatalogs() // loads 'default'

      store.setCurrentCatalog('new-album')
      await flushPromises()

      expect(store.currentCatalog).toBe('new-album')
      // Photos stream must have completed for the new catalog
      const photosStore = usePhotosStore()
      expect(photosStore.streamCompleted).toBe(true)
    })
  })
})
