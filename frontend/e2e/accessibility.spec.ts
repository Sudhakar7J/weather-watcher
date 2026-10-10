import { expect, test } from '@playwright/test'

/**
 * Accessibility and interaction affordance checks.
 *
 * These are the failures that are easy to introduce and invisible in a
 * screenshot: an unlabelled control, a focus ring that cannot be seen against
 * a tinted card, or a large click target that does not advertise itself.
 */

test('the page has exactly one level-one heading', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1 })).toHaveCount(1)
})

test('every control has an accessible name', async ({ page }) => {
  await page.goto('/')
  await page.locator('article[data-city]').first().waitFor()

  const unnamed = await page.evaluate(() => {
    const controls = Array.from(
      document.querySelectorAll<HTMLElement>('button, a[href], input, [role="button"]'),
    )

    return controls
      .filter((element) => {
        if (element.getAttribute('aria-hidden') === 'true') return false
        if (element.closest('[aria-hidden="true"]')) return false

        const name =
          element.getAttribute('aria-label') ??
          element.getAttribute('title') ??
          (element as HTMLInputElement).labels?.[0]?.textContent ??
          element.textContent ??
          ''

        return name.trim() === ''
      })
      .map((element) => element.outerHTML.slice(0, 120))
  })

  expect(unnamed).toEqual([])
})

test('cards are operable by keyboard', async ({ page }) => {
  await page.goto('/')
  await page.locator('article[data-city]').first().waitFor()

  // Located by structure rather than accessible name: the label changes once
  // the card is selected, so a name-based locator would stop matching the
  // moment it was used.
  const cardButton = page.locator(
    'article[data-city="Singapore"] button[aria-pressed]',
  )

  await cardButton.focus()
  await expect(cardButton).toBeFocused()

  await page.keyboard.press('Enter')
  await expect(cardButton).toHaveAttribute('aria-pressed', 'true')
})

test('interactive elements advertise themselves with a pointer cursor', async ({ page }) => {
  await page.goto('/')
  await page.locator('article[data-city]').first().waitFor()

  // Large card-sized buttons render with the default arrow cursor otherwise,
  // which reads as a non-interactive surface.
  const cursor = await page.evaluate(() => {
    const button = document.querySelector<HTMLElement>(
      'article[data-city] button[aria-pressed]',
    )
    return button === null ? null : getComputedStyle(button).cursor
  })

  expect(cursor).toBe('pointer')
})

test('the remove control is reachable without hover on touch layouts', async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== 'mobile-chromium', 'touch-only assertion')

  await page.goto('/')
  await page.locator('article[data-city]').first().waitFor()

  const remove = page.getByRole('button', { name: 'Stop tracking Singapore' })
  await expect(remove).toBeVisible()
  await expect(remove).toHaveCSS('opacity', '1')
})

test('a skip link is the first focusable element', async ({ page }) => {
  await page.goto('/')

  await page.keyboard.press('Tab')
  const skip = page.getByRole('link', { name: 'Skip to content' })

  await expect(skip).toBeFocused()
  await expect(skip).toBeVisible()
})

test('the tracked city form announces validation errors', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem(
      'weather-watcher.tracked-cities',
      JSON.stringify(['Singapore']),
    )
  })

  await page.goto('/')

  const input = page.getByLabel('Add a city')
  await input.fill('Singapore')
  await page.getByRole('button', { name: 'Add', exact: true }).click()

  await expect(input).toHaveAttribute('aria-invalid', 'true')

  // The message must be programmatically linked to the field.
  const describedBy = await input.getAttribute('aria-describedby')
  expect(describedBy).toBeTruthy()
  await expect(page.locator(`#${describedBy}`)).toHaveAttribute('role', 'alert')
})

test('the dashboard reports a backend outage with actionable copy', async ({
  page,
}) => {
  // Point the app at a port nothing is listening on.
  await page.route('**/api/weather*', (route) => route.abort('failed'))

  await page.goto('/')

  const alert = page.getByRole('alert')
  await expect(alert).toBeVisible()
  await expect(alert).toContainText(/cannot reach the weather service/i)
  await expect(
    page.getByRole('button', { name: 'Try again' }),
  ).toBeVisible()
})
