/**
 * Time formatting for the device drawer.
 *
 * Everything here is built on the platform's Intl APIs — no date library is a
 * dependency of this project and none needs to be. The locale is pinned to
 * 'en' rather than left to the runtime: the UI is English throughout, and a
 * fixed locale keeps unit tests deterministic on any machine.
 */

const MINUTE = 60
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR
const MONTH = 30 * DAY
const YEAR = 365 * DAY

/** Below this many seconds a timestamp reads "just now" instead of "0 seconds ago". */
const JUST_NOW_SECONDS = 10

const relative = new Intl.RelativeTimeFormat('en', { numeric: 'always' })
const absolute = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' })

/** Parsed epoch milliseconds, or null for a missing or unparseable timestamp. */
function parse(iso: string | null): number | null {
  if (!iso) return null
  const ms = Date.parse(iso)
  return Number.isNaN(ms) ? null : ms
}

/**
 * "4 minutes ago" / "in 2 minutes" / "just now".
 *
 * `now` is injectable so callers (and tests) can pin the reference point.
 * Returns an empty string for a timestamp that cannot be parsed, so callers
 * can fall back to their own copy rather than print "Invalid Date".
 */
export function formatRelativeTime(iso: string | null, now: number = Date.now()): string {
  const ms = parse(iso)
  if (ms === null) return ''

  const deltaSeconds = Math.round((ms - now) / 1000)
  const magnitude = Math.abs(deltaSeconds)
  if (magnitude < JUST_NOW_SECONDS) return 'just now'

  // Sign is preserved throughout: a clock-skewed device can report a delivery
  // in the future, and "in 3 minutes" is more honest than clamping to zero.
  if (magnitude < MINUTE) return relative.format(deltaSeconds, 'second')
  if (magnitude < HOUR) return relative.format(Math.round(deltaSeconds / MINUTE), 'minute')
  if (magnitude < DAY) return relative.format(Math.round(deltaSeconds / HOUR), 'hour')
  if (magnitude < MONTH) return relative.format(Math.round(deltaSeconds / DAY), 'day')
  if (magnitude < YEAR) return relative.format(Math.round(deltaSeconds / MONTH), 'month')
  return relative.format(Math.round(deltaSeconds / YEAR), 'year')
}

/**
 * "Aug 19, 2026, 12:30 PM" in the viewer's own time zone.
 *
 * Always shown alongside the relative age: relative time answers "is this
 * stale?", the absolute stamp answers "stale since when?", and the second
 * question is the one you need when a frame has stopped asking for images.
 */
export function formatAbsoluteTime(iso: string | null): string {
  const ms = parse(iso)
  if (ms === null) return ''
  return absolute.format(new Date(ms))
}

/**
 * A sleep duration as "45s", "5m", "1h 30m", "24h".
 *
 * Used for both the configured sleep duration and the sleep the server asked
 * for in a single response; neither is a measurement of time actually slept.
 */
export function formatDuration(seconds: number | null): string {
  // Zero is "no duration recorded", not "zero seconds": the wire carries a
  // plain int, so an absent sleep request arrives as 0 rather than as null.
  if (seconds === null || !Number.isFinite(seconds) || seconds <= 0) return ''

  const total = Math.round(seconds)
  if (total < MINUTE) return `${total}s`

  const hours = Math.floor(total / HOUR)
  const minutes = Math.round((total % HOUR) / MINUTE)
  if (hours > 0 && minutes > 0) return `${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h`
  return `${minutes}m`
}
