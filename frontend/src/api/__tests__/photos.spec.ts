import { describe, it, expect, vi, beforeEach } from 'vitest'
import axios from 'axios'
import { API_PATHS } from '@/config'

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
      expect(apiClient.post).toHaveBeenCalledWith('api/catalog/selected/_toggle-visibility', {
        ids: [1, 2, 3],
      })
    })

    it('resolves against the API base to the same URL the literal path did', async () => {
      // The path moved into API_PATHS, whose entries carry no leading slash.
      // Resolution is what matters, not the string: this runs the value axios
      // is actually handed through axios's own URL building and pins the
      // absolute result, so dropping or re-adding a slash cannot change the
      // endpoint without failing here.
      const client = axios.create({ baseURL: 'http://backend.test' })

      const resolved = client.getUri({ url: API_PATHS.catalogToggleVisibility() })

      expect(resolved).toBe('http://backend.test/api/catalog/selected/_toggle-visibility')
      // Identical to what the hardcoded leading-slash literal produced before.
      expect(resolved).toBe(client.getUri({ url: '/api/catalog/selected/_toggle-visibility' }))
    })

    it('resolves to undefined (void) on success', async () => {
      const { photosApi } = await import('../photos')
      await expect(photosApi.toggleVisibility([42])).resolves.toBeUndefined()
    })
  })
})
