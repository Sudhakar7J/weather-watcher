import { X } from 'lucide-react'

import { Button } from '@/components/ui/button'
import type { CityError, Report } from '@/lib/api/schemas'
import { cn } from '@/lib/utils'
import {
  conditionKind,
  conditionStyle,
  formatTemperature,
} from '@/lib/weather-conditions'

import { CityErrorNotice } from './state-display'

interface MetricProps {
  label: string
  value: string
}

function Metric({ label, value }: MetricProps) {
  return (
    <div className="min-w-0">
      <p className="text-[0.6875rem] tracking-wide text-[var(--on-tint-muted)] uppercase">
        {label}
      </p>
      <p className="tabular mt-0.5 truncate text-base font-medium">{value}</p>
    </div>
  )
}

export interface CityCardProps {
  city: string
  report?: Report | undefined
  error?: CityError | undefined
  isSelected?: boolean
  onSelect?: (city: string) => void
  onRemove?: (city: string) => void
}

/**
 * A single tracked city, laid out as a conditions-tinted tile.
 *
 * The reference here is the category benchmark: one dominant temperature
 * reading, a hairline dividing it from a short row of supporting figures, and
 * colour doing the work of telling you the conditions before you read
 * anything. Adding a city means adding a list entry, not writing markup.
 *
 * High and low are derived from the 24 hourly points already fetched, so the
 * card needs no extra request.
 */
export default function CityCard({
  city,
  report,
  error,
  isSelected = false,
  onSelect,
  onRemove,
}: CityCardProps) {
  if (error !== undefined && report === undefined) {
    return (
      <article
        data-city={city}
        className="flex min-h-56 flex-col rounded-2xl border border-border bg-card p-5"
      >
        <header className="flex items-start justify-between gap-2">
          <h3 className="truncate text-lg font-medium">{city}</h3>
          <RemoveButton city={city} onRemove={onRemove} />
        </header>

        <div className="mt-4 flex flex-1 items-center">
          <CityErrorNotice message={error.message} />
        </div>
      </article>
    )
  }

  // A city can be tracked with no report and no error, which happens when the
  // geocoder resolves it under a different name than was typed ("asd" comes
  // back as a real place elsewhere). Returning null here left a silent gap in
  // the grid, so say so instead.
  if (report === undefined) {
    return (
      <article
        className="rounded-2xl border border-dashed border-border bg-card/40 p-5"
        aria-label={`${city}, no data`}
      >
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="truncate text-base font-medium">{city}</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              No matching location
            </p>
          </div>
          {onRemove !== undefined ? (
            <button
              type="button"
              onClick={() => onRemove(city)}
              aria-label={`Remove ${city}`}
              className="cursor-pointer rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <X className="size-4" aria-hidden="true" />
            </button>
          ) : null}
        </div>
        <p className="mt-4 text-sm text-muted-foreground">
          Check the spelling, or try adding the city again.
        </p>
      </article>
    )
  }

  const kind = conditionKind(report.current.code)
  const { icon: ConditionIcon, label } = conditionStyle(report.current.code)

  const temperatures = report.hourly.map((point) => point.temperature_c)
  const high = temperatures.length > 0 ? Math.max(...temperatures) : null
  const low = temperatures.length > 0 ? Math.min(...temperatures) : null

  return (
    <article
      data-city={city}
      data-condition={kind}
      className={cn(
        'card-surface group relative flex flex-col overflow-hidden rounded-2xl',
        'text-[var(--on-tint)] transition-shadow duration-200',
        isSelected
          ? 'ring-2 ring-[var(--card-ring)] ring-offset-2 ring-offset-background'
          : 'hover:shadow-2xl hover:shadow-black/15',
      )}
    >
      <div className="flex flex-1 flex-col p-5">
        <header className="flex items-start justify-between gap-2">
          <h3 className="truncate text-xl font-medium leading-tight">
            {report.city}
          </h3>

          {/* Revealed on hover for pointer devices; always visible on touch and
              whenever a keyboard user reaches it. */}
          <RemoveButton city={city} onRemove={onRemove} deferred />
        </header>

        <button
          type="button"
          onClick={() => onSelect?.(report.city)}
          aria-pressed={isSelected}
          aria-label={`Show the 24 hour trend for ${report.city}`}
          className="flex flex-1 flex-col rounded-lg text-left"
        >
          <div className="mt-5 flex items-end justify-between gap-3">
            <p className="flex items-start">
              <span className="tabular text-[5.5rem] leading-[0.8] font-light tracking-[-0.04em]">
                {formatTemperature(report.current.temperature_c)}
              </span>
              <span className="mt-1 text-3xl text-[var(--on-tint-muted)]">
                &deg;
              </span>
            </p>

            <div className="flex flex-col items-end gap-1.5 pb-1">
              {high !== null && low !== null ? (
                <p className="tabular text-sm font-medium">
                  H:{formatTemperature(high)}&deg; L:
                  {formatTemperature(low)}&deg;
                </p>
              ) : null}
              <ConditionIcon
                className="size-9 text-[var(--on-tint)] opacity-90"
                strokeWidth={1.5}
                aria-hidden="true"
              />
            </div>
          </div>

          <p className="mt-3 text-lg">{label}</p>

          {/* Hairline separates the headline reading from supporting figures. */}
          <dl className="mt-5 grid grid-cols-3 gap-3 border-t border-white/20 pt-4">
            <Metric
              label="Feels"
              value={`${formatTemperature(report.current.feels_like_c)}\u00b0`}
            />
            <Metric
              label="Humidity"
              value={`${Math.round(report.current.humidity_pct)}%`}
            />
            <Metric
              label="Wind"
              value={`${Math.round(report.current.wind_speed_kph)} km/h`}
            />
          </dl>

          {/* The affordance is repeated on every card, so it stays out of the
              way until the card is engaged. Always visible on touch, where
              there is no hover. */}
          <p
            className={cn(
              'mt-4 text-xs text-[var(--on-tint-faint)] transition-opacity',
              'opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100',
              isSelected && 'sm:opacity-100',
            )}
          >
            {isSelected
              ? 'Showing the 24 hour trend below'
              : 'Tap for the 24 hour trend'}
          </p>
        </button>
      </div>
    </article>
  )
}

function RemoveButton({
  city,
  onRemove,
  deferred = false,
}: {
  city: string
  onRemove?: ((city: string) => void) | undefined
  deferred?: boolean
}) {
  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={() => onRemove?.(city)}
      aria-label={`Stop tracking ${city}`}
      className={cn(
        'size-8 shrink-0 rounded-full',
        deferred
          ? 'text-[var(--on-tint)] hover:bg-white/15'
          : 'text-muted-foreground hover:bg-muted',
        deferred &&
          'opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100',
      )}
    >
      <X className="size-4" aria-hidden="true" />
    </Button>
  )
}
