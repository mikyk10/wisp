import { apiClient } from './client'

export const photosApi = {
  /**
   * Toggle the visibility of a batch of photos.
   * POST /catalog/selected/_toggle-visibility  { ids: number[] }
   */
  async toggleVisibility(ids: number[]): Promise<void> {
    await apiClient.post('/api/catalog/selected/_toggle-visibility', { ids })
  },
}
