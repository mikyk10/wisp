import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePhotosStore } from '../photos'
import type { Photo } from '@/types'

// Mock the API module so tests don't make real HTTP calls.
vi.mock('@/api/photos', () => ({
  photosApi: {
    toggleVisibility: vi.fn().mockResolvedValue(undefined),
  },
}))

// Mock config so isApiMode() returns false (mock / offline mode).
vi.mock('@/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/config')>()
  return {
    ...actual,
    API_BASE_URL: '',
    isApiMode: () => false,
    buildImageUrl: (_catalogKey: string, id: number) =>
      `https://picsum.photos/240/240?random=${id}`,
    getDataSourceUrl: (p: string) => `/mock-data/${p}`,
  }
})

function makePhoto(id: number, overrides: Partial<Photo> = {}): Photo {
  return {
    id,
    url: `https://picsum.photos/240/240?random=${id}`,
    enabled: true,
    timestamp: `2024-0${(id % 12) + 1}-01T00:00:00+00:00`,
    ...overrides,
  }
}

describe('usePhotosStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  describe('initial state', () => {
    it('starts with empty items and loading=false', () => {
      const store = usePhotosStore()
      expect(store.items).toHaveLength(0)
      expect(store.loading).toBe(false)
      expect(store.streamCompleted).toBe(false)
      expect(store.error).toBeNull()
    })

    it('totalPhotos is 0 initially', () => {
      const store = usePhotosStore()
      expect(store.totalPhotos).toBe(0)
    })

    it('timelineEntries is empty initially', () => {
      const store = usePhotosStore()
      expect(store.timelineEntries).toHaveLength(0)
    })
  })

  describe('resetPhotos', () => {
    it('clears all state', () => {
      const store = usePhotosStore()
      store.items.push(makePhoto(1))
      store.streamCompleted = true
      store.requestTimelineScroll({ key: '2024-01', startIndex: 0 })

      store.resetPhotos()

      expect(store.items).toHaveLength(0)
      expect(store.streamCompleted).toBe(false)
      expect(store.totalPhotos).toBe(0)
      expect(store.activeTimelineKey).toBe('')
      expect(store.scrollRequest).toBeNull()
    })
  })

  describe('timeline sync (grid ⇔ timeline scrollbar)', () => {
    // items 0–3: 2024-06, items 4–7: 2024-05
    function seedTwoMonths() {
      const store = usePhotosStore()
      for (let i = 0; i < 4; i++)
        store.items.push(makePhoto(i, { timestamp: '2024-06-15T00:00:00Z' }))
      for (let i = 4; i < 8; i++)
        store.items.push(makePhoto(i, { timestamp: '2024-05-15T00:00:00Z' }))
      return store
    }

    it('requestTimelineScroll sets the active key and a scroll request', () => {
      const store = usePhotosStore()
      store.requestTimelineScroll({ key: '2024-05', startIndex: 4 })

      expect(store.activeTimelineKey).toBe('2024-05')
      expect(store.scrollRequest).toEqual({ index: 4, key: '2024-05' })
    })

    it('reportViewport updates the active key from the first visible photo', () => {
      const store = seedTwoMonths()
      store.reportViewport(5)
      expect(store.activeTimelineKey).toBe('2024-05')
    })

    it('the first report after a click is consumed and does not snap the highlight back', () => {
      const store = seedTwoMonths()
      store.requestTimelineScroll({ key: '2024-05', startIndex: 4 })

      // The programmatic scroll landed short of the target (row snapping /
      // tail-of-list): the reported index still belongs to June.
      store.reportViewport(2)

      expect(store.activeTimelineKey).toBe('2024-05')
      expect(store.scrollRequest).toBeNull()
    })

    it('reports after the pending request is consumed win (genuine user scroll)', () => {
      const store = seedTwoMonths()
      store.requestTimelineScroll({ key: '2024-05', startIndex: 4 })

      store.reportViewport(4) // programmatic scroll arrival — consumed
      store.reportViewport(0) // user scrolled back to the top

      expect(store.activeTimelineKey).toBe('2024-06')
      expect(store.scrollRequest).toBeNull()
    })

    it('reportViewport ignores out-of-range indices', () => {
      const store = seedTwoMonths()
      store.activeTimelineKey = '2024-06'
      store.reportViewport(99)
      expect(store.activeTimelineKey).toBe('2024-06')
    })
  })

  describe('togglePhotoStatus', () => {
    it('individually toggles enabled flag on matched photos (mock mode)', async () => {
      const store = usePhotosStore()
      store.items.push(makePhoto(1, { enabled: true }))
      store.items.push(makePhoto(2, { enabled: true }))
      store.items.push(makePhoto(3, { enabled: false }))

      await store.togglePhotoStatus([1, 3])

      expect(store.items.find((p) => p.id === 1)?.enabled).toBe(false) // true → false
      expect(store.items.find((p) => p.id === 2)?.enabled).toBe(true)  // untouched
      expect(store.items.find((p) => p.id === 3)?.enabled).toBe(true)  // false → true
    })

    it('does not crash when ids list is empty', async () => {
      const store = usePhotosStore()
      store.items.push(makePhoto(1))
      await expect(store.togglePhotoStatus([])).resolves.toBeUndefined()
    })

    it('reaches the API only when one is configured', async () => {
      // isApiMode() is false here, so the request is never made. That gate is
      // also why API_PATHS entries may safely omit a leading slash: the path
      // is only ever resolved against a non-empty axios baseURL, where a
      // leading slash makes no difference to the resulting URL.
      const { photosApi } = await import('@/api/photos')
      const store = usePhotosStore()
      store.items.push(makePhoto(1))

      await store.togglePhotoStatus([1])

      expect(photosApi.toggleVisibility).not.toHaveBeenCalled()
    })
  })

  describe('totalPhotos getter', () => {
    it('reflects the length of items', () => {
      const store = usePhotosStore()
      store.items.push(makePhoto(1), makePhoto(2))
      expect(store.totalPhotos).toBe(2)
    })
  })

  describe('timelineEntries getter', () => {
    it('returns entries sorted newest-first', () => {
      const store = usePhotosStore()
      // Manually populate timeline to test the getter without streaming
      store.timeline['2023-01'] = { year: 2023, month: 1, startIndex: 10, count: 5 }
      store.timeline['2024-06'] = { year: 2024, month: 6, startIndex: 0, count: 3 }
      store.timeline['2022-12'] = { year: 2022, month: 12, startIndex: 15, count: 2 }

      const entries = store.timelineEntries
      expect(entries[0].key).toBe('2024-06')
      expect(entries[1].key).toBe('2023-01')
      expect(entries[2].key).toBe('2022-12')
    })

    it('formats the label correctly', () => {
      const store = usePhotosStore()
      store.timeline['2024-03'] = { year: 2024, month: 3, startIndex: 0, count: 1 }
      expect(store.timelineEntries[0].label).toBe('2024/03')
    })
  })
})
