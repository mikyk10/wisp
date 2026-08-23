import { apiClient } from './client'
import { API_PATHS } from '@/config'

export const photosApi = {
  /**
   * Toggle the visibility of a batch of photos.
   * POST /api/catalog/selected/_toggle-visibility  { ids: number[] }
   */
  async toggleVisibility(ids: number[]): Promise<void> {
    await apiClient.post(API_PATHS.catalogToggleVisibility(), { ids })
  },
}
