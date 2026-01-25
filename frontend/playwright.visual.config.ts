import { defineConfig } from '@playwright/test';

/**
 * Playwright configuration for visual testing of Tap templates
 * Runs against all example templates and captures screenshots
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests/visual',
  // Run tests sequentially since each starts its own server
  fullyParallel: false,
  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,
  // No retries for visual tests - we want to see failures immediately
  retries: 0,
  // Single worker since each test manages its own server
  workers: 1,
  // Reporter configuration
  reporter: [
    ['html', { open: 'never', outputFolder: 'tests/visual/report' }],
    ['list'],
  ],
  // Shared settings for all projects
  use: {
    // Fixed viewport for consistent screenshots
    viewport: { width: 1920, height: 1080 },
    // Collect trace on failure for debugging
    trace: 'on-first-retry',
    // Always take screenshots for visual tests
    screenshot: 'on',
  },

  // Configure projects - chromium only for visual consistency
  projects: [
    {
      name: 'chromium',
      use: {
        browserName: 'chromium',
        // Disable animations for consistent screenshots
        launchOptions: {
          args: ['--disable-animations'],
        },
      },
    },
  ],

  // Output directory for screenshots
  outputDir: './tests/visual/screenshots',

  // Timeout settings
  timeout: 60000, // 60 seconds per test
  expect: {
    timeout: 10000,
  },
});
