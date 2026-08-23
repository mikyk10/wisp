import { apiClient } from './client'
import { API_PATHS } from '@/config'
import type { Delivery, DeliveryKind, DeliveryReason, Device, DeviceList } from '@/types'

/**
 * Device and delivery endpoints.
 *
 * The wire format is snake_case; everything below the api layer is camelCase,
 * so the two `to*` functions are the only place the two spellings meet. They
 * are also deliberately total: every field gets a fallback, because a frame
 * that has never delivered anything legitimately produces a record made
 * almost entirely of nulls, and a missing field must not blank the drawer.
 */

interface DeviceResponseItem {
  key?: string
  name?: string
  model?: string
  width?: number
  height?: number
  orientation?: string
  catalog_keys?: string[]
  sleep_duration_seconds?: number
  wake_schedule?: string[]
  last_delivered_at?: string | null
  recent_delivery_count?: number
  recent_error_count?: number
}

interface DevicesResponse {
  recording_enabled?: boolean
  recent_window?: number
  devices?: DeviceResponseItem[]
}

interface DeliveryResponseItem {
  delivered_at?: string
  kind?: string
  reason?: string | null
  image_id?: number | null
  catalog_key?: string | null
  source?: string | null
  requested_sleep_seconds?: number | null
  image_available?: boolean
}

interface DeliveriesResponse {
  device_key?: string
  deliveries?: DeliveryResponseItem[]
}

function toDevice(raw: DeviceResponseItem): Device {
  return {
    key: raw.key ?? '',
    // A display with no configured name is addressed by its key everywhere else,
    // so fall back to it rather than rendering an empty row.
    name: raw.name ?? raw.key ?? '',
    model: raw.model ?? '',
    width: raw.width ?? 0,
    height: raw.height ?? 0,
    orientation: raw.orientation ?? '',
    catalogKeys: raw.catalog_keys ?? [],
    sleepDurationSeconds: raw.sleep_duration_seconds ?? 0,
    wakeSchedule: raw.wake_schedule ?? [],
    lastDeliveredAt: raw.last_delivered_at ?? null,
    recentDeliveryCount: raw.recent_delivery_count ?? 0,
    recentErrorCount: raw.recent_error_count ?? 0,
  }
}

function toDelivery(raw: DeliveryResponseItem): Delivery {
  return {
    deliveredAt: raw.delivered_at ?? '',
    // An unrecognised kind is carried through verbatim: the UI has a default
    // branch for it, and mapping it onto a known kind here would misreport
    // what the server actually did. The same goes for an unrecognised reason.
    kind: (raw.kind ?? '') as DeliveryKind,
    reason: (raw.reason ?? null) as DeliveryReason | null,
    imageId: raw.image_id ?? null,
    // The server sends null for "no catalogue was involved". The empty-string
    // branch guards the older shape, where catalog_key was a plain string and
    // the same condition arrived as "". Both normalise to null.
    //
    // Note what null does NOT mean: it is not a failure signal. The commoner
    // error path — a provider that knew which catalogue it was reading and
    // gave up on it — keeps its key, so an error record with a catalogue is
    // normal. Nothing downstream reads this field to decide anything except
    // whether a thumbnail URL can be built.
    catalogKey: raw.catalog_key ? raw.catalog_key : null,
    source: raw.source ?? null,
    requestedSleepSeconds: raw.requested_sleep_seconds ?? null,
    imageAvailable: raw.image_available ?? false,
  }
}

export const devicesApi = {
  /** GET /api/devices — configured displays in server (service.yaml) order. */
  async fetchAll(): Promise<DeviceList> {
    const { data } = await apiClient.get<DevicesResponse>(API_PATHS.devices())
    return {
      // Absent means an older server that does not report the setting. Assume
      // recording is on: announcing "recording is off" because a field is
      // missing would be a false alarm about data that is in fact live.
      recordingEnabled: data.recording_enabled ?? true,
      recentWindow: data.recent_window ?? 0,
      devices: (data.devices ?? []).map(toDevice),
    }
  },

  /**
   * GET /api/device/{deviceKey}/deliveries — recorded deliveries, newest first.
   * The order is the server's; nothing here re-sorts it.
   */
  async fetchDeliveries(deviceKey: string): Promise<Delivery[]> {
    const { data } = await apiClient.get<DeliveriesResponse>(
      API_PATHS.deviceDeliveries(deviceKey)
    )
    return (data.deliveries ?? []).map(toDelivery)
  },
}
