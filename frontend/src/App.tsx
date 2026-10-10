import {
  Suspense,
  lazy,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { Check, CloudSun, RefreshCw } from 'lucide-react'

import CityCard from '@/components/city-card'
import CityInput from '@/components/city-input'
import {
  CityCardSkeleton,
  EmptyState,
  ErrorState,
} from '@/components/state-display'
import { Button } from '@/components/ui/button'
import { useTrackedCities } from '@/hooks/use-tracked-cities'
import { useWeather } from '@/hooks/use-weather'
import { API_BASE_URL } from '@/lib/api/client'

// The charting library is most of the bundle and is not needed until a city is
// selected, so it becomes its own chunk fetched on demand.
const WeatherChart = lazy(() => import('@/components/weather-chart'))

export default function App() {
  const { cities, addCity, removeCity, restoreDefaults } = useTrackedCities()
  const { reports, cityErrors, loading, error, refresh } = useWeather(cities)

  const [selectedCity, setSelectedCity] = useState<string | null>(null)

  // Keyed lookups so rendering N cards does not scan the arrays N times.
  const reportsByCity = useMemo(
    () => new Map(reports.map((report) => [report.city, report])),
    [reports],
  )
  const errorsByCity = useMemo(
    () => new Map(cityErrors.map((cityError) => [cityError.city, cityError])),
    [cityErrors],
  )

  // Fall back to the first city with data so the chart is populated without
  // the user having to choose anything.
  const activeCity = selectedCity ?? reports[0]?.city ?? null
  const activeReport =
    activeCity === null ? undefined : reportsByCity.get(activeCity)

  const handleRemove = useCallback(
    (city: string) => {
      removeCity(city)
      // Clear the selection when the selected city goes away, otherwise the
      // chart empties out with nothing to fall back to.
      setSelectedCity((current) => (current === city ? null : current))
    },
    [removeCity],
  )

  return (
    <div className="min-h-dvh">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded-lg focus:bg-primary focus:px-4 focus:py-2 focus:text-sm focus:text-primary-foreground"
      >
        Skip to content
      </a>

      <TopBar
        loading={loading}
        cityCount={cities.length}
        onRefresh={refresh}
      />

      <main id="main" className="mx-auto w-full max-w-7xl px-5 pb-20">
        <CityInput onAdd={addCity} disabled={loading} />

        {cities.length === 0 ? (
          <div className="mt-8">
            <EmptyState onRestoreDefaults={restoreDefaults} />
          </div>
        ) : error !== null ? (
          // A whole-request failure, as distinct from one bad city among many.
          <div className="mt-8 max-w-xl">
            <ErrorState error={error} onRetry={refresh} />
          </div>
        ) : (
          <>
            {/* Auto-fit rather than fixed breakpoints: the number of columns
                follows the available width and the number of tracked cities,
                so the grid stays correct from one city to twenty without a
                column count being hard-coded per breakpoint. */}
            <div className="mt-6 grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(17rem,1fr))]">
              {cities.map((city) => {
                    const report = reportsByCity.get(city)
                    const cityError = errorsByCity.get(city)

                    // Per city, not per grid. While a request is in flight a
                    // city with neither a report nor an error is still pending,
                    // so it gets a skeleton. Deciding this once for the whole
                    // grid let a mixed response flash "no matching location"
                    // at cities that were simply still in flight.
                    if (loading && report === undefined && cityError === undefined) {
                      return <CityCardSkeleton key={city} />
                    }

                    return (
                      <CityCard
                        key={city}
                        city={city}
                        report={report}
                        error={cityError}
                        isSelected={city === activeCity}
                        onSelect={setSelectedCity}
                        onRemove={handleRemove}
                      />
                    )
                  })}
            </div>

            {activeReport !== undefined ? (
              <div className="mt-6">
                <Suspense
                  fallback={
                    <div
                      className="h-80 animate-pulse rounded-2xl border border-border bg-muted/60 motion-reduce:animate-none"
                      aria-hidden="true"
                    />
                  }
                >
                  <WeatherChart report={activeReport} />
                </Suspense>
              </div>
            ) : null}
          </>
        )}
      </main>
    </div>
  )
}

/** How long the completion tick stays up before the button returns to rest. */
const SETTLED_HOLD_MS = 1600

function RefreshButton({
  loading,
  onRefresh,
}: {
  loading: boolean
  onRefresh: () => void
}) {
  // A refresh that silently returns to rest reads as "nothing happened",
  // which is indistinguishable from a click that never registered.
  const [settled, setSettled] = useState(false)
  const wasLoading = useRef(loading)

  useEffect(() => {
    if (loading) {
      wasLoading.current = true
      return
    }
    if (!wasLoading.current) return
    wasLoading.current = false
    setSettled(true)
    const timer = setTimeout(() => setSettled(false), SETTLED_HOLD_MS)
    return () => clearTimeout(timer)
  }, [loading])

  return (
    <>
      <Button
        variant="outline"
        size="sm"
        onClick={onRefresh}
        disabled={loading}
        aria-busy={loading}
        className="rounded-lg"
      >
        {settled && !loading ? (
          <Check
            className="refresh-check size-4 text-emerald-600"
            aria-hidden="true"
          />
        ) : (
          <RefreshCw
            className={`size-4 ${loading ? 'refresh-sweep' : ''}`}
            aria-hidden="true"
          />
        )}
        Refresh
      </Button>

      {/* The tick is decorative. This is what actually reports the outcome,
          and it has to be polite so it never interrupts what it reads next. */}
      <span className="sr-only" role="status" aria-live="polite">
        {loading ? 'Refreshing weather' : settled ? 'Weather updated' : ''}
      </span>
    </>
  )
}

function TopBar({
  loading,
  cityCount,
  onRefresh,
}: {
  loading: boolean
  cityCount: number
  onRefresh: () => void
}) {
  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/85 backdrop-blur-md">
      <div className="mx-auto flex w-full max-w-7xl items-center gap-4 px-5 py-3">
        <div className="flex items-center gap-2.5">
          <CloudSun className="size-5 text-primary" aria-hidden="true" />
          {/* The wordmark is the page's only h1. */}
          <h1 className="text-base font-medium tracking-tight">
            Weather Watcher
          </h1>
        </div>

        {/* Deliberately says nothing when the list is empty: the empty state
            below already says so, and duplicating it makes both harder to
            target for assistive tech. */}
        {cityCount > 0 ? (
          <p className="hidden text-sm text-muted-foreground sm:block">
            {cityCount} {cityCount === 1 ? 'city' : 'cities'} tracked
          </p>
        ) : null}

        <div className="ml-auto flex items-center gap-3">
          <span
            className="hidden font-mono text-xs text-muted-foreground lg:inline"
            title="Backend endpoint"
          >
            {API_BASE_URL.replace(/^https?:\/\//, '')}
          </span>

          <RefreshButton loading={loading} onRefresh={onRefresh} />
        </div>
      </div>
    </header>
  )
}
