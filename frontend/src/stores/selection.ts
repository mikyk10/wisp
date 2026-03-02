import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { usePhotosStore } from './photos'

export const useSelectionStore = defineStore('selection', () => {
  // ── State ────────────────────────────────────────────────────────────────
  const selectedIds = ref<number[]>([])
  const updating = ref(false)
  const error = ref<string | null>(null)

  // ── Getters ──────────────────────────────────────────────────────────────
  const isSelectionMode = computed(() => selectedIds.value.length > 0)
  const selectedCount = computed(() => selectedIds.value.length)

  function isPhotoSelected(photoId: number): boolean {
    return selectedIds.value.includes(photoId)
  }

  // ── Actions ──────────────────────────────────────────────────────────────
  function togglePhotoSelection(photoId: number) {
    const index = selectedIds.value.indexOf(photoId)
    if (index === -1) {
      selectedIds.value.push(photoId)
    } else {
      selectedIds.value.splice(index, 1)
    }
  }

  function clearSelection() {
    selectedIds.value = []
    error.value = null
  }

  async function toggleSelectedPhotosStatus() {
    if (selectedIds.value.length === 0) return

    updating.value = true
    error.value = null

    try {
      const photosStore = usePhotosStore()
      await photosStore.togglePhotoStatus(selectedIds.value)
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
    updating,
    error,
    // getters
    isSelectionMode,
    selectedCount,
    isPhotoSelected,
    // actions
    togglePhotoSelection,
    clearSelection,
    toggleSelectedPhotosStatus,
  }
})
