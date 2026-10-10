import { chromium } from '@playwright/test'

/**
 * Captures screenshots for visual review.
 * Not part of the test suite; run with: npx tsx scripts/screenshot.ts
 */
const BASE = process.env.E2E_BASE_URL ?? 'http://localhost:4173'
const OUT = 'screenshots'

const CITIES = ['Singapore', 'Tokyo', 'London', 'Reykjavik', 'Oslo', 'Cairo']

async function main() {
  const browser = await chromium.launch()

  for (const [name, viewport] of [
    ['desktop', { width: 1440, height: 1000 }],
    ['mobile', { width: 390, height: 844 }],
  ] as const) {
    const context = await browser.newContext({ viewport, deviceScaleFactor: 2 })
    const page = await context.newPage()

    await page.addInitScript((seed) => {
      window.localStorage.setItem(
        'weather-watcher.tracked-cities',
        JSON.stringify(seed),
      )
    }, CITIES)

    await page.goto(BASE)
    await page.locator('article[data-city]').first().waitFor()
    // Wait for the lazy chart chunk to finish loading.
    await page
      .getByRole('region', { name: /24 hour temperature trend/i })
      .waitFor()
      .catch(() => undefined)
    await page.waitForTimeout(600)

    await page.screenshot({ path: `${OUT}/${name}.png`, fullPage: true })
    await context.close()
  }

  await browser.close()
  console.log(`Wrote ${OUT}/desktop.png and ${OUT}/mobile.png`)
}

main().catch((error: unknown) => {
  console.error(error)
  process.exit(1)
})
