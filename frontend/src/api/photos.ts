import { apiClient } from './client'
import { API_PATHS } from '@/config'
import type { TagUsage } from '@/types'

export const photosApi = {
  /**
   * The tags carried by photos in one catalogue, most used first.
   *
   * Read when the picker opens rather than with the page, because a catalogue
   * can hold hundreds of tags and most sessions never filter at all.
   */
  async getCatalogTags(catalogKey: string): Promise<TagUsage[]> {
    const { data } = await apiClient.get<{ tags: TagUsage[] }>(API_PATHS.catalogTags(catalogKey))
    return data.tags ?? []
  },

  /**
   * Toggle the visibility of a batch of photos.
   * POST /api/catalog/selected/_toggle-visibility  { ids: number[] }
   */
  async toggleVisibility(ids: number[]): Promise<void> {
    await apiClient.post(API_PATHS.catalogToggleVisibility(), { ids })
  },
}
