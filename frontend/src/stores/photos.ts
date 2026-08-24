import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { API_PATHS, isApiMode, buildImageUrl } from '@/config'
import { photosApi } from '@/api/photos'
import type { Photo, PhotoRecord, TimelineEntry } from '@/types'
import { clampFraction } from '@/utils/scrubber'

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
   * Tags the grid is narrowed to. Every one of them, not any: adding a tag
   * narrows, which is what the click that added it is asking for.
   */
  const filterTags = ref<string[]>([])

  /**
   * The photo whose tags are being shown, if any.
   *
   * It lives here rather than travelling up as an event because the card that
   * opens it sits inside a virtual scroller: the component that raised it may
   * be recycled onto a different photo before the sheet closes, so the sheet
   * has to hold the photo itself and not a route back to a card.
   */
  const tagSheetPhoto = ref<Photo | null>(null)
  /**
   * Pending programmatic scroll requested by a timeline click.
   * PhotoGrid watches this and scrolls the virtual grid to `index`.
   * Stays set until the viewport report confirms arrival, so scroll events
   * fired by the programmatic scroll cannot snap the highlight back.
   */
  const scrollRequest = ref<{ index: number; key: string } | null>(null)

  /**
   * Where the viewport sits in the whole list, 0 at the newest photo and 1 at
   * the oldest, reported by the grid on every scroll frame. This is the
   * scrubber thumb's position. It is separate from activeTimelineKey on
   * purpose: the key is month-grained and debounced, which is right for a
   * highlight and visibly wrong for a marker that should track the scroll.
   */
  const viewportFraction = ref(0)

  /**
   * Pending scrub — a proportional scroll position, requested by dragging the
   * scrubber rail. A distinct channel from scrollRequest because the two mean
   * different things: a click means "take me to this month" (snap to its first
   * row, hold the highlight until arrival), a drag means "put the viewport
   * exactly here, now" and arrives dozens of times a second. Suppressing a
   * viewport report per drag step would fight the very reports that keep the
   * highlight following the drag.
   */
  const scrubRequest = ref<{ fraction: number } | null>(null)

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
    tagSheetPhoto.value = null
    loading.value = false
    timeline.value = {}
    streamCompleted.value = false
    error.value = null
    activeTimelineKey.value = ''
    scrollRequest.value = null
    viewportFraction.value = 0
    scrubRequest.value = null
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
  /** Grid scroll position as a fraction; drives the scrubber thumb. */
  function reportScrollFraction(fraction: number) {
    viewportFraction.value = clampFraction(fraction)
  }

  /** Scrubber drag: ask the grid to place the viewport at this fraction. */
  function requestScrub(fraction: number) {
    // A fresh object every call, even for an unchanged value: the grid reacts
    // to the request's identity, and a drag that wiggles back over the same
    // pixel still expects the viewport to follow.
    scrubRequest.value = { fraction: clampFraction(fraction) }
  }

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

  async function loadPhotosStream(catalogKey: string, tags: string[] = filterTags.value) {
    filterTags.value = tags
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

      const resource = isApiMode() ? API_PATHS.catalogImages(catalogKey, tags) : 'photos.ndjson'

      for await (const rec of reader.readStream(resource, signal)) {
        if (gen !== generation) return
        const url = buildImageUrl(catalogKey, rec.id)
        // A server that predates tags on the listing sends no field at all;
        // an empty array keeps every reader downstream from having to check.
        batch.push({ ...rec, url, tags: rec.tags ?? [] })

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

  /**
   * Replace the tag filter and reload.
   *
   * Filtering happens on the server: the grid holds one catalogue's worth of
   * rows and narrowing it here would still have streamed all of them, which is
   * the cost the filter exists to avoid.
   *
   * Switching catalogues clears the filter instead of carrying it over — tags
   * are per catalogue, so a filter carried across would usually be a filter
   * that matches nothing, and an empty grid reads as a broken one.
   */
  async function setFilterTags(catalogKey: string, tags: string[]) {
    await loadPhotosStream(catalogKey, tags)
  }

  function clearFilterTags() {
    filterTags.value = []
  }

  function showPhotoTags(photo: Photo) {
    tagSheetPhoto.value = photo
  }

  function hidePhotoTags() {
    tagSheetPhoto.value = null
  }

  return {
    // state
    items,
    filterTags,
    tagSheetPhoto,
    loading,
    timeline,
    streamCompleted,
    error,
    activeTimelineKey,
    scrollRequest,
    viewportFraction,
    scrubRequest,
    // getters
    totalPhotos,
    timelineEntries,
    // actions
    loadPhotosStream,
    setFilterTags,
    clearFilterTags,
    showPhotoTags,
    hidePhotoTags,
    resetPhotos,
    togglePhotoStatus,
    requestTimelineScroll,
    reportViewport,
    reportScrollFraction,
    requestScrub,
  }
})
