import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { API_PATHS, isApiMode, buildImageUrl } from '@/config'
import { photosApi } from '@/api/photos'
import type { Photo, PhotoRecord, TimelineEntry } from '@/types'

export const usePhotosStore = defineStore('photos', () => {
  // ── State ────────────────────────────────────────────────────────────────
  const items = ref<Photo[]>([])
  const loading = ref(false)
  const timeline = ref<Record<string, Omit<TimelineEntry, 'key' | 'label'>>>({})
  const streamCompleted = ref(false)
  const error = ref<string | null>(null)

  // ── Getters ──────────────────────────────────────────────────────────────
  const totalPhotos = computed(() => items.value.length)

  const timelineEntries = computed((): TimelineEntry[] => {
    return Object.entries(timeline.value)
      .sort(([a], [b]) => b.localeCompare(a)) // descending order (newest first)
      .map(([key, data]) => ({
        key,
        label: `${data.year}/${data.month}`,
        ...data,
      }))
  })

  // ── Private helpers ──────────────────────────────────────────────────────
  function _updateTimeline(newPhotos: Photo[], startOffset: number) {
    newPhotos.forEach((photo, i) => {
      const date = new Date(photo.timestamp)
      const year = date.getFullYear()
      // Photos without EXIF have taken_at set to Go's zero value (0001-01-01); skip them
      if (isNaN(year) || year < 1900) return
      const month = date.getMonth() + 1
      const key = `${year}-${month.toString().padStart(2, '0')}`
      if (!timeline.value[key]) {
        timeline.value[key] = { year, month, startIndex: startOffset + i, count: 0 }
      }
      timeline.value[key].count++
    })
  }

  function _buildTimeline() {
    const rebuilt: Record<string, Omit<TimelineEntry, 'key' | 'label'>> = {}
    items.value.forEach((photo, index) => {
      const date = new Date(photo.timestamp)
      const year = date.getFullYear()
      if (isNaN(year) || year < 1900) return
      const month = date.getMonth() + 1
      const key = `${year}-${month.toString().padStart(2, '0')}`
      if (!rebuilt[key]) {
        rebuilt[key] = { year, month, startIndex: index, count: 0 }
      }
      rebuilt[key].count++
    })
    timeline.value = rebuilt
  }

  // ── Actions ──────────────────────────────────────────────────────────────
  async function loadPhotosStream(catalogKey: string) {
    loading.value = true
    error.value = null

    try {
      const { NDJSONStreamReader } = await import('@/services/ndjsonStream')
      const reader = new NDJSONStreamReader<PhotoRecord>()

      let batch: Photo[] = []
      const batchSize = 50

      const resource = isApiMode() ? API_PATHS.catalogImages(catalogKey) : 'photos.ndjson'

      for await (const rec of reader.readStream(resource)) {
        const url = buildImageUrl(catalogKey, rec.id)
        batch.push({ ...rec, url })

        if (batch.length >= batchSize) {
          const startOffset = items.value.length
          items.value.push(...batch)
          _updateTimeline(batch, startOffset)
          batch = []
          // Yield control back to the UI thread
          await new Promise((resolve) => setTimeout(resolve, 0))
        }
      }

      if (batch.length > 0) {
        const startOffset = items.value.length
        items.value.push(...batch)
        _updateTimeline(batch, startOffset)
      }

      // Rebuild from all items after stream completion to ensure consistency
      _buildTimeline()
      streamCompleted.value = true
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load photos'
      console.error('Photo load error:', err)
    } finally {
      loading.value = false
    }
  }

  function resetPhotos() {
    items.value = []
    loading.value = false
    timeline.value = {}
    streamCompleted.value = false
    error.value = null
  }

  async function togglePhotoStatus(ids: number[]): Promise<void> {
    if (isApiMode()) {
      await photosApi.toggleVisibility(ids)
    }
    // Mirror the backend's per-photo toggle (deleted_at flip).
    const idSet = new Set(ids)
    items.value = items.value.map((photo) =>
      idSet.has(photo.id) ? { ...photo, enabled: !photo.enabled } : photo,
    )
  }

  return {
    // state
    items,
    loading,
    timeline,
    streamCompleted,
    error,
    // getters
    totalPhotos,
    timelineEntries,
    // actions
    loadPhotosStream,
    resetPhotos,
    togglePhotoStatus,
  }
})
