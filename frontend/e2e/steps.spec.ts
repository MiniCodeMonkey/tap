import { test, expect } from '@playwright/test';

/**
 * Step mechanism E2E tests, covering the map slide at the end of the sample
 * deck. A slide with a `map` code block has one step: the first ArrowRight
 * runs the map animation and keeps the same slide, and the next ArrowRight
 * continues to the following slide.
 *
 * These assertions only need the URL hash and the `.map-slide` element,
 * never the loaded map tiles, so they run offline without waiting on
 * `__tapMapReady` (which requires fetching the map style over the network).
 */

test.describe('Map Slide Step Mechanism', () => {
  test.beforeEach(async ({ page }) => {
    // Slide 34 is the map slide appended to the sample deck.
    await page.goto('/#34');
    await page.waitForSelector('.slide-container');
    await page.waitForSelector('.map-slide');
  });

  test('should stay on the map slide and start the animation on the first ArrowRight', async ({
    page
  }) => {
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(300);

    // The step advanced within the same slide, so the hash is unchanged.
    expect(page.url()).toContain('#34');
    await expect(page.locator('.map-slide')).toBeVisible();
  });

  test('should advance to the next slide on the second ArrowRight', async ({ page }) => {
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(300);
    expect(page.url()).toContain('#34');

    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(300);

    expect(page.url()).toContain('#35');
  });

  test('should return to the map slide in its final state on ArrowLeft from the next slide', async ({
    page
  }) => {
    // Step through the map slide to the next slide.
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(300);
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(300);
    expect(page.url()).toContain('#35');

    // Navigate back to the map slide.
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(300);

    expect(page.url()).toContain('#34');
    await expect(page.locator('.map-slide')).toBeVisible();
  });
});
