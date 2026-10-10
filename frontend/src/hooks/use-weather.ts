import { useCallback, useEffect, useState } from 'react'

import { fetchWeather } from '@/lib/api/client'
import type { CityError, Report } from '@/lib/api/schemas'

export interface WeatherState {
  /** Cities that resolved, in the order the backend returned them. */
  reports: Report[]
  /** Cities that failed individually; the rest of the dashboard still works. */
  cityErrors: CityError[]
  /** True while a request is in flight, including background refreshes. */
  loading: boolean
  /** Set only when the whole request failed, not for individual cities. */
  error: Error | null
  /** Re-runs the request. Useful because the backend caches for an hour. */
  refresh: () => void
}

const INITIAL: Pick<WeatherState, 'reports' | 'cityErrors' | 'loading' | 'error'> = {
  reports: [],
  cityErrors: [],
  loading: false,
  error: null,
}

/**
 * Loads weather for the tracked cities.
 *
 * Two things worth calling out:
 *
 * - Race safety. Removing a city fires a new request while the previous one may
 *   still be in flight. Each effect run owns an AbortController and aborts the
 *   previous one on cleanup, so a slow stale response cannot land last and
 *   overwrite fresher state.
 *
 * - Refreshing keeps existing data on screen. Setting `loading` alone rather
 *   than clearing the reports means the dashboard does not flash a skeleton
 *   every time a city is added.
 */
export function useWeather(cities: readonly string[]): WeatherState {
  const [state, setState] = useState(INITIAL)
  const [attempt, setAttempt] = useState(0)

  // A stable primitive dependency. Depending on the array itself would refetch
  // on every render, since a new array is produced whenever the list changes.
  const citiesKey = cities.join('|')

  useEffect(() => {
    const requested = citiesKey === '' ? [] : citiesKey.split('|')

    if (requested.length === 0) {
      setState(INITIAL)
      return undefined
    }

    const controller = new AbortController()

    setState((previous) => ({ ...previous, loading: true, error: null }))

    fetchWeather(requested, { signal: controller.signal })
      .then((payload) => {
        setState({
          reports: payload.results,
          cityErrors: payload.errors,
          loading: false,
          error: null,
        })
      })
      .catch((caught: unknown) => {
        // A request superseded by a newer one is not a failure to report.
        if (caught instanceof DOMException && caught.name === 'AbortError') return

        setState({
          reports: [],
          cityErrors: [],
          loading: false,
          error: caught instanceof Error ? caught : new Error(String(caught)),
        })
      })

    return () => controller.abort()
  }, [citiesKey, attempt])

  const refresh = useCallback(() => setAttempt((n) => n + 1), [])

  return { ...state, refresh }
}
