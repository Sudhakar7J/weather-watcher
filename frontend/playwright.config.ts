import { defineConfig, devices } from '@playwright/test'

/**
 * End-to-end tests run against the built frontend (vite preview) talking to a
 * real Go backend, so what is exercised is the same request path production
 * uses rather than a mocked fetch.
 *
 * Both servers must be running first; see README.
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? 'github' : 'list',

  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:4173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'desktop-chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'mobile-chromium',
      // The mobile project guards the responsive layout, where the remove
      // control must stay reachable without hover.
      use: { ...devices['Pixel 7'] },
      testMatch: /responsive\.spec\.ts/,
    },
  ],
})
