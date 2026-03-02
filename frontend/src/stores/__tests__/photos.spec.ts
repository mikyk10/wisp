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

      store.resetPhotos()

      expect(store.items).toHaveLength(0)
      expect(store.streamCompleted).toBe(false)
      expect(store.totalPhotos).toBe(0)
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
      expect(store.timelineEntries[0].label).toBe('2024/3')
    })
  })
})
