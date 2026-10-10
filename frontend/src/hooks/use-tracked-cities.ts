import { useCallback, useEffect, useState } from 'react'

const STORAGE_KEY = 'weather-watcher.tracked-cities'

/** Cities tracked on a first visit, before the user changes anything. */
const DEFAULT_CITIES: readonly string[] = ['Singapore', 'Tokyo', 'London']

export type AddCityResult =
  | { ok: true }
  | { ok: false; reason: 'empty' | 'duplicate' | 'limit' }

/**
 * Matches the backend's MAX_CITIES default.
 *
 * The API rejects a request listing more cities than this, and it rejects the
 * whole request rather than the overflow. Without a matching limit here a user
 * who tracked 21 cities would see the entire dashboard fail, so the ceiling is
 * enforced before the request is ever made. Keep the two in step: the backend
 * value is configurable via MAX_CITIES, so this constant needs to follow it.
 */
export const MAX_TRACKED_CITIES = 20

/**
 * Reads the tracked list from localStorage.
 *
 * Falls back to the defaults when storage is unavailable or holds something
 * unexpected, rather than letting a corrupt value take the dashboard down.
 */
function readStoredCities(): string[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (raw === null) return [...DEFAULT_CITIES]

    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return [...DEFAULT_CITIES]

    return parsed.filter(
      (entry): entry is string => typeof entry === 'string' && entry.trim() !== '',
    )
  } catch {
    return [...DEFAULT_CITIES]
  }
}

/**
 * Manages the tracked city list, persisted to localStorage.
 *
 * De-duplication is case-insensitive but keeps the spelling the user typed, so
 * "singapore" and "Singapore" cannot both be tracked.
 */
export function useTrackedCities() {
  const [cities, setCities] = useState<string[]>(readStoredCities)

  // Persist on change. Guarded because localStorage throws when storage is
  // blocked or the quota is exhausted, and neither is worth interrupting the
  // session over.
  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(cities))
    } catch {
      // Persistence is a convenience; the dashboard still works without it.
    }
  }, [cities])

  const addCity = useCallback(
    (city: string): AddCityResult => {
      const trimmed = city.trim()
      if (trimmed === '') return { ok: false, reason: 'empty' }

      const exists = cities.some((c) => c.toLowerCase() === trimmed.toLowerCase())
      if (exists) return { ok: false, reason: 'duplicate' }

      if (cities.length >= MAX_TRACKED_CITIES) {
        return { ok: false, reason: 'limit' }
      }

      setCities((previous) => [...previous, trimmed])
      return { ok: true }
    },
    [cities],
  )

  const removeCity = useCallback((city: string) => {
    setCities((previous) => previous.filter((c) => c !== city))
  }, [])

  const restoreDefaults = useCallback(() => setCities([...DEFAULT_CITIES]), [])

  return { cities, addCity, removeCity, restoreDefaults }
}
