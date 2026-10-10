import { Component, type ErrorInfo, type ReactNode } from 'react'

import { Button } from '@/components/ui/button'

interface ErrorBoundaryProps {
  children: ReactNode
}

interface ErrorBoundaryState {
  error: Error | null
}

/**
 * Last line of defence for render-time failures.
 *
 * Without this, any exception during render unmounts the whole tree and the
 * user gets a blank white page with no idea what happened. Here they get a
 * panel in the product's own visual language, plus the technical detail kept
 * behind a disclosure rather than splashed across the screen.
 *
 * This is a class component because React exposes no hook equivalent of
 * componentDidCatch.
 */
export class ErrorBoundary extends Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  override state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    // Wire this to a reporting service in production. The console keeps the
    // componentDidCatch contract meaningful until then.
    console.error('Unhandled render error', error, info.componentStack)
  }

  private readonly handleRetry = () => {
    this.setState({ error: null })
  }

  override render() {
    const { error } = this.state
    if (error === null) return this.props.children

    return (
      <div className="flex min-h-dvh items-center justify-center px-5">
        <div className="w-full max-w-md rounded-2xl border border-border bg-card p-6">
          <p className="text-lg font-medium">The dashboard stopped unexpectedly</p>

          <p className="mt-2 text-sm text-muted-foreground">
            Something failed while rendering. Reloading usually clears it.
          </p>

          <div className="mt-5 flex gap-2">
            <Button variant="outline" size="sm" onClick={this.handleRetry}>
              Try again
            </Button>
            <Button size="sm" onClick={() => window.location.reload()}>
              Reload the page
            </Button>
          </div>

          {/* Technical detail is available but not presented as the headline,
              so it helps a support conversation without alarming anyone. */}
          <details className="mt-6 border-t border-border pt-4">
            <summary className="cursor-pointer text-sm text-muted-foreground">
              Technical detail
            </summary>
            <pre className="mt-2 overflow-x-auto text-xs whitespace-pre-wrap text-muted-foreground">
              {error.message}
            </pre>
          </details>
        </div>
      </div>
    )
  }
}
