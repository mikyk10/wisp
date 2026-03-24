import { apiClient } from './client'

export const photosApi = {
  /**
   * Toggle the visibility of a batch of photos.
   * POST /catalog/selected/_toggle-visibility  { ids: number[] }
   */
  async toggleVisibility(ids: number[]): Promise<void> {
    await apiClient.post('/api/catalog/selected/_toggle-visibility', { ids })
  },

  async getTags(id: number): Promise<string[]> {
    const res = await apiClient.get<{ tags: string[] }>(`/api/images/${id}/tags`)
    return res.data.tags ?? []
  },

  async getCatalogTags(catalogKey: string): Promise<string[]> {
    const res = await apiClient.get<{ tags: string[] }>(`/api/catalog/${catalogKey}/tags`)
    return res.data.tags ?? []
  },
}
