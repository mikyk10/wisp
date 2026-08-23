import { describe, it, expect } from 'vitest'
import { formatAbsoluteTime, formatDuration, formatRelativeTime } from '../time'

const NOW = Date.UTC(2026, 7, 19, 12, 30, 0)

function ago(seconds: number): string {
  return new Date(NOW - seconds * 1000).toISOString()
}

describe('formatRelativeTime', () => {
  it('reads "just now" for a delivery seconds old', () => {
    expect(formatRelativeTime(ago(3), NOW)).toBe('just now')
  })

  it('scales through seconds, minutes, hours, days, months and years', () => {
    expect(formatRelativeTime(ago(30), NOW)).toBe('30 seconds ago')
    expect(formatRelativeTime(ago(4 * 60), NOW)).toBe('4 minutes ago')
    expect(formatRelativeTime(ago(3 * 3600), NOW)).toBe('3 hours ago')
    expect(formatRelativeTime(ago(26 * 3600), NOW)).toBe('1 day ago')
    expect(formatRelativeTime(ago(70 * 86400), NOW)).toBe('2 months ago')
    expect(formatRelativeTime(ago(400 * 86400), NOW)).toBe('1 year ago')
  })

  it('keeps the sign for a clock-skewed timestamp in the future', () => {
    expect(formatRelativeTime(ago(-5 * 60), NOW)).toBe('in 5 minutes')
  })

  it('returns an empty string for a missing or unparseable timestamp', () => {
    expect(formatRelativeTime(null, NOW)).toBe('')
    expect(formatRelativeTime('', NOW)).toBe('')
    expect(formatRelativeTime('not a date', NOW)).toBe('')
  })
})

describe('formatAbsoluteTime', () => {
  it('renders a readable date and time', () => {
    const formatted = formatAbsoluteTime('2026-08-19T12:30:00Z')
    // Time zone is the viewer's, so only the stable parts are asserted.
    expect(formatted).toContain('2026')
    expect(formatted).toMatch(/Aug (19|20)/)
  })

  it('returns an empty string for a missing or unparseable timestamp', () => {
    expect(formatAbsoluteTime(null)).toBe('')
    expect(formatAbsoluteTime('not a date')).toBe('')
  })
})

describe('formatDuration', () => {
  it('formats seconds, minutes and hours', () => {
    expect(formatDuration(45)).toBe('45s')
    expect(formatDuration(300)).toBe('5m')
    expect(formatDuration(3600)).toBe('1h')
    expect(formatDuration(5400)).toBe('1h 30m')
    expect(formatDuration(86400)).toBe('24h')
  })

  it('returns an empty string when there is no duration to show', () => {
    expect(formatDuration(null)).toBe('')
    expect(formatDuration(0)).toBe('')
    expect(formatDuration(-1)).toBe('')
    expect(formatDuration(Number.NaN)).toBe('')
  })
})
