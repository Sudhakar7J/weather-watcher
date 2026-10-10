import { Suspense, lazy, useCallback, useMemo, useState } from 'react'
import { CloudSun, RefreshCw } from 'lucide-react'

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
              {isInitialLoad(loading, reports.length, cityErrors.length)
                ? cities.map((city) => <CityCardSkeleton key={city} />)
                : cities.map((city) => (
                    <CityCard
                      key={city}
                      city={city}
                      report={reportsByCity.get(city)}
                      error={errorsByCity.get(city)}
                      isSelected={city === activeCity}
                      onSelect={setSelectedCity}
                      onRemove={handleRemove}
                    />
                  ))}
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

/**
 * Whether the grid should show skeletons: a request is in flight and nothing
 * has arrived yet. Distinguishing this from a background refresh is what
 * stops the grid flashing on every add.
 */
function isInitialLoad(
  loading: boolean,
  reportCount: number,
  errorCount: number,
): boolean {
  return loading && reportCount === 0 && errorCount === 0
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

          <Button
            variant="outline"
            size="sm"
            onClick={onRefresh}
            disabled={loading}
            aria-busy={loading}
            className="rounded-lg"
          >
            <RefreshCw
              className={`size-4 ${loading ? 'animate-spin motion-reduce:animate-none' : ''}`}
              aria-hidden="true"
            />
            Refresh
          </Button>
        </div>
      </div>
    </header>
  )
}
