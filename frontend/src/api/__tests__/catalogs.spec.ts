import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/client', () => ({
  apiClient: {
    get: vi.fn(),
  },
}))

describe('catalogsApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('fetchAll', () => {
    it('returns the catalogs array from the API response', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({ data: { catalogs: ['album-a', 'album-b'] } })

      const { catalogsApi } = await import('../catalogs')
      const result = await catalogsApi.fetchAll()

      expect(apiClient.get).toHaveBeenCalledWith('catalogs')
      expect(result).toEqual(['album-a', 'album-b'])
    })

    it('returns an empty array when the catalogs field is missing', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({ data: {} })

      const { catalogsApi } = await import('../catalogs')
      const result = await catalogsApi.fetchAll()

      expect(result).toEqual([])
    })

    it('propagates errors thrown by the API client', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockRejectedValue(new Error('network error'))

      const { catalogsApi } = await import('../catalogs')
      await expect(catalogsApi.fetchAll()).rejects.toThrow('network error')
    })
  })
})
