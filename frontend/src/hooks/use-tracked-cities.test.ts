import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import {
  MAX_TRACKED_CITIES,
  useTrackedCities,
} from './use-tracked-cities'

const STORAGE_KEY = 'weather-watcher.tracked-cities'

describe('useTrackedCities', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('starts from the defaults on a first visit', () => {
    const { result } = renderHook(() => useTrackedCities())
    expect(result.current.cities).toEqual(['Singapore', 'Tokyo', 'London'])
  })

  it('restores a previously stored list', () => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify(['Oslo', 'Reykjavik']),
    )

    const { result } = renderHook(() => useTrackedCities())
    expect(result.current.cities).toEqual(['Oslo', 'Reykjavik'])
  })

  it('falls back to the defaults when storage holds corrupt JSON', () => {
    window.localStorage.setItem(STORAGE_KEY, '{not json')

    const { result } = renderHook(() => useTrackedCities())
    expect(result.current.cities).toEqual(['Singapore', 'Tokyo', 'London'])
  })

  it('falls back to the defaults when storage holds the wrong shape', () => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ cities: ['Oslo'] }))

    const { result } = renderHook(() => useTrackedCities())
    expect(result.current.cities).toEqual(['Singapore', 'Tokyo', 'London'])
  })

  it('drops non-string and blank entries from a stored list', () => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify(['Oslo', 42, null, '   ', 'Cairo']),
    )

    const { result } = renderHook(() => useTrackedCities())
    expect(result.current.cities).toEqual(['Oslo', 'Cairo'])
  })

  it('adds a city and persists it', () => {
    const { result } = renderHook(() => useTrackedCities())

    act(() => {
      expect(result.current.addCity('  Oslo  ')).toEqual({ ok: true })
    })

    expect(result.current.cities).toContain('Oslo')
    expect(
      JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? '[]'),
    ).toContain('Oslo')
  })

  it('rejects an empty name', () => {
    const { result } = renderHook(() => useTrackedCities())

    for (const input of ['', '   ', '\t']) {
      act(() => {
        expect(result.current.addCity(input)).toEqual({
          ok: false,
          reason: 'empty',
        })
      })
    }
  })

  it('rejects a duplicate regardless of case or padding', () => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(['Singapore']))

    const { result } = renderHook(() => useTrackedCities())

    act(() => {
      expect(result.current.addCity('singapore')).toEqual({
        ok: false,
        reason: 'duplicate',
      })
      expect(result.current.addCity('  SINGAPORE  ')).toEqual({
        ok: false,
        reason: 'duplicate',
      })
    })
  })

  it('keeps the spelling the user typed', () => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(['Singapore']))

    const { result } = renderHook(() => useTrackedCities())
    act(() => {
      result.current.addCity('SINGAPORE')
    })

    expect(result.current.cities).toEqual(['Singapore'])
  })

  it('removes a city', () => {
    const { result } = renderHook(() => useTrackedCities())

    act(() => {
      result.current.removeCity('Tokyo')
    })

    expect(result.current.cities).toEqual(['Singapore', 'London'])
  })

  it('restores the defaults', () => {
    const { result } = renderHook(() => useTrackedCities())

    act(() => {
      result.current.removeCity('Singapore')
    })
    act(() => {
      result.current.restoreDefaults()
    })

    expect(result.current.cities).toEqual(['Singapore', 'Tokyo', 'London'])
  })

  /**
   * The backend rejects a request listing more cities than MAX_CITIES, and it
   * rejects the whole request. Enforcing the same ceiling here is what stops a
   * user at the limit from breaking their entire dashboard.
   */
  it('refuses to exceed the backend city limit', () => {
    const many = Array.from(
      { length: MAX_TRACKED_CITIES },
      (_, index) => `City${index}`,
    )
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(many))

    const { result } = renderHook(() => useTrackedCities())
    expect(result.current.cities).toHaveLength(MAX_TRACKED_CITIES)

    act(() => {
      expect(result.current.addCity('OneTooMany')).toEqual({
        ok: false,
        reason: 'limit',
      })
    })

    expect(result.current.cities).toHaveLength(MAX_TRACKED_CITIES)
  })

  it('still accepts a city at exactly the limit', () => {
    const nearly = Array.from(
      { length: MAX_TRACKED_CITIES - 1 },
      (_, index) => `City${index}`,
    )
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(nearly))

    const { result } = renderHook(() => useTrackedCities())

    act(() => {
      expect(result.current.addCity('LastOne')).toEqual({ ok: true })
    })
    expect(result.current.cities).toHaveLength(MAX_TRACKED_CITIES)
  })
})
