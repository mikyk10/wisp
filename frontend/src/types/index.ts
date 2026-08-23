/** A single photo entry as returned by the API / NDJSON stream. */
export interface Photo {
  id: number
  url: string
  enabled: boolean
  timestamp: string
  /**
   * Always an array. Tags ride along with the listing rather than being asked
   * for per photo: the grid shows hundreds of cards at once, so anything it
   * needs per card is either already here or it is a request per card.
   */
  tags: string[]
}

/** Raw record from the NDJSON stream (before the `url` field is added). */
export interface PhotoRecord {
  id: number
  enabled: boolean
  timestamp: string
  tags?: string[]
}

/** One tag and how many photos in the current catalogue carry it. */
export interface TagUsage {
  name: string
  count: number
}

/** One bucket in the timeline sidebar. */
export interface TimelineEntry {
  key: string
  label: string
  year: number
  month: number
  startIndex: number
  count: number
}

/**
 * Kind of payload one delivery carried.
 *
 * The backend may grow kinds this build has never heard of, so every consumer
 * needs a default branch: values outside this union are passed through from
 * the wire untouched rather than coerced into a kind we made up.
 */
export type DeliveryKind = 'photo' | 'http' | 'colorbar' | 'error'

/**
 * Why an error card went out instead of a picture.
 *
 * Set only on `kind: 'error'` records and null everywhere else. The server
 * sends the code and not prose on purpose: the wording belongs to the client,
 * where it can be reworded without a server release. As with DeliveryKind, a
 * code outside this union is passed through rather than coerced.
 */
export type DeliveryReason =
  | 'no_images'
  | 'db_error'
  | 'file_missing'
  | 'no_catalog'
  | 'no_provider'
  | 'unknown_display'
  | 'load_failed'
  | 'encode_failed'

/** A display configured on the server, plus a summary of its recent deliveries. */
export interface Device {
  key: string
  name: string
  model: string
  width: number
  height: number
  orientation: string
  catalogKeys: string[]
  sleepDurationSeconds: number
  wakeSchedule: string[]
  /** null when the server has never handed this display an image. */
  lastDeliveredAt: string | null
  recentDeliveryCount: number
  recentErrorCount: number
}

/** GET /api/devices — the configured displays and the size of the recorded window. */
export interface DeviceList {
  /**
   * Whether deliveries are being written down at all right now.
   *
   * This qualifies every count and timestamp below it. With recording off the
   * numbers stop moving while still reporting real facts about the past, and a
   * reader taking them as current would conclude a frame had stopped when what
   * stopped was the bookkeeping.
   */
  recordingEnabled: boolean
  recentWindow: number
  devices: Device[]
}

/**
 * One recorded delivery.
 *
 * "Delivered" is the server's own view: it wrote bytes to the response. It
 * cannot know whether the frame woke, decoded them, or drew them, so nothing
 * derived from this record may claim the image is on screen.
 */
export interface Delivery {
  deliveredAt: string
  kind: DeliveryKind
  /** Why this one failed; null for every kind other than 'error'. */
  reason: DeliveryReason | null
  imageId: number | null
  catalogKey: string | null
  source: string | null
  requestedSleepSeconds: number | null
  /**
   * False when the backing image is gone. The image endpoint answers a deleted
   * photo with a decodable error card under a 404, which a browser renders
   * like any other image — so this flag, not a load failure, is what tells the
   * UI to draw a placeholder instead.
   */
  imageAvailable: boolean
}
