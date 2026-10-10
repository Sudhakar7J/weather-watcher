import { ZodError } from 'zod'

import {
  healthResponseSchema,
  weatherResponseSchema,
  type HealthResponse,
  type WeatherResponse,
} from './schemas'

/**
 * Base URL of the Go backend.
 *
 * Read from VITE_API_BASE_URL so one build can target a local backend or a
 * deployed one. Vite inlines this at build time, so it is configuration, never
 * a secret.
 */
export const API_BASE_URL = (
  import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'
).replace(/\/+$/, '')

/** How long to wait for the backend before giving up. */
export const REQUEST_TIMEOUT_MS = 15_000

/**
 * Every failure the client can produce, classified.
 *
 * The UI renders from the kind rather than inspecting error messages, so a
 * thrown TypeError from a browser fetch cannot leak into the interface as raw
 * developer text. Each kind maps to one clear explanation and one recovery
 * action.
 */
export type WeatherErrorKind =
  | 'offline'
  | 'unreachable'
  | 'timeout'
  | 'cancelled'
  | 'bad-request'
  | 'server'
  | 'malformed'
  | 'unknown'

export interface WeatherApiErrorOptions {
  kind: WeatherErrorKind
  status?: number | undefined
  cause?: unknown
  /** Server-supplied detail, used when it is safe and specific. */
  detail?: string | undefined
}

/** A classified API failure. */
export class WeatherApiError extends Error {
  readonly kind: WeatherErrorKind
  readonly status: number | null
  readonly detail: string | undefined

  constructor(message: string, options: WeatherApiErrorOptions) {
    super(message, { cause: options.cause })
    this.name = 'WeatherApiError'
    this.kind = options.kind
    this.status = options.status ?? null
    this.detail = options.detail
  }

  /** Whether retrying the same request could plausibly succeed. */
  get retryable(): boolean {
    return this.kind !== 'bad-request'
  }
}

/** Turns any thrown value into a classified error. Never throws. */
export function classifyError(cause: unknown): WeatherApiError {
  if (cause instanceof WeatherApiError) return cause

  if (isAbortError(cause)) {
    return new WeatherApiError('The request was cancelled.', {
      kind: 'cancelled',
      cause,
    })
  }

  if (cause instanceof TypeError) {
    // fetch rejects with a TypeError for DNS failure, connection refused and
    // CORS rejection alike.
    return new WeatherApiError(
      navigator.onLine
        ? `Could not reach the weather service at ${API_BASE_URL}.`
        : 'You appear to be offline.',
      { kind: navigator.onLine ? 'unreachable' : 'offline', cause },
    )
  }

  if (cause instanceof ZodError) {
    // The shape did not match what the API is supposed to return. Surfacing the
    // Zod paths would put implementation detail in front of the user.
    return new WeatherApiError(
      'The weather service returned data in an unexpected format.',
      { kind: 'malformed', cause },
    )
  }

  return new WeatherApiError(
    cause instanceof Error ? cause.message : 'An unexpected error occurred.',
    { kind: 'unknown', cause },
  )
}

/**
 * Fetches weather for the given cities.
 *
 * Aborts are rethrown classified as `cancelled` so the caller can tell a
 * superseded request from a real failure.
 */
export async function fetchWeather(
  cities: readonly string[],
  { signal }: { signal?: AbortSignal } = {},
): Promise<WeatherResponse> {
  if (cities.length === 0) {
    throw new WeatherApiError('At least one city is required.', { kind: 'bad-request' })
  }

  const url = `${API_BASE_URL}/api/weather?cities=${encodeURIComponent(cities.join(','))}`
  const payload = await requestJson(url, signal)

  // parse() throws a raw ZodError, which would bypass the taxonomy and surface
  // as an unknown error. Route it back through classification.
  try {
    return weatherResponseSchema.parse(payload)
  } catch (cause) {
    throw classifyError(cause)
  }
}

/** Fetches the backend health payload. */
export async function fetchHealth({
  signal,
}: { signal?: AbortSignal } = {}): Promise<HealthResponse> {
  const payload = await requestJson(`${API_BASE_URL}/api/health`, signal)

  try {
    return healthResponseSchema.parse(payload)
  } catch (cause) {
    throw classifyError(cause)
  }
}

async function requestJson(
  url: string,
  signal?: AbortSignal,
): Promise<unknown> {
  // A client-side deadline is separate from the backend's own: if the network
  // drops in a way that hangs rather than fails, the request would otherwise
  // stay pending indefinitely.
  const deadline = AbortSignal.timeout(REQUEST_TIMEOUT_MS)
  const composed =
    signal === undefined ? deadline : AbortSignal.any([signal, deadline])

  let response: Response

  try {
    response = await fetch(url, {
      signal: composed,
      headers: { Accept: 'application/json' },
    })
  } catch (cause) {
    // Distinguish our deadline from the caller's cancellation.
    if (deadline.aborted && signal?.aborted !== true) {
      throw new WeatherApiError(
        `The weather service did not respond within ${REQUEST_TIMEOUT_MS / 1000} seconds.`,
        { kind: 'timeout', cause },
      )
    }
    throw classifyError(cause)
  }

  if (!response.ok) {
    throw new WeatherApiError(defaultMessageFor(response.status), {
      kind: response.status >= 500 ? 'server' : 'bad-request',
      status: response.status,
      detail: await readErrorMessage(response),
    })
  }

  try {
    return await response.json()
  } catch (cause) {
    throw new WeatherApiError(
      'The weather service returned a response that could not be read as JSON.',
      { kind: 'malformed', status: response.status, cause },
    )
  }
}

/**
 * Reads the backend's own error text where present.
 *
 * The API returns {"error": "..."} for request-level failures. It is used only
 * for 4xx, where the message is specific and written for a person; a 5xx body
 * could contain anything and is deliberately ignored.
 */
async function readErrorMessage(response: Response): Promise<string | undefined> {
  if (response.status >= 500) return undefined

  try {
    const body: unknown = await response.json()
    if (
      typeof body === 'object' &&
      body !== null &&
      'error' in body &&
      typeof body.error === 'string' &&
      body.error.trim() !== ''
    ) {
      return body.error
    }
  } catch {
    // Not JSON; fall back to the status-based message.
  }

  return undefined
}

function defaultMessageFor(status: number): string {
  switch (status) {
    case 400:
      return 'The weather service rejected that request.'
    case 404:
      return 'The weather service could not find that endpoint.'
    case 429:
      return 'Too many requests. Wait a moment and try again.'
    case 503:
      return 'The weather service is starting up or unavailable.'
    default:
      return status >= 500
        ? 'The weather service is having trouble right now.'
        : `The weather service responded with an error (HTTP ${status}).`
  }
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}
