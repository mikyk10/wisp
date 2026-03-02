/** A single photo entry as returned by the API / NDJSON stream. */
export interface Photo {
  id: number
  url: string
  enabled: boolean
  timestamp: string
}

/** Raw record from the NDJSON stream (before the `url` field is added). */
export interface PhotoRecord {
  id: number
  enabled: boolean
  timestamp: string
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
