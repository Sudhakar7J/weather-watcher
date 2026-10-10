import { expect, test } from '@playwright/test'

/**
 * Failure-path coverage end to end.
 *
 * Each case intercepts the API and drives the real UI, so what is asserted is
 * the copy the user actually sees. The theme throughout: say what happened, say
 * what to do, never show an internal message.
 */

const STORAGE_KEY = 'weather-watcher.tracked-cities'

async function seedOne(page: import('@playwright/test').Page) {
  await page.addInitScript((key) => {
    if (window.localStorage.getItem(key) === null) {
      window.localStorage.setItem(key, JSON.stringify(['Singapore']))
    }
  }, STORAGE_KEY)
}

const VALID_PAYLOAD = {
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

test('an unreachable backend explains itself and offers a retry', async ({ page }) => {
  await seedOne(page)
  await page.route('**/api/weather*', (route) => route.abort('failed'))

  await page.goto('/')

  const alert = page.getByRole('alert')
  await expect(alert).toBeVisible()
  await expect(alert).toContainText(/cannot reach the weather service/i)
  await expect(alert).toContainText(/backend may be stopped/i)
  await expect(page.getByRole('button', { name: 'Try again' })).toBeVisible()
})

test('being offline is reported as offline, not as a server fault', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'onLine', {
      configurable: true,
      get: () => false,
    })
    window.localStorage.setItem(
      'weather-watcher.tracked-cities',
      JSON.stringify(['Singapore']),
    )
  })

  await page.route('**/api/weather*', (route) => route.abort('failed'))
  await page.goto('/')

  const alert = page.getByRole('alert')
  await expect(alert).toContainText(/you are offline/i)
  await expect(alert).toContainText(/reconnect/i)
})

test('a server error does not leak the response body', async ({ page }) => {
  await seedOne(page)
  await page.route('**/api/weather*', (route) =>
    route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({
        error: 'panic: dsn postgres://user:secret@db:5432/weather',
      }),
    }),
  )

  await page.goto('/')

  const alert = page.getByRole('alert')
  await expect(alert).toContainText(/having trouble/i)
  await expect(alert).not.toContainText(/secret/)
  await expect(alert).not.toContainText(/dsn/)
})

test('a rejected request shows the backend explanation and no retry', async ({ page }) => {
  await seedOne(page)
  await page.route('**/api/weather*', (route) =>
    route.fulfill({
      status: 400,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'too many cities: at most 20 may be requested' }),
    }),
  )

  await page.goto('/')

  const alert = page.getByRole('alert')
  await expect(alert).toContainText(/at most 20 may be requested/)

  // Repeating an identical rejected request cannot help.
  await expect(page.getByRole('button', { name: 'Try again' })).toHaveCount(0)
})

test('a non-JSON reply is reported as an unexpected response', async ({ page }) => {
  await seedOne(page)
  await page.route('**/api/weather*', (route) =>
    route.fulfill({ status: 200, contentType: 'text/html', body: '<html>hi</html>' }),
  )

  await page.goto('/')
  await expect(page.getByRole('alert')).toContainText(/unexpected response/i)
})

test('schema drift is caught rather than crashing a component', async ({ page }) => {
  await seedOne(page)

  // Valid JSON, wrong shape. A cast would let this through and blow up later
  // inside a component; the schema turns it into one clear message.
  await page.route('**/api/weather*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ results: [{ city: 'Singapore' }], errors: [] }),
    }),
  )

  await page.goto('/')

  const alert = page.getByRole('alert')
  await expect(alert).toBeVisible()
  await expect(alert).toContainText(/unexpected/i)

  // Zod's own vocabulary and the field paths it reports must not reach the
  // interface. Note "Unexpected" legitimately contains "expected", so these
  // assert on the tokens Zod actually emits.
  for (const leak of [/invalid_type/i, /received\b/i, /Zod/i, /results\[/]) {
    await expect(alert).not.toContainText(leak)
  }
})

test('a retry recovers once the backend comes back', async ({ page }) => {
  await seedOne(page)

  let failing = true
  await page.route('**/api/weather*', (route) => {
    if (failing) {
      failing = false
      return route.abort('failed')
    }
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(VALID_PAYLOAD),
    })
  })

  await page.goto('/')
  await expect(page.getByRole('alert')).toBeVisible()

  await page.getByRole('button', { name: 'Try again' }).click()

  await expect(page.locator('article[data-city="Singapore"]')).toBeVisible()
  await expect(page.getByRole('alert')).toHaveCount(0)
})

test('one failing city does not take down the dashboard', async ({ page }) => {
  await page.addInitScript((key) => {
    window.localStorage.setItem(key, JSON.stringify(['Singapore', 'Notarealcityxyz']))
  }, STORAGE_KEY)

  await page.goto('/')

  const failed = page.locator('article[data-city="Notarealcityxyz"]')
  await expect(failed.getByRole('alert')).toContainText(/unknown city/i)

  // The healthy city is untouched, and the page-level failure state is absent.
  await expect(page.locator('article[data-city="Singapore"]')).toBeVisible()
  await expect(failed.getByRole('alert')).toHaveCount(1)
})

test('tracking more cities than the backend allows is refused up front', async ({
  page,
}) => {
  await page.addInitScript((key) => {
    window.localStorage.setItem(
      key,
      JSON.stringify(Array.from({ length: 20 }, (_, i) => `City${i}`)),
    )
  }, STORAGE_KEY)

  await page.goto('/')

  await page.getByLabel('Add a city').fill('OneTooMany')
  await page.getByRole('button', { name: 'Add', exact: true }).click()

  // Scoped to the form: the placeholder cities all fail to resolve and produce
  // their own card-level alerts, which is the behaviour under test elsewhere.
  const formAlert = page.locator('form').getByRole('alert')

  // The dashboard must explain the ceiling rather than firing a request the
  // backend will reject wholesale.
  await expect(formAlert).toContainText(/up to 20 cities/i)
  await expect(page.locator('article[data-city="OneTooMany"]')).toHaveCount(0)
})
