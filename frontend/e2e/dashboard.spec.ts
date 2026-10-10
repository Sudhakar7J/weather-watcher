import { expect, test } from '@playwright/test'

/**
 * Behavioural tests for the dashboard, driven against the real backend.
 *
 * These assert what a user would notice: that data arrives, that a bad city
 * fails in isolation, that adding and removing works, and that the explicit
 * states are reachable.
 */

const DEFAULT_CITIES = ['Singapore', 'Tokyo', 'London']

const STORAGE_KEY = 'weather-watcher.tracked-cities'

/**
 * Seeds the tracked list before the app boots.
 *
 * Seeds only when storage is empty, so a reload preserves whatever the test
 * just changed. An unconditional write would run again on every navigation and
 * quietly undo the test's own writes.
 */
async function seedCities(page: import('@playwright/test').Page, cities: string[]) {
  await page.addInitScript(
    ([key, seed]) => {
      if (window.localStorage.getItem(key) === null) {
        window.localStorage.setItem(key, JSON.stringify(seed))
      }
    },
    [STORAGE_KEY, cities] as const,
  )
}

test.beforeEach(async ({ page }) => {
  await seedCities(page, DEFAULT_CITIES)
})

test('loads and renders a card per tracked city with live data', async ({ page }) => {
  await page.goto('/')

  const cards = page.locator('article[data-city]')
  await expect(cards).toHaveCount(DEFAULT_CITIES.length)

  for (const city of DEFAULT_CITIES) {
    await expect(page.locator(`article[data-city="${city}"]`)).toBeVisible()
  }

  // The temperature is the hero element; assert it parsed as a real number
  // rather than a placeholder.
  const temperature = page
    .locator('article[data-city="Singapore"] .tabular')
    .first()
  await expect(temperature).toHaveText(/^-?\d+$/)

  // A resolved card must carry a condition, which drives its tint.
  await expect(page.locator('article[data-city="Singapore"][data-condition]')).toHaveCount(1)
})

test('renders the 24 hour chart for the selected city', async ({ page }) => {
  await page.goto('/')

  // The chart is code-split, so wait for it rather than assuming it is there.
  const chart = page.getByRole('region', { name: /24 hour temperature trend/i })
  await expect(chart).toBeVisible()

  // Selecting another city swaps which trend is shown.
  await page.getByRole('button', { name: /show the 24 hour trend for Tokyo/i }).click()
  await expect(chart).toHaveAttribute('aria-label', /Tokyo/)
})

test('adds a city and persists it', async ({ page }) => {
  await page.goto('/')

  await page.getByLabel('Add a city').fill('Reykjavik')
  await page.getByRole('button', { name: 'Add', exact: true }).click()

  await expect(page.locator('article[data-city="Reykjavik"]')).toBeVisible()

  const stored = await page.evaluate(() =>
    window.localStorage.getItem('weather-watcher.tracked-cities'),
  )
  expect(JSON.parse(stored ?? '[]')).toContain('Reykjavik')
})

test('rejects a duplicate city with a message naming it', async ({ page }) => {
  await page.goto('/')

  await page.getByLabel('Add a city').fill('Singapore')
  await page.getByRole('button', { name: 'Add', exact: true }).click()

  const alert = page.getByRole('alert')
  await expect(alert).toHaveText(/Singapore is already being tracked/)
})

test('removes a city', async ({ page }) => {
  await page.goto('/')

  await expect(page.locator('article[data-city]')).toHaveCount(3)

  await page
    .getByRole('button', { name: 'Stop tracking Tokyo' })
    .click()

  await expect(page.locator('article[data-city="Tokyo"]')).toHaveCount(0)
  await expect(page.locator('article[data-city]')).toHaveCount(2)
})

test('an unknown city fails in isolation without breaking the rest', async ({ page }) => {
  await page.goto('/')

  await page.getByLabel('Add a city').fill('Notarealcityxyz')
  await page.getByRole('button', { name: 'Add', exact: true }).click()

  // The failed city gets its own card-level error.
  const failed = page.locator('article[data-city="Notarealcityxyz"]')
  await expect(failed).toBeVisible()
  await expect(failed.getByRole('alert')).toContainText(/unknown city/i)

  // The cities that did resolve are untouched.
  await expect(page.locator('article[data-city="Singapore"]')).toBeVisible()
  await expect(page.locator('article[data-city="Tokyo"]')).toBeVisible()
  await expect(page.locator('article[data-city="London"]')).toBeVisible()
})

test('shows an empty state when every city is removed', async ({ page }) => {
  await page.goto('/')

  for (const city of DEFAULT_CITIES) {
    await page.getByRole('button', { name: `Stop tracking ${city}` }).click()
  }

  await expect(page.getByText('No cities tracked')).toBeVisible()
})

test('keeps the tracked list across a reload', async ({ page }) => {
  await page.goto('/')

  await page.getByLabel('Add a city').fill('Oslo')
  await page.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(page.locator('article[data-city="Oslo"]')).toBeVisible()

  await page.reload()

  await expect(page.locator('article[data-city="Oslo"]')).toBeVisible()
})
