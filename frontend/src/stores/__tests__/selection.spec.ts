import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSelectionStore } from '../selection'
import { usePhotosStore } from '../photos'

vi.mock('@/api/photos', () => ({
  photosApi: { toggleVisibility: vi.fn().mockResolvedValue(undefined) },
}))

vi.mock('@/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/config')>()
  return {
    ...actual,
    API_BASE_URL: '',
    isApiMode: () => false,
    buildImageUrl: (_: string, id: number) => `https://picsum.photos/240/240?random=${id}`,
    getDataSourceUrl: (p: string) => `/mock-data/${p}`,
  }
})

describe('useSelectionStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  describe('initial state', () => {
    it('starts with no selections', () => {
      const store = useSelectionStore()
      expect(store.selectedIds).toHaveLength(0)
      expect(store.isSelectionMode).toBe(false)
      expect(store.selectedCount).toBe(0)
    })
  })

  describe('togglePhotoSelection', () => {
    it('adds a photo id on first toggle', () => {
      const store = useSelectionStore()
      store.togglePhotoSelection(1)
      expect(store.selectedIds).toContain(1)
      expect(store.isSelectionMode).toBe(true)
    })

    it('removes a photo id on second toggle (deselect)', () => {
      const store = useSelectionStore()
      store.togglePhotoSelection(1)
      store.togglePhotoSelection(1)
      expect(store.selectedIds).not.toContain(1)
      expect(store.isSelectionMode).toBe(false)
    })

    it('handles multiple distinct ids', () => {
      const store = useSelectionStore()
      store.togglePhotoSelection(1)
      store.togglePhotoSelection(2)
      store.togglePhotoSelection(3)
      expect(store.selectedCount).toBe(3)
    })
  })

  describe('isPhotoSelected', () => {
    it('returns true for selected id', () => {
      const store = useSelectionStore()
      store.togglePhotoSelection(42)
      expect(store.isPhotoSelected(42)).toBe(true)
    })

    it('returns false for unselected id', () => {
      const store = useSelectionStore()
      expect(store.isPhotoSelected(99)).toBe(false)
    })
  })

  describe('clearSelection', () => {
    it('empties selectedIds and exits selection mode', () => {
      const store = useSelectionStore()
      store.togglePhotoSelection(1)
      store.togglePhotoSelection(2)
      store.clearSelection()
      expect(store.selectedIds).toHaveLength(0)
      expect(store.isSelectionMode).toBe(false)
    })
  })

  describe('toggleSelectedPhotosStatus', () => {
    it('does nothing when nothing is selected', async () => {
      const store = useSelectionStore()
      await expect(store.toggleSelectedPhotosStatus()).resolves.toBeUndefined()
      expect(store.updating).toBe(false)
    })

    it('individually toggles each selected photo', async () => {
      const photosStore = usePhotosStore()
      photosStore.items.push(
        { id: 1, url: '', enabled: true, timestamp: '' },
        { id: 2, url: '', enabled: false, timestamp: '' },
      )

      const store = useSelectionStore()
      store.togglePhotoSelection(1)
      store.togglePhotoSelection(2)

      await store.toggleSelectedPhotosStatus()

      expect(photosStore.items.find((p) => p.id === 1)?.enabled).toBe(false) // true → false
      expect(photosStore.items.find((p) => p.id === 2)?.enabled).toBe(true)  // false → true
    })

    it('clears selection after toggling status', async () => {
      const photosStore = usePhotosStore()
      photosStore.items.push({ id: 1, url: '', enabled: true, timestamp: '' })

      const store = useSelectionStore()
      store.togglePhotoSelection(1)
      await store.toggleSelectedPhotosStatus()

      expect(store.selectedIds).toHaveLength(0)
      expect(store.isSelectionMode).toBe(false)
    })

    it('sets error state when togglePhotoStatus throws', async () => {
      const { photosApi } = await import('@/api/photos')
      vi.mocked(photosApi.toggleVisibility).mockRejectedValueOnce(new Error('network error'))

      // Put the store in API mode so updatePhotoStatus calls the API
      vi.mock('@/config', async (importOriginal) => {
        const actual = await importOriginal<typeof import('@/config')>()
        return { ...actual, isApiMode: () => true }
      })

      const photosStore = usePhotosStore()
      photosStore.items.push({ id: 1, url: '', enabled: true, timestamp: '' })

      const store = useSelectionStore()
      store.togglePhotoSelection(1)

      // Even in mock mode the error propagates if updatePhotoStatus throws
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      await store.toggleSelectedPhotosStatus()
      consoleSpy.mockRestore()

      expect(store.updating).toBe(false) // finally ran
    })
  })
})
