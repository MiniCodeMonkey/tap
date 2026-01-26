import { test, expect } from '@playwright/test';

test.describe('Scroll Reveal', () => {
  // Scroll test slide is slide #29 (third to last)
  const SCROLL_SLIDE = '#29';
  const AFTER_SCROLL_SLIDE = '#30';

  test.beforeEach(async ({ page }) => {
    await page.goto(`/${SCROLL_SLIDE}`);
    await page.waitForSelector('.slide-container');
  });

  test('slide with scroll directive should have scroll-enabled class', async ({ page }) => {
    const slideRenderer = page.locator('.slide-renderer');
    await expect(slideRenderer).toHaveClass(/scroll-enabled/);
  });

  test('should start at top position', async ({ page }) => {
    const scrollContent = page.locator('.scroll-content');
    const transform = await scrollContent.evaluate(el =>
      window.getComputedStyle(el).transform
    );
    expect(transform === 'none' || transform === 'matrix(1, 0, 0, 1, 0, 0)').toBeTruthy();
  });

  test('first ArrowRight should scroll instead of advancing slide', async ({ page }) => {
    const initialUrl = page.url();

    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(600); // 500ms animation + buffer

    // URL should NOT change
    expect(page.url()).toBe(initialUrl);

    // Transform should be non-zero (scrolled down)
    const scrollContent = page.locator('.scroll-content');
    const transform = await scrollContent.evaluate(el =>
      window.getComputedStyle(el).transform
    );
    expect(transform).not.toBe('none');
    expect(transform).not.toBe('matrix(1, 0, 0, 1, 0, 0)');
  });

  test('second ArrowRight should advance to next slide', async ({ page }) => {
    await page.keyboard.press('ArrowRight'); // Scroll
    await page.waitForTimeout(600);

    await page.keyboard.press('ArrowRight'); // Advance
    await page.waitForTimeout(500);

    expect(page.url()).toContain(AFTER_SCROLL_SLIDE);
    await expect(page.locator('.slide-content')).toContainText('After Scroll Slide');
  });

  test('ArrowLeft from next slide returns to scrolled state', async ({ page }) => {
    // Navigate through scroll to next slide
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(600);
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);

    // Go back
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(500);

    // Should be on scroll slide, scrolled to bottom
    expect(page.url()).toContain(SCROLL_SLIDE);

    const scrollContent = page.locator('.scroll-content');
    const transform = await scrollContent.evaluate(el =>
      window.getComputedStyle(el).transform
    );
    expect(transform).not.toBe('none');
  });

  test('second ArrowLeft scrolls back to top', async ({ page }) => {
    // Navigate through and back to scrolled state
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(600);
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(500);

    // Scroll back to top
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(600);

    // Should be at top
    const scrollContent = page.locator('.scroll-content');
    const transform = await scrollContent.evaluate(el =>
      window.getComputedStyle(el).transform
    );
    expect(transform === 'none' || transform.includes('0, 0)')).toBeTruthy();
  });

  test('should work with Space key', async ({ page }) => {
    await page.keyboard.press('Space');
    await page.waitForTimeout(600);

    const scrollContent = page.locator('.scroll-content');
    const transform = await scrollContent.evaluate(el =>
      window.getComputedStyle(el).transform
    );
    expect(transform).not.toBe('none');
  });
});
