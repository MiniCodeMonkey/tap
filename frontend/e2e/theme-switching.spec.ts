import { test, expect } from '@playwright/test';

/**
 * Coverage for switching themes at runtime, against the shared suite
 * server (see playwright.config.ts): the `t` key cycles to the next theme
 * in themes.json order, and `?theme=<slug>` selects one directly at load.
 * `data-theme` lives on the canvas frame (`.slide` under `.slide-container`
 * - see SlideCanvas.tsx), not on the outer slide content, so it is
 * selected here by the attribute alone.
 */

test.describe('Theme switching', () => {
	test('the t key cycles to the next theme, changing data-theme on the canvas frame', async ({ page }) => {
		await page.goto('/#1');
		await page.waitForSelector('.slide-container');

		const canvas = page.locator('[data-theme]').first();
		await expect(canvas).toHaveAttribute('data-theme', 'base');

		await page.keyboard.press('t');

		// terminal is the theme after base in internal/themes/themes.json's
		// order, which listThemes()/cycleTheme() follow.
		await expect(canvas).toHaveAttribute('data-theme', 'terminal');
	});

	test('?theme=<slug> selects that theme on load', async ({ page }) => {
		await page.goto('/?theme=terminal#1');
		await page.waitForSelector('.slide-container');

		const canvas = page.locator('[data-theme]').first();
		await expect(canvas).toHaveAttribute('data-theme', 'terminal');
	});
});
