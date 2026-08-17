import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { usePhotosStore } from './photos'

export const useSelectionStore = defineStore('selection', () => {
  // ── State ────────────────────────────────────────────────────────────────
  const selectedIds = ref<Set<number>>(new Set())
  /** Anchor for Shift+click range selection: the last individually toggled photo. */
  const lastSelectedId = ref<number | null>(null)
  const updating = ref(false)
  const error = ref<string | null>(null)

  // ── Getters ──────────────────────────────────────────────────────────────
  const isSelectionMode = computed(() => selectedIds.value.size > 0)
  const selectedCount = computed(() => selectedIds.value.size)

  function isPhotoSelected(photoId: number): boolean {
    return selectedIds.value.has(photoId)
  }

  // ── Actions ──────────────────────────────────────────────────────────────
  function togglePhotoSelection(photoId: number) {
    if (selectedIds.value.has(photoId)) {
      selectedIds.value.delete(photoId)
    } else {
      selectedIds.value.add(photoId)
      lastSelectedId.value = photoId
    }
  }

  /**
   * Shift+click: select every photo between the anchor (last toggled photo)
   * and `photoId`, in display order. Falls back to a plain toggle when there
   * is no usable anchor. The anchor is kept so repeated Shift+clicks extend
   * from the same origin.
   */
  function selectRangeTo(photoId: number) {
    const photos = usePhotosStore().items
    const anchorId = lastSelectedId.value
    const anchorIndex = anchorId === null ? -1 : photos.findIndex((p) => p.id === anchorId)
    const targetIndex = photos.findIndex((p) => p.id === photoId)

    if (anchorIndex === -1 || targetIndex === -1) {
      togglePhotoSelection(photoId)
      return
    }

    const [lo, hi] =
      anchorIndex < targetIndex ? [anchorIndex, targetIndex] : [targetIndex, anchorIndex]
    for (let i = lo; i <= hi; i++) {
      selectedIds.value.add(photos[i].id)
    }
  }

  function clearSelection() {
    selectedIds.value.clear()
    lastSelectedId.value = null
    error.value = null
  }

  async function toggleSelectedPhotosStatus() {
    if (selectedIds.value.size === 0) return

    updating.value = true
    error.value = null

    try {
      const photosStore = usePhotosStore()
      await photosStore.togglePhotoStatus([...selectedIds.value])
      clearSelection()
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to update status'
      console.error('Error updating status for selected photos:', err)
    } finally {
      updating.value = false
    }
  }

  return {
    // state
    selectedIds,
    lastSelectedId,
    updating,
    error,
    // getters
    isSelectionMode,
    selectedCount,
    isPhotoSelected,
    // actions
    togglePhotoSelection,
    selectRangeTo,
    clearSelection,
    toggleSelectedPhotosStatus,
  }
})
