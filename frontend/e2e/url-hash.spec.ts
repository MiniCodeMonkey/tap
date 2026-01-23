import { test, expect } from '@playwright/test';

test.describe('URL Hash Navigation', () => {
  test('should navigate to specific slide via URL hash on load', async ({ page }) => {
    // Navigate directly to slide 3
    await page.goto('/#3');
    await page.waitForSelector('.slide-container');
    // Slide 3 is "Core Features"
    await expect(page.locator('.slide-content')).toContainText('Core Features');
  });

  test('should navigate to slide 5 via URL hash', async ({ page }) => {
    // Navigate directly to slide 5
    await page.goto('/#5');
    await page.waitForSelector('.slide-container');
    // Slide 5 is "Code Focus Layout"
    await expect(page.locator('.slide-content')).toContainText('Code Focus Layout');
  });

  test('should update URL hash when navigating with keyboard', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.slide-container');

    // Navigate forward
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#2');

    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#3');

    // Navigate backward
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#2');
  });

  test('should handle browser back/forward navigation', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.slide-container');

    // Navigate to slide 2
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#2');

    // Navigate to slide 3
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#3');

    // Use browser back button
    await page.goBack();
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#2');
    await expect(page.locator('.slide-content')).toContainText('Getting Started');

    // Use browser forward button
    await page.goForward();
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#3');
    await expect(page.locator('.slide-content')).toContainText('Core Features');
  });

  test('should handle invalid hash gracefully (hash too high)', async ({ page }) => {
    // Navigate to a slide number that exceeds the total
    await page.goto('/#999');
    await page.waitForSelector('.slide-container');
    // Should show the last slide or stay on a valid slide
    const content = await page.locator('.slide-content').textContent();
    expect(content).toBeTruthy();
  });

  test('should handle invalid hash gracefully (negative or zero)', async ({ page }) => {
    // Navigate to slide 0 (invalid)
    await page.goto('/#0');
    await page.waitForSelector('.slide-container');
    // Should show the first slide
    await expect(page.locator('.slide-content')).toContainText('Welcome to Tap');
  });

  test('should handle non-numeric hash gracefully', async ({ page }) => {
    // Navigate with non-numeric hash
    await page.goto('/#abc');
    await page.waitForSelector('.slide-container');
    // Should show the first slide
    const content = await page.locator('.slide-content').textContent();
    expect(content).toBeTruthy();
  });

  test('should persist slide position on page refresh', async ({ page }) => {
    // Navigate to slide 4
    await page.goto('/#4');
    await page.waitForSelector('.slide-container');
    await expect(page.locator('.slide-content')).toContainText('Two Column Layout');

    // Refresh the page
    await page.reload();
    await page.waitForSelector('.slide-container');

    // Should still be on slide 4
    expect(page.url()).toContain('#4');
    await expect(page.locator('.slide-content')).toContainText('Two Column Layout');
  });

  test('should sync URL when using Home key', async ({ page }) => {
    // Start at slide 5
    await page.goto('/#5');
    await page.waitForSelector('.slide-container');

    // Press Home to go to first slide
    await page.keyboard.press('Home');
    await page.waitForTimeout(500);

    expect(page.url()).toContain('#1');
    await expect(page.locator('.slide-content')).toContainText('Welcome to Tap');
  });

  test('should sync URL when using End key', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.slide-container');

    // Press End to go to last slide
    await page.keyboard.press('End');
    await page.waitForTimeout(500);

    // URL should have the last slide number
    const url = page.url();
    const match = url.match(/#(\d+)/);
    expect(match).toBeTruthy();
    const slideNumber = parseInt(match![1], 10);
    expect(slideNumber).toBeGreaterThan(20); // Sample has many slides

    await expect(page.locator('.slide-content')).toContainText('Thank You!');
  });
});
