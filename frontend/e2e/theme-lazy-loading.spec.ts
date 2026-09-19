import { test, expect } from '@playwright/test';

/**
 * A theme's CSS is loaded on demand (see $lib/themes/loader.ts's loadTheme):
 * `base` ships in the main bundle, but every other theme's stylesheet is
 * its own chunk, named `theme-<slug>-<hash>` by vite.config.ts specifically
 * so a request for it is identifiable by slug. This deck loads on theme
 * `base`, so the `terminal` chunk must not be requested until the deck
 * actually switches to it.
 */

test('a theme stylesheet is not requested until the deck switches to it', async ({ page }) => {
	const requestedUrls: string[] = [];
	page.on('request', (request) => requestedUrls.push(request.url()));

	await page.goto('/#1');
	await page.waitForSelector('.slide-container');

	expect(requestedUrls.some((url) => url.includes('theme-terminal-'))).toBe(false);

	await page.keyboard.press('t');
	await expect(page.locator('[data-theme]').first()).toHaveAttribute('data-theme', 'terminal');

	await expect
		.poll(() => requestedUrls.some((url) => url.includes('theme-terminal-')))
		.toBe(true);
});
