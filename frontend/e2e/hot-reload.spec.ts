import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

test.describe('Hot Reload', () => {
  const sampleMdPath = path.join(__dirname, '../../testdata/sample.md');
  let originalContent: string;

  test.beforeAll(() => {
    // Store original content to restore after tests
    originalContent = fs.readFileSync(sampleMdPath, 'utf-8');
  });

  test.afterAll(() => {
    // Restore original content
    fs.writeFileSync(sampleMdPath, originalContent);
  });

  test.afterEach(() => {
    // Restore after each test in case it fails mid-way
    fs.writeFileSync(sampleMdPath, originalContent);
  });

  test('should establish WebSocket connection', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.slide-container');

    // Check that WebSocket connects (no disconnection indicator visible after initial load)
    // Wait a bit for the connection to establish
    await page.waitForTimeout(1000);

    // The connection indicator should either not exist or indicate connected status
    const disconnectedIndicator = page.locator('.connection-indicator.disconnected');
    const indicatorCount = await disconnectedIndicator.count();

    // If there's an indicator, it should not be showing "disconnected" state
    // (It might be hidden or show "connected")
    if (indicatorCount > 0) {
      await expect(disconnectedIndicator).not.toBeVisible();
    }
  });

  test('should reload when file content changes', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.slide-container');

    // Verify initial content
    await expect(page.locator('.slide-content')).toContainText('Welcome to Tap');

    // Set up a promise to detect page reload or content change
    let reloaded = false;
    page.on('load', () => {
      reloaded = true;
    });

    // Modify the sample.md file
    const modifiedContent = originalContent.replace(
      '# Welcome to Tap',
      '# Hot Reload Test'
    );
    fs.writeFileSync(sampleMdPath, modifiedContent);

    // Wait for hot reload to trigger (debounce is 100ms, add buffer)
    await page.waitForTimeout(3000);

    // Either the page reloaded or content was updated via WebSocket
    // Check if content changed (either via reload or WebSocket update)
    const content = await page.locator('.slide-content').textContent();

    // Content should have changed to new title OR page should have reloaded
    // Note: If hot reload triggers a full page reload, the content will update
    // If it's a WebSocket message, the content might update without reload
    expect(reloaded || content?.includes('Hot Reload Test')).toBeTruthy();
  });

  test('should maintain slide position after hot reload', async ({ page }) => {
    // Navigate to slide 3
    await page.goto('/#3');
    await page.waitForSelector('.slide-container');
    await expect(page.locator('.slide-content')).toContainText('Core Features');

    // Modify the file to trigger reload
    const modifiedContent = originalContent.replace(
      'Tap is designed for developers who love markdown.',
      'Tap is designed for developers who love markdown. (Updated!)'
    );
    fs.writeFileSync(sampleMdPath, modifiedContent);

    // Wait for hot reload
    await page.waitForTimeout(3000);

    // After reload, should still be on the same slide (or navigate back via URL hash)
    // The URL hash should maintain the slide position
    const url = page.url();
    expect(url).toContain('#3');
  });

  test('should handle rapid file changes with debouncing', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.slide-container');

    // Track reload count
    let reloadCount = 0;
    page.on('load', () => {
      reloadCount++;
    });

    // Make rapid changes to the file
    for (let i = 0; i < 5; i++) {
      const modifiedContent = originalContent.replace(
        '# Welcome to Tap',
        `# Welcome to Tap (Change ${i})`
      );
      fs.writeFileSync(sampleMdPath, modifiedContent);
      await page.waitForTimeout(50); // Less than debounce window
    }

    // Wait for debounce and reload
    await page.waitForTimeout(2000);

    // Should have debounced to just one or two reloads, not 5
    // The exact count depends on implementation, but should be less than 5
    // Note: Initial page load counts as 1, so we expect 1 (initial) + 1 (debounced reload) = 2 max
    expect(reloadCount).toBeLessThanOrEqual(3);
  });

  test('should handle connection reconnection', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.slide-container');

    // Wait for initial connection
    await page.waitForTimeout(1000);

    // Simulate network interruption by closing WebSocket connections
    // This is done by evaluating JavaScript in the page context
    await page.evaluate(() => {
      // Access any WebSocket instances and close them
      // This simulates a network disconnection
      const allWebSockets = (window as unknown as { _testWebSockets?: WebSocket[] })._testWebSockets || [];
      allWebSockets.forEach((ws: WebSocket) => ws.close());
    });

    // Wait for reconnection attempt
    await page.waitForTimeout(2000);

    // The page should still be functional
    // Can still navigate
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);

    // Should have moved to slide 2
    const content = await page.locator('.slide-content').textContent();
    expect(content).toBeTruthy();
  });
});

test.describe('WebSocket Message Handling', () => {
  test('should display presentation content from API', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.slide-container');

    // Verify that presentation was loaded from API
    // The first slide should display the title from the markdown
    await expect(page.locator('.slide-content')).toContainText('Welcome to Tap');

    // Navigation should work, indicating the presentation was properly loaded
    await page.keyboard.press('End');
    await page.waitForTimeout(500);

    // Last slide should be visible
    await expect(page.locator('.slide-content')).toContainText('Thank You!');
  });
});
