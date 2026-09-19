import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for the per-theme check suite (overflow,
 * contrast, minimum text size, isolation, and visual snapshots).
 *
 * Unlike playwright.config.ts, this suite has no built-in web server: it
 * runs against a Go dev server and a Vite dev server the caller already
 * started (see docs/reference/theme-porting.md), each on its own pair of
 * ports, so several theme ports can run this suite in parallel without
 * fighting over a shared server or a shared snapshot folder.
 *
 * Run with:
 *   BASE_URL=http://localhost:<vite-port> [THEME=<slug>] \
 *     npx playwright test -c playwright.themes.config.ts
 *
 * With no THEME, every theme with a folder under frontend/src/lib/themes/
 * runs. Snapshots are written per theme, under
 * frontend/e2e-themes/__snapshots__/<slug>/, so parallel runs against
 * different themes never write the same file.
 */

const baseURL = process.env.BASE_URL;
if (!baseURL) {
	throw new Error(
		'BASE_URL is required, e.g. BASE_URL=http://localhost:5300 npx playwright test -c playwright.themes.config.ts'
	);
}

// The suite measures in the slide's own 1920x1080 canvas coordinate space
// (see findSmallText's `scale` factor), not raw CSS pixels, so it is not
// supposed to depend on the browser window's actual size. VIEWPORT_WIDTH/
// VIEWPORT_HEIGHT let a one-off run prove that: shrink the window below
// 1920x1080 and confirm overflow/size checks still pass. Defaults to the
// full canvas size, matching every previous run of this suite.
const viewportWidth = Number(process.env.VIEWPORT_WIDTH) || 1920;
const viewportHeight = Number(process.env.VIEWPORT_HEIGHT) || 1080;

export default defineConfig({
	testDir: './e2e-themes',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 1 : 0,
	// Each check loads its own slide in its own page; nothing here shares
	// server-side state the way the main e2e suite's websocket sync does.
	workers: process.env.CI ? 2 : undefined,
	reporter: [['list']],
	// Snapshots for slug "terminal", slide 3 land at
	// __snapshots__/terminal/slide-3.png, independent of platform or project
	// name, since this suite only ever runs against chromium.
	snapshotPathTemplate: '{testDir}/__snapshots__/{arg}{ext}',
	use: {
		baseURL,
		trace: 'on-first-retry',
		screenshot: 'only-on-failure'
	},
	projects: [
		{
			name: 'chromium',
			use: {
				...devices['Desktop Chrome'],
				// Overrides the device preset's own viewport: every check
				// measures against the slide's native 1920x1080 canvas.
				viewport: { width: viewportWidth, height: viewportHeight },
				deviceScaleFactor: 1
			}
		}
	]
});
