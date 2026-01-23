import { test, expect } from '@playwright/test';

test.describe('Fragment Reveals', () => {
  // Slide 3 "Core Features" has fragments (pause markers)
  // Slide 11 "Fragment Reveals" also has fragments

  test.beforeEach(async ({ page }) => {
    // Navigate to slide 3 which has fragments
    await page.goto('/#3');
    await page.waitForSelector('.slide-container');
  });

  test('should display initial content without hidden fragments', async ({ page }) => {
    // Slide 3 starts with "Core Features" and "Tap is designed for developers"
    await expect(page.locator('.slide-content')).toContainText('Core Features');
    await expect(page.locator('.slide-content')).toContainText('Tap is designed for developers');
  });

  test('should reveal first fragment on forward navigation', async ({ page }) => {
    // Initial state - check content is visible
    const content = page.locator('.slide-content');
    await expect(content).toContainText('Core Features');

    // Press right to reveal first fragment
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);

    // Should reveal the first bullet points
    await expect(content).toContainText('Simple');
    await expect(content).toContainText('Powerful');
  });

  test('should reveal second fragment on second forward navigation', async ({ page }) => {
    const content = page.locator('.slide-content');

    // Reveal first fragment
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);

    // Reveal second fragment
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);

    // Should now show all content including the second set of bullets
    await expect(content).toContainText('Fast');
    await expect(content).toContainText('Beautiful');
  });

  test('should move to next slide after all fragments revealed', async ({ page }) => {
    // Slide 3 has 2 pause markers, so 3 navigations to get to next slide
    await page.keyboard.press('ArrowRight'); // Reveal fragment 1
    await page.waitForTimeout(300);
    await page.keyboard.press('ArrowRight'); // Reveal fragment 2
    await page.waitForTimeout(300);
    await page.keyboard.press('ArrowRight'); // Move to slide 4
    await page.waitForTimeout(500);

    // Should now be on slide 4 "Two Column Layout"
    expect(page.url()).toContain('#4');
    await expect(page.locator('.slide-content')).toContainText('Two Column Layout');
  });

  test('should hide last fragment on backward navigation', async ({ page }) => {
    const content = page.locator('.slide-content');

    // Reveal all fragments
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(300);
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(300);

    // All content should be visible
    await expect(content).toContainText('Fast');
    await expect(content).toContainText('Beautiful');

    // Go back one fragment
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(500);

    // Should still be on slide 3
    expect(page.url()).toContain('#3');
  });

  test('should go to previous slide when at first fragment and pressing back', async ({ page }) => {
    // Try to go back from slide 3 with no fragments revealed
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(500);

    // Should be on slide 2 "Getting Started"
    expect(page.url()).toContain('#2');
    await expect(page.locator('.slide-content')).toContainText('Getting Started');
  });

  test('should handle rapid fragment navigation', async ({ page }) => {
    // Rapidly press right multiple times
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(600);

    // Should be on slide 4 after going through all fragments
    expect(page.url()).toContain('#4');
  });

  test('should work with Space key for fragment reveals', async ({ page }) => {
    const content = page.locator('.slide-content');

    // Use Space to reveal fragments
    await page.keyboard.press('Space');
    await page.waitForTimeout(500);

    // Should reveal first fragment
    await expect(content).toContainText('Simple');
    await expect(content).toContainText('Powerful');
  });

  test('should work with Enter key for fragment reveals', async ({ page }) => {
    const content = page.locator('.slide-content');

    // Use Enter to reveal fragments
    await page.keyboard.press('Enter');
    await page.waitForTimeout(500);

    // Should reveal first fragment
    await expect(content).toContainText('Simple');
    await expect(content).toContainText('Powerful');
  });
});

test.describe('Fragment Reveals on Slide 11', () => {
  // Slide 11 "Fragment Reveals" explicitly demonstrates this feature

  test('should reveal numbered points incrementally', async ({ page }) => {
    await page.goto('/#11');
    await page.waitForSelector('.slide-container');

    const content = page.locator('.slide-content');
    await expect(content).toContainText('Fragment Reveals');

    // Reveal first numbered point
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);
    await expect(content).toContainText('First point appears');

    // Reveal second numbered point
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);
    await expect(content).toContainText('Then the second');

    // Reveal third numbered point
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);
    await expect(content).toContainText('And finally the third');
  });
});
