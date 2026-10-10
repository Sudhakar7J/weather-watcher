import type { ReactNode } from 'react'
import { AlertCircle, CloudOff, Inbox, Loader2, WifiOff } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { WeatherApiError } from '@/lib/api/client'
import { describeError } from '@/lib/error-messages'
import { cn } from '@/lib/utils'

/**
 * Shared presentation for every explicit state the dashboard can be in.
 *
 * Used at both the whole-dashboard level (nothing tracked, the request failed)
 * and the per-card level (one city failed), so the wording and treatment stay
 * consistent wherever an error appears.
 */

export function LoadingState({
  label = 'Loading current conditions',
}: {
  label?: string
}) {
  return (
    <div
      className="flex flex-col items-center gap-3 py-24 text-muted-foreground"
      role="status"
      aria-live="polite"
    >
      <Loader2
        className="size-5 animate-spin motion-reduce:animate-none"
        aria-hidden="true"
      />
      <p className="text-sm">{label}</p>
    </div>
  )
}

/** Icon chosen from the failure kind, so the cause is recognisable at a glance. */
function iconFor(error: unknown): ReactNode {
  const kind =
    error instanceof WeatherApiError ? error.kind : ('unknown' as const)

  switch (kind) {
    case 'offline':
      return <WifiOff className="size-4" aria-hidden="true" />
    case 'unreachable':
      return <CloudOff className="size-4" aria-hidden="true" />
    default:
      return <AlertCircle className="size-4" aria-hidden="true" />
  }
}

export interface ErrorStateProps {
  /** The thrown value. Presentation is derived, never written by the caller. */
  error: unknown
  onRetry?: () => void
  /** Tighter treatment for use inside a city card rather than the page. */
  compact?: boolean
  /** Overrides the derived title, for per-city failures that have their own. */
  title?: string
}

/**
 * Renders a failure with a cause, a consequence and a way forward.
 *
 * Raw error detail is written to the console rather than rendered, so an
 * internal message can never end up in front of a user.
 */
export function ErrorState({
  error,
  onRetry,
  compact = false,
  title,
}: ErrorStateProps) {
  const presentation = describeError(error)

  if (error instanceof WeatherApiError && error.detail !== undefined) {
    console.warn('Weather request failed:', error.kind, error.detail)
  }

  return (
    <div
      className={cn(
        'rounded-xl border border-border bg-muted/50',
        compact ? 'p-4' : 'p-6',
      )}
      role="alert"
    >
      <div className="flex items-start gap-3">
        <span
          className="mt-0.5 shrink-0 text-muted-foreground"
          aria-hidden="true"
        >
          {iconFor(error)}
        </span>

        <div className="min-w-0 flex-1">
          <p className="font-medium">{title ?? presentation.title}</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {presentation.message}
          </p>

          {onRetry !== undefined && presentation.retryable ? (
            <Button
              variant="outline"
              size="sm"
              className="mt-4"
              onClick={onRetry}
            >
              Try again
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  )
}

/**
 * Failure for one city inside the grid.
 *
 * Distinct from {@link ErrorState}: the backend already wrote these messages
 * for a person ("unknown city"), so they are shown as given rather than being
 * run back through the error taxonomy.
 */
export function CityErrorNotice({ message }: { message: string }) {
  return (
    <div
      className="rounded-xl border border-border bg-muted/50 p-4"
      role="alert"
    >
      <p className="text-sm font-medium">Unavailable</p>
      <p className="mt-1 text-sm text-muted-foreground">{message}</p>
    </div>
  )
}

export function EmptyState({
  onRestoreDefaults,
}: {
  onRestoreDefaults: () => void
}) {
  return (
    <div className="rounded-2xl border border-dashed border-border py-24 text-center">
      <Inbox
        className="mx-auto size-6 text-muted-foreground"
        strokeWidth={1.5}
        aria-hidden="true"
      />
      <p className="mt-4 text-lg font-medium">No cities tracked</p>
      <p className="mx-auto mt-1.5 max-w-sm text-sm text-muted-foreground">
        Add a city above to start tracking its current conditions.
      </p>
      <Button
        variant="outline"
        size="sm"
        className="mt-6"
        onClick={onRestoreDefaults}
      >
        Restore the default cities
      </Button>
    </div>
  )
}

/** Placeholder card shown while initial data loads. */
export function CityCardSkeleton() {
  return (
    <div
      className="min-h-56 animate-pulse rounded-2xl border border-border bg-muted/60 p-5 motion-reduce:animate-none"
      aria-hidden="true"
    >
      <div className="h-5 w-28 rounded bg-muted" />
      <div className="mt-8 h-16 w-36 rounded bg-muted" />
      <div className="mt-5 h-5 w-24 rounded bg-muted" />
      <div className="mt-6 grid grid-cols-3 gap-3 border-t border-border pt-4">
        <div className="h-8 rounded bg-muted" />
        <div className="h-8 rounded bg-muted" />
        <div className="h-8 rounded bg-muted" />
      </div>
    </div>
  )
}
