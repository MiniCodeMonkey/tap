import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright E2E test configuration for Tap presentation viewer
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './e2e',
  // Run tests in files in parallel
  fullyParallel: true,
  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,
  // Retry on CI only
  retries: process.env.CI ? 2 : 0,
  // All specs share one dev server whose websocket syncs the current slide between
  // every connected client, so parallel test sessions would navigate each other's pages.
  workers: 1,
  // Reporter to use
  reporter: [
    ['html', { open: 'never' }],
    ['list'],
  ],
  // Shared settings for all projects
  use: {
    // Base URL to use in tests. Port 3100 is dedicated to this suite's own
    // server (see webServer below) so it never collides with a developer's
    // own `tap dev` on the default port 3000.
    baseURL: 'http://localhost:3100',
    // Collect trace on first retry
    trace: 'on-first-retry',
    // Take screenshot on failure
    screenshot: 'only-on-failure',
  },

  // Configure projects for different browsers
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    // Uncomment to test in Firefox and Safari
    // {
    //   name: 'firefox',
    //   use: { ...devices['Desktop Firefox'] },
    // },
    // {
    //   name: 'webkit',
    //   use: { ...devices['Desktop Safari'] },
    // },
  ],

  // Run the dev server before starting tests
  webServer: {
    // Run the Go dev server with the sample presentation. Every spec in
    // this suite shares this one server; TAP_HUB_STATE_RETENTION=0s keeps
    // its websocket hub from retaining slide/fragment state between
    // specs (see CONTRIBUTING.md), so one spec's viewer disconnecting
    // never leaves state behind for a later spec's "no state" assertions
    // to see. Port 3100 is dedicated to this suite: reuseExistingServer is
    // always false so Playwright never silently attaches to somebody
    // else's already-running server (a developer's own `tap dev`, or a
    // stray server left behind by another run) and serves that server's
    // deck and stale hub state to these specs instead of its own.
    command: 'cd .. && TAP_HUB_STATE_RETENTION=0s go run ./cmd/tap dev testdata/sample.md --port 3100 --headless',
    url: 'http://localhost:3100',
    reuseExistingServer: false,
    timeout: 30000, // 30 seconds to start server
  },
});
