import { defineStore } from 'pinia'
import { ref } from 'vue'
import { isApiMode } from '@/config'
import { catalogsApi } from '@/api/catalogs'
import { usePhotosStore } from './photos'

export const useCatalogsStore = defineStore('catalogs', () => {
  // ── State ────────────────────────────────────────────────────────────────
  const catalogs = ref<string[]>([])
  const currentCatalog = ref('')
  const error = ref<string | null>(null)

  // ── Private helpers ──────────────────────────────────────────────────────
  async function _loadCatalog(catalogKey: string) {
    const photosStore = usePhotosStore()
    photosStore.resetPhotos()
    await photosStore.loadPhotosStream(catalogKey)
  }

  // ── Actions ──────────────────────────────────────────────────────────────
  async function initCatalogs() {
    error.value = null

    if (isApiMode()) {
      try {
        const fetched = await catalogsApi.fetchAll()
        catalogs.value = fetched
        if (fetched.length > 0) {
          currentCatalog.value = fetched[0]
          await _loadCatalog(fetched[0])
        }
      } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to fetch catalogs'
        console.error('Catalog fetch error:', err)
      }
    } else {
      catalogs.value = ['default']
      currentCatalog.value = 'default'
      await _loadCatalog('default')
    }
  }

  async function setCurrentCatalog(catalog: string) {
    currentCatalog.value = catalog
    usePhotosStore().filterTags = []
    await _loadCatalog(catalog)
  }

  return {
    // state
    catalogs,
    currentCatalog,
    error,
    // actions
    initCatalogs,
    setCurrentCatalog,
  }
})
