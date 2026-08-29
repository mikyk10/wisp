/**
 * Position mapping for the timeline scrubber.
 *
 * The whole design rests on one property of the grid: every photo occupies the
 * same amount of scroll height (rows are uniform, each holds a fixed number of
 * cells). That makes "fraction of the rail" and "fraction of the photo list"
 * the same number, so a month's share of the rail is its share of the
 * catalogue — a month with half the photos covers half the rail — and the spot
 * a finger lands on is the spot the grid scrolls to, with no correction pass.
 *
 * Everything here is pure and works on the store's `timelineEntries`, which
 * are ordered newest-first. Newest-first is also ascending-startIndex order,
 * because the server lists photos newest-first; the binary search below relies
 * on that and on nothing else.
 */
import type { TimelineEntry } from '@/types'

/** Clamp a rail position to the [0, 1] it is supposed to be. */
export function clampFraction(fraction: number): number {
  if (!Number.isFinite(fraction)) return 0
  return Math.min(1, Math.max(0, fraction))
}

/**
 * The photo index a rail position lands on.
 *
 * Floor, not round: the rail is divided into `totalPhotos` equal bands and a
 * position inside a band belongs to that band's photo. Rounding would give the
 * first and last photo half-width bands and every other photo a full one,
 * which is exactly the unevenness the proportional rail exists to avoid.
 */
export function fractionToIndex(fraction: number, totalPhotos: number): number {
  if (totalPhotos <= 0) return 0
  const index = Math.floor(clampFraction(fraction) * totalPhotos)
  return Math.min(index, totalPhotos - 1)
}

/** The rail position of a photo index — the top of its band. */
export function indexToFraction(index: number, totalPhotos: number): number {
  if (totalPhotos <= 0) return 0
  return clampFraction(index / totalPhotos)
}

/**
 * The month whose photos contain the given index, or null when none does.
 *
 * Null is a real answer, not a failure. Photos without a usable date are in
 * the grid but in no month bucket, and where they sort is the database's
 * decision — one dialect puts them before every dated photo, another after —
 * so the covered range can start late, end early, or both. Labelling an
 * undated stretch with whichever month sits next to it would put a date on
 * photos whose defining property is that they have none; the caller shows
 * "no date" instead.
 */
export function entryForIndex(entries: TimelineEntry[], index: number): TimelineEntry | null {
  if (index < 0) return null

  // Binary search for the last entry with startIndex <= index.
  let lo = 0
  let hi = entries.length - 1
  let candidate = -1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    if (entries[mid].startIndex <= index) {
      candidate = mid
      lo = mid + 1
    } else {
      hi = mid - 1
    }
  }
  if (candidate === -1) return null

  const entry = entries[candidate]
  return index < entry.startIndex + entry.count ? entry : null
}

export interface YearMark {
  year: number
  /** Rail position of the year's newest photo — where its stretch begins. */
  fraction: number
}

/**
 * The year labels the rail has room for.
 *
 * One candidate per year, placed where that year's newest month begins. Years
 * are then thinned against the rail's actual height: a mark closer than
 * `minGapPx` to the one already kept above it is dropped rather than drawn on
 * top of it. Thinning keeps the newest year and drops from the crowded ones,
 * so a catalogue whose last twelve months dwarf a thin decade of backlog
 * shows the recent year and as much of the backlog as fits — which is the
 * honest picture: label density follows photo density.
 */
export function yearMarks(
  entries: TimelineEntry[],
  totalPhotos: number,
  railHeightPx: number,
  minGapPx: number,
): YearMark[] {
  if (totalPhotos <= 0 || railHeightPx <= 0) return []

  // Entries are newest-first, so the first entry seen for a year is its
  // newest month, which is where the year's stretch starts on the rail.
  const candidates: YearMark[] = []
  let lastYear: number | null = null
  for (const entry of entries) {
    if (entry.year === lastYear) continue
    lastYear = entry.year
    candidates.push({ year: entry.year, fraction: indexToFraction(entry.startIndex, totalPhotos) })
  }

  const kept: YearMark[] = []
  for (const mark of candidates) {
    const prev = kept[kept.length - 1]
    if (prev && (mark.fraction - prev.fraction) * railHeightPx < minGapPx) continue
    kept.push(mark)
  }
  return kept
}
