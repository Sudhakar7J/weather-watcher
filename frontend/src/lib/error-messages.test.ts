import { describe, expect, it } from 'vitest'

import { describeError } from './error-messages'
import { WeatherApiError } from '@/lib/api/client'

/**
 * Copy is part of the interface, so the rules that govern it are asserted:
 * never blank, never raw, and retry offered only where it can help.
 */

describe('describeError', () => {
  const kinds = [
    'offline',
    'unreachable',
    'timeout',
    'cancelled',
    'bad-request',
    'server',
    'malformed',
    'unknown',
  ] as const

  it('always produces a title and a message', () => {
    for (const kind of kinds) {
      const presentation = describeError(new WeatherApiError('x', { kind }))

      expect(presentation.title.trim(), `title for ${kind}`).not.toBe('')
      expect(presentation.message.trim(), `message for ${kind}`).not.toBe('')
    }
  })

  it('offers a retry only where retrying could help', () => {
    // A rejected request needs a change rather than a repeat, and a cancelled
    // one was already superseded by a newer request.
    const notRetryable = new Set(['bad-request', 'cancelled'])

    for (const kind of kinds) {
      const presentation = describeError(new WeatherApiError('x', { kind }))
      expect(presentation.retryable, `retryable for ${kind}`).toBe(
        !notRetryable.has(kind),
      )
    }
  })

  it('shows the server detail for a 4xx because it is written for a person', () => {
    const presentation = describeError(
      new WeatherApiError('ignored', {
        kind: 'bad-request',
        detail: 'at most 20 cities may be requested',
      }),
    )

    expect(presentation.message).toBe('at most 20 cities may be requested')
  })

  it('does not leak the internal message of a server error', () => {
    const presentation = describeError(
      new WeatherApiError('panic: database dsn postgres://...', { kind: 'server' }),
    )

    expect(presentation.message).not.toContain('dsn')
    expect(presentation.message).not.toContain('panic')
  })

  it('gives distinct guidance per kind', () => {
    const offline = describeError(new WeatherApiError('x', { kind: 'offline' }))
    const timeout = describeError(new WeatherApiError('x', { kind: 'timeout' }))

    expect(offline.message).not.toBe(timeout.message)
    expect(offline.message).toMatch(/reconnect/i)
    expect(timeout.message).toMatch(/time/i)
  })

  it('handles values that are not errors at all', () => {
    for (const input of [null, undefined, 'boom', 42, {}]) {
      const presentation = describeError(input)
      expect(presentation.title).not.toBe('')
      expect(presentation.message).not.toBe('')
    }
  })
})
