import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { API_PATHS, isApiMode, buildImageUrl } from '@/config'
import { photosApi } from '@/api/photos'
import type { Photo, PhotoRecord, TimelineEntry } from '@/types'

/** Month bucket key ("YYYY-MM") for a photo, or null for invalid/zero-value dates. */
function monthKeyOf(timestamp: string): string | null {
  const date = new Date(timestamp)
  const year = date.getFullYear()
  // Photos without EXIF have taken_at set to Go's zero value (0001-01-01); skip them
  if (isNaN(year) || year < 1900) return null
  const month = date.getMonth() + 1
  return `${year}-${month.toString().padStart(2, '0')}`
}

export const usePhotosStore = defineStore('photos', () => {
  // ── State ────────────────────────────────────────────────────────────────
  const items = ref<Photo[]>([])
  const loading = ref(false)
  const timeline = ref<Record<string, Omit<TimelineEntry, 'key' | 'label'>>>({})
  const streamCompleted = ref(false)
  const error = ref<string | null>(null)
  let abortController: AbortController | null = null
  // Generation token: incremented on every load/reset. In-flight streams from
  // older generations must not write any state once a newer generation starts.
  let generation = 0

  // ── Timeline sync state (grid ⇔ timeline scrollbar) ──────────────────────
  /** Month key of the entry highlighted in the timeline sidebar. */
  const activeTimelineKey = ref('')
  /**
   * Pending programmatic scroll requested by a timeline click.
   * PhotoGrid watches this and scrolls the virtual grid to `index`.
   * Stays set until the viewport report confirms arrival, so scroll events
   * fired by the programmatic scroll cannot snap the highlight back.
   */
  const scrollRequest = ref<{ index: number; key: string } | null>(null)

  // ── Getters ──────────────────────────────────────────────────────────────
  const totalPhotos = computed(() => items.value.length)

  const timelineEntries = computed((): TimelineEntry[] => {
    return Object.entries(timeline.value)
      .sort(([a], [b]) => b.localeCompare(a)) // descending order (newest first)
      .map(([key, data]) => ({
        key,
        label: `${data.year}/${String(data.month).padStart(2, '0')}`,
        ...data,
      }))
  })

  // ── Private helpers ──────────────────────────────────────────────────────
  function _updateTimeline(newPhotos: Photo[], startOffset: number) {
    newPhotos.forEach((photo, i) => {
      const key = monthKeyOf(photo.timestamp)
      if (!key) return
      const date = new Date(photo.timestamp)
      if (!timeline.value[key]) {
        timeline.value[key] = {
          year: date.getFullYear(),
          month: date.getMonth() + 1,
          startIndex: startOffset + i,
          count: 0,
        }
      }
      timeline.value[key].count++
    })
  }

  function _buildTimeline() {
    const rebuilt: Record<string, Omit<TimelineEntry, 'key' | 'label'>> = {}
    items.value.forEach((photo, index) => {
      const key = monthKeyOf(photo.timestamp)
      if (!key) return
      const date = new Date(photo.timestamp)
      if (!rebuilt[key]) {
        rebuilt[key] = {
          year: date.getFullYear(),
          month: date.getMonth() + 1,
          startIndex: index,
          count: 0,
        }
      }
      rebuilt[key].count++
    })
    timeline.value = rebuilt
  }

  function _resetState() {
    items.value = []
    loading.value = false
    timeline.value = {}
    streamCompleted.value = false
    error.value = null
    activeTimelineKey.value = ''
    scrollRequest.value = null
  }

  // ── Actions ──────────────────────────────────────────────────────────────
  /**
   * Timeline entry clicked: highlight it and ask the grid to scroll there.
   */
  function requestTimelineScroll(entry: Pick<TimelineEntry, 'key' | 'startIndex'>) {
    activeTimelineKey.value = entry.key
    scrollRequest.value = { index: entry.startIndex, key: entry.key }
  }

  /**
   * Called by the grid (debounced) with the first visible item index after a
   * scroll. The first report while a scroll request is pending is caused by
   * the programmatic scroll itself: it can land short of the target (row
   * snapping, or a tail-of-list month that cannot scroll all the way up), so
   * it must not overwrite the clicked month. Consuming exactly one report is
   * the state-based replacement for the old 300ms suppression timer.
   */
  function reportViewport(firstVisibleIndex: number) {
    if (scrollRequest.value) {
      scrollRequest.value = null
      return
    }
    const photo = items.value[firstVisibleIndex]
    if (!photo) return
    const key = monthKeyOf(photo.timestamp)
    if (!key) return
    activeTimelineKey.value = key
  }

  async function loadPhotosStream(catalogKey: string) {
    // Invalidate any in-flight stream, then reset and start a new one.
    const gen = ++generation
    abortController?.abort()
    abortController = new AbortController()
    const { signal } = abortController

    _resetState()
    loading.value = true

    try {
      const { NDJSONStreamReader } = await import('@/services/ndjsonStream')
      const reader = new NDJSONStreamReader<PhotoRecord>()

      let batch: Photo[] = []
      const batchSize = 50

      const resource = isApiMode() ? API_PATHS.catalogImages(catalogKey) : 'photos.ndjson'

      for await (const rec of reader.readStream(resource, signal)) {
        if (gen !== generation) return
        const url = buildImageUrl(catalogKey, rec.id)
        batch.push({ ...rec, url })

        if (batch.length >= batchSize) {
          const startOffset = items.value.length
          items.value.push(...batch)
          _updateTimeline(batch, startOffset)
          batch = []
          // Yield control back to the UI thread
          await new Promise((resolve) => setTimeout(resolve, 0))
          if (gen !== generation || signal.aborted) return
        }
      }

      if (gen !== generation) return

      if (batch.length > 0) {
        const startOffset = items.value.length
        items.value.push(...batch)
        _updateTimeline(batch, startOffset)
      }

      // Rebuild from all items after stream completion to ensure consistency
      _buildTimeline()
      streamCompleted.value = true
    } catch (err) {
      if (gen !== generation || signal.aborted) return
      error.value = err instanceof Error ? err.message : 'Failed to load photos'
      console.error('Photo load error:', err)
    } finally {
      if (gen === generation) {
        loading.value = false
      }
    }
  }

  function resetPhotos() {
    generation++
    abortController?.abort()
    abortController = null
    _resetState()
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
    activeTimelineKey,
    scrollRequest,
    // getters
    totalPhotos,
    timelineEntries,
    // actions
    loadPhotosStream,
    resetPhotos,
    togglePhotoStatus,
    requestTimelineScroll,
    reportViewport,
  }
})
