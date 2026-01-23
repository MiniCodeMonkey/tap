import { test, expect } from '@playwright/test';

test.describe('Slide Navigation with Keyboard', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    // Wait for the presentation to load
    await page.waitForSelector('.slide-container');
  });

  test('should display the first slide initially', async ({ page }) => {
    // First slide should be the title slide with "Welcome to Tap"
    await expect(page.locator('.slide-content')).toContainText('Welcome to Tap');
    // URL should have #1 or be empty (defaults to slide 1)
    const url = page.url();
    expect(url.endsWith('#1') || !url.includes('#')).toBeTruthy();
  });

  test('should navigate to next slide with ArrowRight', async ({ page }) => {
    await page.keyboard.press('ArrowRight');
    // Wait for slide transition
    await page.waitForTimeout(500);
    // Second slide is "Getting Started" section
    await expect(page.locator('.slide-content')).toContainText('Getting Started');
    expect(page.url()).toContain('#2');
  });

  test('should navigate to next slide with ArrowDown', async ({ page }) => {
    await page.keyboard.press('ArrowDown');
    await page.waitForTimeout(500);
    await expect(page.locator('.slide-content')).toContainText('Getting Started');
    expect(page.url()).toContain('#2');
  });

  test('should navigate to next slide with Space', async ({ page }) => {
    await page.keyboard.press('Space');
    await page.waitForTimeout(500);
    await expect(page.locator('.slide-content')).toContainText('Getting Started');
    expect(page.url()).toContain('#2');
  });

  test('should navigate to next slide with Enter', async ({ page }) => {
    await page.keyboard.press('Enter');
    await page.waitForTimeout(500);
    await expect(page.locator('.slide-content')).toContainText('Getting Started');
    expect(page.url()).toContain('#2');
  });

  test('should navigate to previous slide with ArrowLeft', async ({ page }) => {
    // First go to slide 2
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#2');

    // Then go back to slide 1
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(500);
    await expect(page.locator('.slide-content')).toContainText('Welcome to Tap');
    expect(page.url()).toContain('#1');
  });

  test('should navigate to previous slide with ArrowUp', async ({ page }) => {
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);

    await page.keyboard.press('ArrowUp');
    await page.waitForTimeout(500);
    await expect(page.locator('.slide-content')).toContainText('Welcome to Tap');
    expect(page.url()).toContain('#1');
  });

  test('should navigate to previous slide with Backspace', async ({ page }) => {
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);

    await page.keyboard.press('Backspace');
    await page.waitForTimeout(500);
    await expect(page.locator('.slide-content')).toContainText('Welcome to Tap');
    expect(page.url()).toContain('#1');
  });

  test('should navigate to first slide with Home key', async ({ page }) => {
    // Navigate to slide 3
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#3');

    // Press Home to go to slide 1
    await page.keyboard.press('Home');
    await page.waitForTimeout(500);
    await expect(page.locator('.slide-content')).toContainText('Welcome to Tap');
    expect(page.url()).toContain('#1');
  });

  test('should navigate to last slide with End key', async ({ page }) => {
    await page.keyboard.press('End');
    await page.waitForTimeout(500);
    // Last slide is "Thank You!"
    await expect(page.locator('.slide-content')).toContainText('Thank You!');
  });

  test('should not navigate before first slide', async ({ page }) => {
    // Try to go back from slide 1
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(300);
    // Should still be on slide 1
    await expect(page.locator('.slide-content')).toContainText('Welcome to Tap');
  });

  test('should not navigate past last slide', async ({ page }) => {
    // Go to last slide
    await page.keyboard.press('End');
    await page.waitForTimeout(500);
    const lastSlideContent = await page.locator('.slide-content').textContent();

    // Try to go forward
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(300);

    // Should still have the same content
    await expect(page.locator('.slide-content')).toContainText('Thank You!');
  });

  test('should skip keyboard navigation when input is focused', async ({ page }) => {
    // This test assumes there might be an input element
    // If no input exists, we can skip this test
    const hasInput = await page.locator('input, textarea').count() > 0;
    if (hasInput) {
      await page.locator('input, textarea').first().focus();
      const initialSlide = page.url();
      await page.keyboard.press('ArrowRight');
      await page.waitForTimeout(300);
      // URL should not change
      expect(page.url()).toBe(initialSlide);
    }
  });
});
