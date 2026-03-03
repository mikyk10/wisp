import { apiClient } from './client'

export const photosApi = {
  /**
   * Toggle the visibility of a batch of photos.
   * POST /catalog/selected/_toggle-visibility  { ids: number[] }
   */
  async toggleVisibility(ids: number[]): Promise<void> {
    await apiClient.post('/api/catalog/selected/_toggle-visibility', { ids })
  },

  /**
   * Fetch AI-assigned tags for a single photo.
   * GET /api/images/:id/tags → { tags: string[] }
   */
  async getTags(id: number): Promise<string[]> {
    const res = await apiClient.get<{ tags: string[] }>(`/api/images/${id}/tags`)
    return res.data.tags ?? []
  },

  /**
   * Fetch all tag names used in the given catalog.
   * GET /api/catalog/:catalogKey/tags → { tags: string[] }
   */
  async getCatalogTags(catalogKey: string): Promise<string[]> {
    const res = await apiClient.get<{ tags: string[] }>(`/api/catalog/${catalogKey}/tags`)
    return res.data.tags ?? []
  },
}
