import { describe, expect, it } from 'vitest'

import {
  conditionKind,
  conditionStyle,
  formatClockTime,
  formatHourLabel,
  formatRelativeTime,
  formatTemperature,
} from './weather-conditions'

describe('conditionKind', () => {
  it('maps every documented WMO code to a category', () => {
    const expectations: Array<[number, string]> = [
      [0, 'clear'],
      [1, 'clear'],
      [2, 'cloudy'],
      [3, 'overcast'],
      [45, 'fog'],
      [48, 'fog'],
      [51, 'drizzle'],
      [57, 'drizzle'],
      [61, 'rain'],
      [82, 'rain'],
      [71, 'snow'],
      [86, 'snow'],
      [95, 'thunder'],
      [99, 'thunder'],
    ]

    for (const [code, expected] of expectations) {
      expect(conditionKind(code), `code ${code}`).toBe(expected)
    }
  })

  it('falls back rather than throwing on an unrecognised code', () => {
    for (const code of [-1, 999, 1234]) {
      expect(conditionKind(code)).toBe('overcast')
    }
  })

  it('always resolves an icon and a label', () => {
    for (const code of [0, 3, 45, 61, 71, 95, 999]) {
      const style = conditionStyle(code)
      expect(style.label.length).toBeGreaterThan(0)
      expect(style.icon).toBeDefined()
    }
  })
})

describe('formatTemperature', () => {
  it('rounds to a whole degree', () => {
    expect(formatTemperature(31.2)).toBe('31')
    expect(formatTemperature(31.8)).toBe('32')
    expect(formatTemperature(-0.4)).toBe('0')
  })
})

describe('formatRelativeTime', () => {
  const now = new Date('2026-10-10T12:00:00Z')

  it.each([
    ['2026-10-10T11:59:40Z', 'just now'],
    ['2026-10-10T11:59:00Z', '1 min ago'],
    ['2026-10-10T11:58:00Z', '2 min ago'],
    ['2026-10-10T11:45:00Z', '15 min ago'],
    ['2026-10-10T11:00:00Z', '1 hour ago'],
    ['2026-10-10T09:00:00Z', '3 hours ago'],
    ['2026-10-09T12:00:00Z', '1 day ago'],
    ['2026-10-07T12:00:00Z', '3 days ago'],
  ])('formats %s as "%s"', (iso, expected) => {
    expect(formatRelativeTime(iso, now)).toBe(expected)
  })

  it('degrades safely on an unparseable timestamp', () => {
    expect(formatRelativeTime('not-a-date', now)).toBe('just now')
  })
})

describe('clock formatting', () => {
  it('degrades safely on an unparseable timestamp', () => {
    expect(formatClockTime('nonsense')).toBe('--')
    expect(formatHourLabel('nonsense')).toBe('--')
  })

  it('produces a non-empty label for a valid timestamp', () => {
    const formatted = formatClockTime('2026-10-10T04:45:00Z')
    expect(formatted).not.toBe('--')
    expect(formatted.length).toBeGreaterThan(0)
  })
})
