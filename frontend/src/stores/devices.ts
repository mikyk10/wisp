import { defineStore } from 'pinia'
import { ref } from 'vue'
import { isApiMode } from '@/config'
import { devicesApi } from '@/api/devices'
import type { Delivery, DeliveryReason, DeviceList } from '@/types'

/**
 * Displays and their recorded deliveries.
 *
 * As everywhere else in this app, the mock/API branch lives here and not in
 * the api layer: `src/api/*` only ever talks to a backend, and a store decides
 * whether there is one to talk to.
 */

/** Recorded window the mock fixture pretends the server keeps. */
const MOCK_RECENT_WINDOW = 20

const MINUTE = 60
const HOUR = 60 * MINUTE

/** Reasons cycled through the fixture's failing display, so the wording shows. */
const MOCK_ERROR_REASONS: readonly DeliveryReason[] = ['file_missing', 'db_error', 'load_failed']

function isoAgo(now: number, seconds: number): string {
  return new Date(now - seconds * 1000).toISOString()
}

/**
 * Three displays covering the states this drawer exists to show:
 * one delivering normally, one whose recent history ends in a run of errors,
 * and one that has never been handed an image at all.
 *
 * Timestamps are derived from the caller's `now` rather than frozen into
 * literals, so the mock UI always shows plausible ages instead of drifting
 * years into the past.
 */
function mockDeviceList(now: number): DeviceList {
  return {
    // The mock server is recording; the drawer's recording-off notice is
    // exercised in the unit tests rather than shown permanently here, where it
    // would misdescribe a fixture that does keep producing new deliveries.
    recordingEnabled: true,
    recentWindow: MOCK_RECENT_WINDOW,
    devices: [
      {
        key: 'a1b2c3d4e5f6',
        name: 'living-room',
        model: 'ws7in3e',
        width: 800,
        height: 480,
        orientation: 'landscape',
        catalogKeys: ['photos'],
        sleepDurationSeconds: 5 * MINUTE,
        wakeSchedule: ['*/30 7-16 * * *'],
        lastDeliveredAt: isoAgo(now, 4 * MINUTE),
        recentDeliveryCount: 12,
        recentErrorCount: 0,
      },
      {
        key: '9c8d7e6f5a4b',
        name: 'hallway',
        model: 'ws7in3e',
        width: 800,
        height: 480,
        orientation: 'landscape',
        catalogKeys: ['photos', 'artwork'],
        sleepDurationSeconds: 30 * MINUTE,
        wakeSchedule: [],
        lastDeliveredAt: isoAgo(now, 26 * HOUR),
        recentDeliveryCount: 9,
        recentErrorCount: 5,
      },
      {
        key: 'dev',
        name: 'desk-test',
        model: 'ws4in0e',
        width: 400,
        height: 600,
        orientation: 'portrait',
        catalogKeys: ['photos'],
        sleepDurationSeconds: 24 * HOUR,
        wakeSchedule: [],
        lastDeliveredAt: null,
        recentDeliveryCount: 0,
        recentErrorCount: 0,
      },
    ],
  }
}

/**
 * Mock deliveries, newest first — the same order the API documents.
 * 'dev' is absent on purpose: it has never delivered.
 */
function mockDeliveries(now: number, deviceKey: string): Delivery[] {
  if (deviceKey === 'a1b2c3d4e5f6') {
    return Array.from({ length: 12 }, (_, i) => {
      const deliveredAt = isoAgo(now, 4 * MINUTE + i * 5 * MINUTE)
      const requestedSleepSeconds = 5 * MINUTE
      if (i === 3) {
        return {
          deliveredAt,
          kind: 'colorbar' as const,
          reason: null,
          imageId: null,
          catalogKey: null,
          source: null,
          requestedSleepSeconds,
          imageAvailable: false,
        }
      }
      // One photo whose file has since been removed from the catalog.
      const imageAvailable = i !== 6
      return {
        deliveredAt,
        kind: 'photo' as const,
        reason: null,
        imageId: 4800 + i,
        catalogKey: 'photos',
        source: `/mnt/photos/IMG_${(421 + i).toString().padStart(4, '0')}.jpg`,
        requestedSleepSeconds,
        imageAvailable,
      }
    })
  }

  if (deviceKey === '9c8d7e6f5a4b') {
    // Newest five are errors: reversed onto the strip they read as a solid run
    // at the right-hand end, which is the pattern the strip exists to surface.
    const errors: Delivery[] = Array.from({ length: 5 }, (_, i) => ({
      deliveredAt: isoAgo(now, 26 * HOUR + i * 30 * MINUTE),
      kind: 'error' as const,
      reason: MOCK_ERROR_REASONS[i % MOCK_ERROR_REASONS.length],
      imageId: null,
      catalogKey: 'photos',
      source: null,
      requestedSleepSeconds: 30 * MINUTE,
      imageAvailable: false,
    }))
    const photos: Delivery[] = Array.from({ length: 4 }, (_, i) => ({
      deliveredAt: isoAgo(now, 28.5 * HOUR + i * 30 * MINUTE),
      kind: 'photo' as const,
      reason: null,
      imageId: 4700 + i,
      catalogKey: 'artwork',
      source: `/mnt/artwork/plate-${i + 1}.png`,
      requestedSleepSeconds: 30 * MINUTE,
      imageAvailable: true,
    }))
    return [...errors, ...photos]
  }

  return []
}

export const useDevicesStore = defineStore('devices', () => {
  // ── State ────────────────────────────────────────────────────────────────
  const devices = ref<DeviceList['devices']>([])
  /**
   * Whether the server is still writing deliveries down. Defaults to true so a
   * drawer that has not loaded yet never claims recording has stopped.
   */
  const recordingEnabled = ref(true)
  const recentWindow = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  /** Deliveries per device key. Presence of a key means "already loaded". */
  const deliveries = ref<Record<string, Delivery[]>>({})
  const deliveryLoading = ref<Record<string, boolean>>({})
  const deliveryError = ref<Record<string, string | null>>({})

  // ── Getters ──────────────────────────────────────────────────────────────
  function deliveriesFor(deviceKey: string): Delivery[] {
    return deliveries.value[deviceKey] ?? []
  }

  function isDeliveryLoading(deviceKey: string): boolean {
    return deliveryLoading.value[deviceKey] === true
  }

  function deliveryErrorFor(deviceKey: string): string | null {
    return deliveryError.value[deviceKey] ?? null
  }

  // ── Actions ──────────────────────────────────────────────────────────────
  /**
   * Load the display list. Order is the server's (service.yaml order) and is
   * never re-sorted here: a list that reshuffles by staleness or error count
   * between refreshes moves rows out from under whoever is reading them.
   */
  async function loadDevices() {
    loading.value = true
    error.value = null

    if (!isApiMode()) {
      const fixture = mockDeviceList(Date.now())
      devices.value = fixture.devices
      recordingEnabled.value = fixture.recordingEnabled
      recentWindow.value = fixture.recentWindow
      loading.value = false
      return
    }

    try {
      const fetched = await devicesApi.fetchAll()
      devices.value = fetched.devices
      recordingEnabled.value = fetched.recordingEnabled
      recentWindow.value = fetched.recentWindow
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch displays'
      console.error('Device fetch error:', err)
    } finally {
      loading.value = false
    }
  }

  /**
   * Load one display's deliveries.
   *
   * Cached by device key: the strip needs every display's sequence as soon as
   * the drawer opens, so expanding a panel must not re-fetch what is already
   * on screen. `force` is for the explicit refresh button only.
   */
  async function loadDeliveries(deviceKey: string, force = false) {
    if (!force && deliveries.value[deviceKey] !== undefined) return

    deliveryLoading.value = { ...deliveryLoading.value, [deviceKey]: true }
    deliveryError.value = { ...deliveryError.value, [deviceKey]: null }

    if (!isApiMode()) {
      deliveries.value = { ...deliveries.value, [deviceKey]: mockDeliveries(Date.now(), deviceKey) }
      deliveryLoading.value = { ...deliveryLoading.value, [deviceKey]: false }
      return
    }

    try {
      const fetched = await devicesApi.fetchDeliveries(deviceKey)
      deliveries.value = { ...deliveries.value, [deviceKey]: fetched }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch deliveries'
      deliveryError.value = { ...deliveryError.value, [deviceKey]: message }
      console.error('Delivery fetch error:', err)
    } finally {
      deliveryLoading.value = { ...deliveryLoading.value, [deviceKey]: false }
    }
  }

  /**
   * Fill in the delivery sequence for every known display.
   *
   * The list endpoint carries counts, not sequences, so the collapsed strips
   * cannot be drawn from it — one request per display is what makes a run of
   * consecutive errors visible without expanding anything. Failures are
   * settled individually so one unreachable display cannot blank the rest.
   */
  async function loadAllDeliveries(force = false) {
    await Promise.allSettled(devices.value.map((device) => loadDeliveries(device.key, force)))
  }

  /** Full reload: the display list and every strip behind it. */
  async function refresh() {
    await loadDevices()
    await loadAllDeliveries(true)
  }

  return {
    // state
    devices,
    recordingEnabled,
    recentWindow,
    loading,
    error,
    deliveries,
    deliveryLoading,
    deliveryError,
    // getters
    deliveriesFor,
    isDeliveryLoading,
    deliveryErrorFor,
    // actions
    loadDevices,
    loadDeliveries,
    loadAllDeliveries,
    refresh,
  }
})
