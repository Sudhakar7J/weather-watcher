import { afterEach, describe, expect, it, vi } from 'vitest'
import { ZodError } from 'zod'

import {
  WeatherApiError,
  classifyError,
  fetchWeather,
} from './client'

/**
 * The error taxonomy is what stops raw browser errors reaching the interface,
 * so it is tested directly rather than only through the UI.
 */

afterEach(() => {
  vi.restoreAllMocks()
})

describe('classifyError', () => {
  it('passes an already-classified error through unchanged', () => {
    const original = new WeatherApiError('nope', { kind: 'server' })
    expect(classifyError(original)).toBe(original)
  })

  it('classifies a TypeError from fetch as a connection failure', () => {
    // fetch rejects with TypeError for DNS failure, refused connections and
    // CORS rejection alike.
    const classified = classifyError(new TypeError('Failed to fetch'))
    expect(classified).toBeInstanceOf(WeatherApiError)
    expect(['unreachable', 'offline']).toContain(classified.kind)
  })

  it('reports offline distinctly when the browser says it is offline', () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false)
    expect(classifyError(new TypeError('Failed to fetch')).kind).toBe('offline')
  })

  it('classifies an AbortError as cancelled', () => {
    const abort = new DOMException('aborted', 'AbortError')
    expect(classifyError(abort).kind).toBe('cancelled')
  })

  it('hides Zod internals behind a malformed-response error', () => {
    const zodError = new ZodError([
      {
        code: 'custom',
        path: ['results', 0, 'current', 'temperature_c'],
        message: 'expected number, received string',
      },
    ])

    const classified = classifyError(zodError)
    expect(classified.kind).toBe('malformed')
    // The Zod path must not appear in anything the user could see.
    expect(classified.message).not.toContain('temperature_c')
    expect(classified.message).not.toContain('Zod')
  })

  it('classifies an arbitrary thrown value as unknown', () => {
    expect(classifyError('a string').kind).toBe('unknown')
    expect(classifyError({ weird: true }).kind).toBe('unknown')
    expect(classifyError(undefined).kind).toBe('unknown')
  })

  it('never throws, whatever it is given', () => {
    const inputs: unknown[] = [
      null,
      undefined,
      0,
      '',
      Symbol('s'),
      new Error('boom'),
      { nested: { deeper: true } },
    ]

    for (const input of inputs) {
      expect(() => classifyError(input)).not.toThrow()
      expect(classifyError(input)).toBeInstanceOf(WeatherApiError)
    }
  })
})

describe('fetchWeather', () => {
  const validPayload = {
    results: [
      {
        city: 'Singapore',
        country: 'Singapore',
        latitude: 1.2897,
        longitude: 103.85,
        current: {
          temperature_c: 31.2,
          feels_like_c: 36.7,
          humidity_pct: 64,
          wind_speed_kph: 12.2,
          code: 3,
          description: 'Overcast',
          observed_at: '2026-10-10T04:45:00Z',
        },
        hourly: [{ time: '2026-10-10T04:00:00Z', temperature_c: 31 }],
        fetched_at: '2026-10-10T04:45:00Z',
        from_cache: false,
      },
    ],
    errors: [],
    requested_at: '2026-10-10T04:45:00Z',
    cached_count: 0,
    total: 1,
  }

  it('rejects an empty city list without making a request', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch')

    await expect(fetchWeather([])).rejects.toMatchObject({ kind: 'bad-request' })
    expect(fetchSpy).not.toHaveBeenCalled()
  })

  it('parses a well-formed response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(validPayload), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const result = await fetchWeather(['Singapore'])
    expect(result.results[0]?.city).toBe('Singapore')
  })

  it('treats a response that does not match the schema as malformed', async () => {
    // Valid JSON, wrong shape: the case a plain cast would let through.
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ results: 'not-an-array' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    await expect(fetchWeather(['Singapore'])).rejects.toMatchObject({
      kind: 'malformed',
    })
  })

  it('treats a non-JSON body as malformed', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('<html>gateway</html>', { status: 200 }),
    )

    await expect(fetchWeather(['Singapore'])).rejects.toMatchObject({
      kind: 'malformed',
    })
  })

  it('classifies 5xx as a server error and ignores the body', async () => {
    // A 500 body could contain anything, so its detail must not be surfaced.
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'panic: secret' }), { status: 500 }),
    )

    await expect(fetchWeather(['Singapore'])).rejects.toMatchObject({
      kind: 'server',
      detail: undefined,
    })
  })

  it('keeps a 4xx body detail because it is written for a person', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'too many cities: at most 20' }), {
        status: 400,
      }),
    )

    await expect(fetchWeather(['Singapore'])).rejects.toMatchObject({
      kind: 'bad-request',
      detail: 'too many cities: at most 20',
    })
  })

  it('marks a 4xx as not retryable', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'bad' }), { status: 400 }),
    )

    const error = await fetchWeather(['Singapore']).catch((e: unknown) => e)
    expect(error).toBeInstanceOf(WeatherApiError)
    expect((error as WeatherApiError).retryable).toBe(false)
  })

  it('url-encodes city names containing spaces', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(validPayload), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    await fetchWeather(['New York'])

    const requested = String(fetchSpy.mock.calls[0]?.[0])
    expect(requested).toContain('New%20York')
  })
})
