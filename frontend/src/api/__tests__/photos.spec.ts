import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/client', () => ({
  apiClient: {
    post: vi.fn().mockResolvedValue({}),
  },
}))

describe('photosApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('toggleVisibility', () => {
    it('sends a POST request with the correct ids', async () => {
      const { apiClient } = await import('@/api/client')
      const { photosApi } = await import('../photos')

      await photosApi.toggleVisibility([1, 2, 3])

      expect(apiClient.post).toHaveBeenCalledOnce()
      expect(apiClient.post).toHaveBeenCalledWith('/catalog/selected/_toggle-visibility', {
        ids: [1, 2, 3],
      })
    })

    it('resolves to undefined (void) on success', async () => {
      const { photosApi } = await import('../photos')
      await expect(photosApi.toggleVisibility([42])).resolves.toBeUndefined()
    })
  })
})
