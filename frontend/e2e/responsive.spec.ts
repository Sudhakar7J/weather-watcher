import { expect, test } from '@playwright/test'

/**
 * Layout checks across viewports.
 *
 * The dashboard is a grid, so the risks are horizontal overflow at narrow
 * widths and controls colliding on small screens.
 */

test('lays out cards in a grid that reflows on small screens', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem(
      'weather-watcher.tracked-cities',
      JSON.stringify(['Singapore', 'Tokyo', 'London']),
    )
  })

  await page.goto('/')
  await page.locator('article[data-city]').first().waitFor()

  const cards = page.locator('article[data-city]')
  const first = await cards.nth(0).boundingBox()
  const second = await cards.nth(1).boundingBox()

  expect(first).not.toBeNull()
  expect(second).not.toBeNull()
  if (first === null || second === null) return

  // On a wide viewport the first two cards sit side by side.
  const viewportWidth = page.viewportSize()?.width ?? 0
  if (viewportWidth >= 640) {
    expect(second.y).toBeCloseTo(first.y, 0)
  }
})

test('does not scroll horizontally at any tested width', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem(
      'weather-watcher.tracked-cities',
      JSON.stringify(['Singapore', 'Tokyo', 'London', 'Reykjavik', 'Oslo']),
    )
  })

  for (const width of [360, 390, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 900 })
    await page.goto('/')
    await page.locator('article[data-city]').first().waitFor()

    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    )

    expect(overflow, `horizontal overflow at ${width}px`).toBe(false)
  }
})

test('cards keep a readable minimum height', async ({ page }) => {
  await page.goto('/')
  await page.locator('article[data-city]').first().waitFor()

  const box = await page.locator('article[data-city]').first().boundingBox()
  expect(box).not.toBeNull()
  if (box !== null) {
    expect(box.height).toBeGreaterThan(200)
  }
})

test('the chart stays within the viewport on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')

  const chart = page.getByRole('region', { name: /24 hour temperature trend/i })
  await expect(chart).toBeVisible()

  const box = await chart.boundingBox()
  const viewportWidth = page.viewportSize()?.width ?? 0

  expect(box).not.toBeNull()
  if (box !== null) {
    expect(box.width).toBeLessThanOrEqual(viewportWidth)
  }
})
