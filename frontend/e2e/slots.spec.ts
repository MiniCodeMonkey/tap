import { test, expect } from '@playwright/test';

test.describe('Big Stat Layout Slots', () => {
  // Slide 32 "99%" is a big-stat layout with ::caption and ::figure slots,
  // appended to the end of the sample deck.

  test.beforeEach(async ({ page }) => {
    await page.goto('/#32');
    await page.waitForSelector('.slide-container');
  });

  test('should render the caption slot text', async ({ page }) => {
    await expect(page.locator('.layout-big-stat .slot-caption')).toContainText(
      'Uptime across every region'
    );
  });

  test('should render a visible image in the figure slot', async ({ page }) => {
    await expect(page.locator('.layout-big-stat .slot-figure img')).toBeVisible();
  });
});

test.describe('Two Column Layout Fragment in a Slot', () => {
  // Slide 33 "Column Fragments" has a pause fragment inside ::right.

  test.beforeEach(async ({ page }) => {
    await page.goto('/#33');
    await page.waitForSelector('.slide-container');
  });

  test('should hide the fragment in the right slot until advanced', async ({ page }) => {
    const fragment = page.locator('.slot-right [data-fragment-index]');
    await expect(fragment).toHaveClass(/fragment-hidden/);
  });

  test('should reveal the fragment in the right slot after ArrowRight', async ({ page }) => {
    const fragment = page.locator('.slot-right [data-fragment-index]');
    await expect(fragment).toHaveClass(/fragment-hidden/);

    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);

    await expect(fragment).toHaveClass(/fragment-visible/);
    await expect(fragment).toBeVisible();
  });
});
