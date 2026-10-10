import { WeatherApiError, classifyError } from '@/lib/api/client'

/**
 * How an error should be presented to the person using the dashboard.
 *
 * All user-facing error copy lives here rather than being written inline at
 * each call site, so the same failure always reads the same way no matter
 * which screen hit it.
 *
 * Two rules hold throughout:
 *
 * - Say what happened and what to do next. Never apologise, never be vague.
 * - Never show a raw error string. Internal detail belongs in the console.
 */
export interface ErrorPresentation {
  /** Short headline for the state. */
  title: string
  /** One sentence explaining the cause and the way out. */
  message: string
  /** Whether offering a retry button makes sense. */
  retryable: boolean
  /** Extra context shown behind a disclosure, for support conversations. */
  detail?: string
}

/**
 * Turns any thrown value into something renderable.
 *
 * Never throws and never returns an empty message, so a caller can render the
 * result unconditionally.
 */
export function describeError(error: unknown): ErrorPresentation {
  const classified = classifyError(error)

  if (classified instanceof WeatherApiError) {
    return fromWeatherApiError(classified)
  }

  return {
    title: 'Something went wrong',
    message: 'An unexpected error stopped the dashboard from updating.',
    retryable: true,
  }
}

function fromWeatherApiError(error: WeatherApiError): ErrorPresentation {
  switch (error.kind) {
    case 'offline':
      return {
        title: 'You are offline',
        message: 'Reconnect to the internet, then try loading again.',
        retryable: true,
      }

    case 'unreachable':
      return {
        title: 'Cannot reach the weather service',
        message:
          'The request did not get a response. The backend may be stopped, or the configured address may be wrong.',
        retryable: true,
        detail: error.message,
      }

    case 'timeout':
      return {
        title: 'The weather service took too long',
        message: 'It did not respond in time. This is usually temporary.',
        retryable: true,
      }

    case 'cancelled':
      return {
        title: 'Request cancelled',
        message: 'The request was replaced by a newer one.',
        retryable: false,
      }

    case 'bad-request':
      // A 4xx body is written for a person, so it is safe and specific enough
      // to show as-is.
      return {
        title: 'That request was rejected',
        message:
          error.detail ?? 'The weather service could not accept the request.',
        retryable: error.retryable,
      }

    case 'server':
      return {
        title: 'The weather service is having trouble',
        message:
          'Something failed on the backend while fetching weather. Try again in a moment.',
        retryable: true,
      }

    case 'malformed':
      return {
        title: 'Unexpected response',
        message:
          'The weather service replied with data in a format this dashboard does not recognise. It may be running a newer version.',
        retryable: true,
      }

    default:
      return {
        title: 'Something went wrong',
        message: 'The dashboard hit an unexpected problem while updating.',
        retryable: true,
      }
  }
}
