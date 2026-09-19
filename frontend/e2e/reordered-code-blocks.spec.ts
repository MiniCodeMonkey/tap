import { test, expect } from '@playwright/test';

test.describe('Two Column Layout With Reordered Live Code', () => {
  // Slide 36 "Reordered Live Code" is a two-column layout whose source
  // writes ::right (a live sqlite block) before ::left (a plain js block),
  // so the layout renders the DOM in the opposite order from the source.
  // Regression test for pairing codeBlocks[i] with the i-th <pre> in the
  // DOM: the sql block (codeBlocks[0], the one with a driver) must become
  // the live code widget in the right slot, and the js block must stay a
  // plain, syntax-highlighted block in the left slot.

  test.beforeEach(async ({ page }) => {
    await page.goto('/#36');
    await page.waitForSelector('.slide-container');
  });

  test('should mount the live code widget in the right slot, not the left', async ({ page }) => {
    await expect(page.locator('.slot-right .live-code-block-portal')).toBeVisible();
    await expect(page.locator('.slot-left .live-code-block-portal')).toHaveCount(0);
  });

  test('should keep the left slot as a plain, highlighted code block', async ({ page }) => {
    await expect(page.locator('.slot-left pre.shiki, .slot-left pre code')).toContainText(
      "console.log('static, no driver')"
    );
    await expect(page.locator('.slot-left .live-code-block-portal')).toHaveCount(0);
  });

  test('should not show the sql source text as static content', async ({ page }) => {
    // The sql block's raw text must not remain anywhere as plain code -
    // it should only exist inside the live code widget's own UI.
    await expect(page.locator('.slot-right pre > code.language-sql')).toHaveCount(0);
  });
});
