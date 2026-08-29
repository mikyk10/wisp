import { describe, it, expect } from 'vitest'
import {
  clampFraction,
  entryForIndex,
  fractionToIndex,
  indexToFraction,
  yearMarks,
} from '../scrubber'
import type { TimelineEntry } from '@/types'

function entry(key: string, year: number, month: number, startIndex: number, count: number): TimelineEntry {
  return { key, label: `${year}/${String(month).padStart(2, '0')}`, year, month, startIndex, count }
}

// Newest-first, ascending startIndex — the order timelineEntries provides.
// 100 photos: a small recent month, a large one, an older year.
const entries: TimelineEntry[] = [
  entry('2024-12', 2024, 12, 0, 10),
  entry('2024-01', 2024, 1, 10, 40),
  entry('2023-06', 2023, 6, 50, 50),
]

describe('clampFraction', () => {
  it('confines any input to [0, 1]', () => {
    expect(clampFraction(-0.5)).toBe(0)
    expect(clampFraction(0.5)).toBe(0.5)
    expect(clampFraction(1.5)).toBe(1)
  })

  it('reads a non-finite value as the top, not as a crash', () => {
    expect(clampFraction(NaN)).toBe(0)
    expect(clampFraction(Infinity)).toBe(0)
  })
})

describe('fractionToIndex', () => {
  it('maps the ends of the rail to the ends of the list', () => {
    expect(fractionToIndex(0, 100)).toBe(0)
    expect(fractionToIndex(1, 100)).toBe(99)
  })

  it('divides the rail into equal bands, one per photo', () => {
    // Floor semantics: a position anywhere inside a band belongs to that
    // band's photo. Rounding would halve the first and last bands, which is
    // the unevenness a proportional rail exists to avoid.
    expect(fractionToIndex(0.009, 100)).toBe(0)
    expect(fractionToIndex(0.01, 100)).toBe(1)
    expect(fractionToIndex(0.999, 100)).toBe(99)
  })

  it('answers 0 for an empty list rather than -1 or NaN', () => {
    expect(fractionToIndex(0.5, 0)).toBe(0)
  })
})

describe('indexToFraction', () => {
  it('is the top of the band fractionToIndex reads', () => {
    for (const index of [0, 37, 99]) {
      expect(fractionToIndex(indexToFraction(index, 100), 100)).toBe(index)
    }
  })
})

describe('entryForIndex', () => {
  it('finds the month whose photos contain the index', () => {
    expect(entryForIndex(entries, 0)?.key).toBe('2024-12')
    expect(entryForIndex(entries, 9)?.key).toBe('2024-12')
    expect(entryForIndex(entries, 10)?.key).toBe('2024-01')
    expect(entryForIndex(entries, 49)?.key).toBe('2024-01')
    expect(entryForIndex(entries, 50)?.key).toBe('2023-06')
    expect(entryForIndex(entries, 99)?.key).toBe('2023-06')
  })

  it('answers null before the first dated photo', () => {
    // Undated photos can sort in front of every dated one (where they land is
    // the database's decision), leaving indices no month claims. Naming the
    // neighbouring month would put a date on the dateless.
    const withUndatedFront = [
      entry('2024-12', 2024, 12, 20, 10),
      entry('2024-01', 2024, 1, 30, 70),
    ]
    expect(entryForIndex(withUndatedFront, 0)).toBeNull()
    expect(entryForIndex(withUndatedFront, 19)).toBeNull()
    expect(entryForIndex(withUndatedFront, 20)?.key).toBe('2024-12')
  })

  it('answers null past the last dated photo', () => {
    // The mirror case: undated photos sorted to the back.
    expect(entryForIndex(entries, 100)).toBeNull()
    expect(entryForIndex(entries, 500)).toBeNull()
  })

  it('answers null for a negative index and for no entries at all', () => {
    expect(entryForIndex(entries, -1)).toBeNull()
    expect(entryForIndex([], 0)).toBeNull()
  })
})

describe('yearMarks', () => {
  it('places one mark per year, where that year begins on the rail', () => {
    const marks = yearMarks(entries, 100, 1000, 28)
    expect(marks).toEqual([
      { year: 2024, fraction: 0 },
      { year: 2023, fraction: 0.5 }, // 2023 starts at index 50 of 100
    ])
  })

  it('drops a year that would sit on top of the one above it', () => {
    // 2023 starts at fraction 0.5; on a 40px rail that is 20px below 2024,
    // inside the 28px minimum — so 2023 goes, 2024 stays.
    const marks = yearMarks(entries, 100, 40, 28)
    expect(marks).toEqual([{ year: 2024, fraction: 0 }])
  })

  it('keeps a dense stretch of years only as far as the rail has room', () => {
    // Ten years of 10 photos each: marks every 10% of the rail. At 100px,
    // every mark is 10px from its neighbour — under a 25px minimum, roughly
    // every third survives, the newest always first.
    const decade: TimelineEntry[] = Array.from({ length: 10 }, (_, i) =>
      entry(`${2024 - i}-01`, 2024 - i, 1, i * 10, 10),
    )
    const marks = yearMarks(decade, 100, 100, 25)
    expect(marks.map((m) => m.year)).toEqual([2024, 2021, 2018, 2015])
  })

  it('has nothing to say about an unmeasured rail or an empty catalogue', () => {
    expect(yearMarks(entries, 100, 0, 28)).toEqual([])
    expect(yearMarks(entries, 0, 1000, 28)).toEqual([])
  })
})
