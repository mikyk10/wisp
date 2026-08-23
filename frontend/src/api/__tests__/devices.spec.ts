import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/client', () => ({
  apiClient: {
    get: vi.fn(),
  },
}))

describe('devicesApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('fetchAll', () => {
    it('requests /api/devices and maps the payload to camelCase', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({
        data: {
          recording_enabled: true,
          recent_window: 20,
          devices: [
            {
              key: 'a1b2c3d4e5f6',
              name: 'living-room',
              model: 'ws7in3e',
              width: 800,
              height: 480,
              orientation: 'landscape',
              catalog_keys: ['photos'],
              sleep_duration_seconds: 300,
              wake_schedule: ['*/30 7-16 * * *'],
              last_delivered_at: '2026-08-19T12:30:00Z',
              recent_delivery_count: 20,
              recent_error_count: 3,
            },
          ],
        },
      })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchAll()

      expect(apiClient.get).toHaveBeenCalledWith('api/devices')
      expect(result.recentWindow).toBe(20)
      expect(result.recordingEnabled).toBe(true)
      expect(result.devices).toEqual([
        {
          key: 'a1b2c3d4e5f6',
          name: 'living-room',
          model: 'ws7in3e',
          width: 800,
          height: 480,
          orientation: 'landscape',
          catalogKeys: ['photos'],
          sleepDurationSeconds: 300,
          wakeSchedule: ['*/30 7-16 * * *'],
          lastDeliveredAt: '2026-08-19T12:30:00Z',
          recentDeliveryCount: 20,
          recentErrorCount: 3,
        },
      ])
    })

    it('keeps last_delivered_at: null as null (never delivered)', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({
        data: { devices: [{ key: 'dev', name: 'desk-test', last_delivered_at: null }] },
      })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchAll()

      expect(result.devices[0].lastDeliveredAt).toBeNull()
    })

    it('reports recording being switched off', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({
        data: { recording_enabled: false, recent_window: 20, devices: [] },
      })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchAll()

      expect(result.recordingEnabled).toBe(false)
    })

    it('assumes recording is on when the server does not report the field', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({ data: { devices: [] } })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchAll()

      // An older server that omits the flag must not be announced as "off".
      expect(result.recordingEnabled).toBe(true)
    })

    it('tolerates missing fields', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({ data: { devices: [{ key: 'bare' }] } })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchAll()

      expect(result.recentWindow).toBe(0)
      expect(result.devices[0]).toMatchObject({
        key: 'bare',
        name: 'bare', // falls back to the key
        catalogKeys: [],
        wakeSchedule: [],
        lastDeliveredAt: null,
      })
    })

    it('returns an empty list when the devices field is missing', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({ data: {} })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchAll()

      expect(result.devices).toEqual([])
    })

    it('propagates errors thrown by the API client', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockRejectedValue(new Error('network error'))

      const { devicesApi } = await import('../devices')
      await expect(devicesApi.fetchAll()).rejects.toThrow('network error')
    })
  })

  describe('fetchDeliveries', () => {
    it('requests the singular /api/device/{key}/deliveries path', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({ data: { deliveries: [] } })

      const { devicesApi } = await import('../devices')
      await devicesApi.fetchDeliveries('a1b2c3d4e5f6')

      expect(apiClient.get).toHaveBeenCalledWith('api/device/a1b2c3d4e5f6/deliveries')
    })

    it('maps delivery records to camelCase and preserves wire order', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({
        data: {
          device_key: 'a1b2c3d4e5f6',
          deliveries: [
            {
              delivered_at: '2026-08-19T12:30:00Z',
              kind: 'photo',
              reason: null,
              image_id: 4821,
              catalog_key: 'photos',
              source: '/mnt/photos/IMG_0421.jpg',
              requested_sleep_seconds: 300,
              image_available: true,
            },
            {
              delivered_at: '2026-08-19T12:20:00Z',
              kind: 'error',
              reason: 'file_missing',
              image_id: null,
              catalog_key: 'photos',
              source: null,
              requested_sleep_seconds: 300,
              image_available: false,
            },
          ],
        },
      })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchDeliveries('a1b2c3d4e5f6')

      expect(result).toEqual([
        {
          deliveredAt: '2026-08-19T12:30:00Z',
          kind: 'photo',
          reason: null,
          imageId: 4821,
          catalogKey: 'photos',
          source: '/mnt/photos/IMG_0421.jpg',
          requestedSleepSeconds: 300,
          imageAvailable: true,
        },
        {
          deliveredAt: '2026-08-19T12:20:00Z',
          kind: 'error',
          reason: 'file_missing',
          imageId: null,
          catalogKey: 'photos',
          source: null,
          requestedSleepSeconds: 300,
          imageAvailable: false,
        },
      ])
    })

    it('defaults image_available to false when the field is absent', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({
        data: { deliveries: [{ delivered_at: '2026-08-19T12:30:00Z', kind: 'photo', image_id: 1 }] },
      })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchDeliveries('dev')

      // Absent means "unknown", and an unknown image must not be rendered as
      // an <img>: a deleted photo answers 404 with a decodable error card.
      expect(result[0].imageAvailable).toBe(false)
    })

    it('defaults a missing reason to null', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({
        data: { deliveries: [{ delivered_at: '2026-08-19T12:30:00Z', kind: 'photo' }] },
      })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchDeliveries('dev')

      expect(result[0].reason).toBeNull()
    })

    it('passes an unrecognised reason code through untouched', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({
        data: {
          deliveries: [
            { delivered_at: '2026-08-19T12:30:00Z', kind: 'error', reason: 'sunspots' },
          ],
        },
      })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchDeliveries('dev')

      expect(result[0].reason).toBe('sunspots')
    })

    it('keeps a null catalog_key as null', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({
        data: {
          deliveries: [
            { delivered_at: '2026-08-19T12:30:00Z', kind: 'error', catalog_key: null },
          ],
        },
      })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchDeliveries('dev')

      expect(result[0].catalogKey).toBeNull()
    })

    it('treats an empty catalog_key as no catalogue at all', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({
        data: {
          deliveries: [
            { delivered_at: '2026-08-19T12:30:00Z', kind: 'colorbar', catalog_key: '' },
          ],
        },
      })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchDeliveries('dev')

      // Guards the older wire shape, where catalog_key was a plain string and
      // "none" arrived as "" rather than null.
      expect(result[0].catalogKey).toBeNull()
    })

    it('passes an unrecognised kind through untouched', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({
        data: { deliveries: [{ delivered_at: '2026-08-19T12:30:00Z', kind: 'hologram' }] },
      })

      const { devicesApi } = await import('../devices')
      const result = await devicesApi.fetchDeliveries('dev')

      expect(result[0].kind).toBe('hologram')
    })

    it('returns an empty list when the deliveries field is missing', async () => {
      const { apiClient } = await import('@/api/client')
      vi.mocked(apiClient.get).mockResolvedValue({ data: { device_key: 'dev' } })

      const { devicesApi } = await import('../devices')
      await expect(devicesApi.fetchDeliveries('dev')).resolves.toEqual([])
    })
  })
})
