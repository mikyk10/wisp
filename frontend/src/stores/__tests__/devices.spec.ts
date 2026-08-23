import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useDevicesStore } from '../devices'

// ---------- module mocks ----------

vi.mock('@/api/devices', () => ({
  devicesApi: {
    fetchAll: vi.fn(),
    fetchDeliveries: vi.fn(),
  },
}))

// isApiMode is a vi.fn() so individual tests can flip between mock and API mode.
vi.mock('@/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/config')>()
  return {
    ...actual,
    API_BASE_URL: '',
    isApiMode: vi.fn().mockReturnValue(false),
    buildImageUrl: (_: string, id: number) => `/mock-data/images/photo-${id % 12}.svg`,
    getDataSourceUrl: (p: string) => `/mock-data/${p}`,
  }
})

let fetchMock: ReturnType<typeof vi.fn>

// ---------- tests ----------

describe('useDevicesStore', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.unstubAllGlobals()
    // Any network call at all in mock mode is a bug, so fetch is stubbed with
    // something that fails loudly rather than something that works.
    fetchMock = vi.fn(() => {
      throw new Error('mock mode must not hit the network')
    })
    vi.stubGlobal('fetch', fetchMock)

    const { isApiMode } = await import('@/config')
    vi.mocked(isApiMode).mockReturnValue(false)
  })

  describe('initial state', () => {
    it('starts empty', () => {
      const store = useDevicesStore()
      expect(store.devices).toHaveLength(0)
      expect(store.recentWindow).toBe(0)
      // Defaults to on: an unloaded drawer must not claim recording has stopped.
      expect(store.recordingEnabled).toBe(true)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })
  })

  // ── mock mode ────────────────────────────────────────────────────────────

  describe('mock mode', () => {
    it('yields the fixture without touching the network or the api layer', async () => {
      const { devicesApi } = await import('@/api/devices')
      const store = useDevicesStore()

      await store.loadDevices()
      await store.loadAllDeliveries()

      expect(store.devices).toHaveLength(3)
      expect(store.recentWindow).toBe(20)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(fetchMock).not.toHaveBeenCalled()
      expect(devicesApi.fetchAll).not.toHaveBeenCalled()
      expect(devicesApi.fetchDeliveries).not.toHaveBeenCalled()
    })

    it('includes exactly one display that has never delivered', async () => {
      const store = useDevicesStore()
      await store.loadDevices()

      const never = store.devices.filter((d) => d.lastDeliveredAt === null)
      expect(never).toHaveLength(1)
      expect(never[0].name).toBe('desk-test')
      expect(store.deliveriesFor(never[0].key)).toEqual([])
    })

    it('dates the fixture relative to now rather than to frozen literals', async () => {
      const before = Date.now()
      const store = useDevicesStore()
      await store.loadDevices()

      const delivered = store.devices.find((d) => d.lastDeliveredAt !== null)
      const stamp = Date.parse(delivered?.lastDeliveredAt ?? '')
      // Four minutes ago, measured from the call — not a date baked into source.
      expect(stamp).toBeGreaterThan(before - 5 * 60_000)
      expect(stamp).toBeLessThanOrEqual(Date.now())
    })

    it('includes a display whose newest deliveries are a run of five errors', async () => {
      const store = useDevicesStore()
      await store.loadDevices()
      await store.loadAllDeliveries()

      const failing = store.devices.find((d) => d.recentErrorCount > 0)
      expect(failing).toBeDefined()

      // Newest first on the wire: the run sits at the head of the array.
      const deliveries = store.deliveriesFor(failing?.key ?? '')
      expect(deliveries.slice(0, 5).every((d) => d.kind === 'error')).toBe(true)
      expect(deliveries[5].kind).not.toBe('error')
    })

    it('gives every fixture error a reason to render', async () => {
      const store = useDevicesStore()
      await store.loadDevices()
      await store.loadAllDeliveries()

      const errors = store.devices
        .flatMap((d) => store.deliveriesFor(d.key))
        .filter((d) => d.kind === 'error')
      expect(errors.length).toBeGreaterThan(0)
      expect(errors.every((d) => d.reason !== null)).toBe(true)
      // Only errors carry one.
      const others = store.devices
        .flatMap((d) => store.deliveriesFor(d.key))
        .filter((d) => d.kind !== 'error')
      expect(others.every((d) => d.reason === null)).toBe(true)
    })

    it('includes a photo delivery whose image is no longer available', async () => {
      const store = useDevicesStore()
      await store.loadDevices()
      await store.loadAllDeliveries()

      const all = store.devices.flatMap((d) => store.deliveriesFor(d.key))
      expect(all.some((d) => d.kind === 'photo' && !d.imageAvailable)).toBe(true)
    })
  })

  // ── API mode ─────────────────────────────────────────────────────────────

  describe('API mode', () => {
    beforeEach(async () => {
      const { isApiMode } = await import('@/config')
      vi.mocked(isApiMode).mockReturnValue(true)

      const { devicesApi } = await import('@/api/devices')
      vi.mocked(devicesApi.fetchAll).mockResolvedValue({
        recordingEnabled: true,
        recentWindow: 20,
        devices: [
          {
            key: 'a1b2c3d4e5f6',
            name: 'living-room',
            model: 'ws7in3e',
            width: 800,
            height: 480,
            orientation: 'landscape',
            catalogKeys: ['photos'],
            sleepDurationSeconds: 300,
            wakeSchedule: [],
            lastDeliveredAt: '2026-08-19T12:30:00Z',
            recentDeliveryCount: 1,
            recentErrorCount: 0,
          },
        ],
      })
      vi.mocked(devicesApi.fetchDeliveries).mockResolvedValue([
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
      ])
    })

    it('carries the recording flag through from the server', async () => {
      const { devicesApi } = await import('@/api/devices')
      vi.mocked(devicesApi.fetchAll).mockResolvedValueOnce({
        recordingEnabled: false,
        recentWindow: 20,
        devices: [],
      })

      const store = useDevicesStore()
      await store.loadDevices()

      expect(store.recordingEnabled).toBe(false)
    })

    it('loads the display list in the order the server returned it', async () => {
      const { devicesApi } = await import('@/api/devices')
      const store = useDevicesStore()

      await store.loadDevices()

      expect(devicesApi.fetchAll).toHaveBeenCalledOnce()
      expect(store.devices.map((d) => d.key)).toEqual(['a1b2c3d4e5f6'])
      expect(store.recentWindow).toBe(20)
    })

    it('records an error and stops loading when the list request fails', async () => {
      const { devicesApi } = await import('@/api/devices')
      vi.mocked(devicesApi.fetchAll).mockRejectedValueOnce(new Error('backend unreachable'))
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      const store = useDevicesStore()
      await store.loadDevices()
      consoleSpy.mockRestore()

      expect(store.error).toBe('backend unreachable')
      expect(store.loading).toBe(false)
    })

    it('caches deliveries and does not refetch on a second call', async () => {
      const { devicesApi } = await import('@/api/devices')
      const store = useDevicesStore()

      await store.loadDeliveries('a1b2c3d4e5f6')
      await store.loadDeliveries('a1b2c3d4e5f6')
      await store.loadDeliveries('a1b2c3d4e5f6')

      expect(devicesApi.fetchDeliveries).toHaveBeenCalledOnce()
      expect(store.deliveriesFor('a1b2c3d4e5f6')).toHaveLength(1)
    })

    it('refetches when force is set', async () => {
      const { devicesApi } = await import('@/api/devices')
      const store = useDevicesStore()

      await store.loadDeliveries('a1b2c3d4e5f6')
      await store.loadDeliveries('a1b2c3d4e5f6', true)

      expect(devicesApi.fetchDeliveries).toHaveBeenCalledTimes(2)
    })

    it('fetches one delivery list per display so the strips can be drawn collapsed', async () => {
      const { devicesApi } = await import('@/api/devices')
      const store = useDevicesStore()

      await store.loadDevices()
      await store.loadAllDeliveries()

      expect(devicesApi.fetchDeliveries).toHaveBeenCalledWith('a1b2c3d4e5f6')
      expect(store.deliveriesFor('a1b2c3d4e5f6')).toHaveLength(1)
    })

    it('keeps a per-display error without blanking the others', async () => {
      const { devicesApi } = await import('@/api/devices')
      vi.mocked(devicesApi.fetchDeliveries).mockRejectedValueOnce(new Error('no such device'))
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      const store = useDevicesStore()
      await store.loadDeliveries('gone')
      await store.loadDeliveries('a1b2c3d4e5f6')
      consoleSpy.mockRestore()

      expect(store.deliveryErrorFor('gone')).toBe('no such device')
      expect(store.deliveryErrorFor('a1b2c3d4e5f6')).toBeNull()
      expect(store.deliveriesFor('a1b2c3d4e5f6')).toHaveLength(1)
      expect(store.isDeliveryLoading('gone')).toBe(false)
    })

    it('refresh reloads the list and every strip behind it', async () => {
      const { devicesApi } = await import('@/api/devices')
      const store = useDevicesStore()

      await store.loadDevices()
      await store.loadAllDeliveries()
      await store.refresh()

      expect(devicesApi.fetchAll).toHaveBeenCalledTimes(2)
      expect(devicesApi.fetchDeliveries).toHaveBeenCalledTimes(2)
    })
  })
})
