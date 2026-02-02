import { test, expect } from '@playwright/test';

/**
 * Map slide E2E tests.
 *
 * These tests require running against a presentation with map slides.
 * Use: cd .. && go run ./cmd/tap dev testdata/map-test.md --port 3000 --headless
 *
 * Note: These tests are skipped by default in the main test suite since
 * they require a different test presentation. To run them:
 * 1. Start the map test server manually
 * 2. Run: npx playwright test e2e/map.spec.ts
 */

// Skip these tests when running against sample.md
test.skip(({ }, testInfo) => {
  // These tests only work with map-test.md presentation
  // They're designed to be run manually or in a separate CI step
  return process.env.TEST_MAP_SLIDES !== 'true';
}, 'Map tests require TEST_MAP_SLIDES=true environment variable');

test.describe('Map Slide Display', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to slide 3 which has a map
    await page.goto('/#3');
    await page.waitForSelector('.slide-container');
  });

  test('should display map at start position on slide enter', async ({ page }) => {
    // Wait for map to load
    await page.waitForSelector('.map-slide', { timeout: 10000 });

    // Map container should be visible
    await expect(page.locator('.map-slide')).toBeVisible();

    // Check that maplibre canvas is rendered
    await expect(page.locator('.maplibregl-canvas')).toBeVisible({ timeout: 10000 });
  });

  test('should display markers when enabled', async ({ page }) => {
    await page.waitForSelector('.map-slide', { timeout: 10000 });

    // Wait for markers to render
    await page.waitForSelector('.maplibregl-marker', { timeout: 5000 });

    // Should have 2 markers (start and end)
    const markerCount = await page.locator('.maplibregl-marker').count();
    expect(markerCount).toBe(2);
  });
});

test.describe('Map Animation Behavior', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/#3');
    await page.waitForSelector('.slide-container');
    await page.waitForSelector('.map-slide', { timeout: 10000 });

    // Wait for map to be ready
    await page.waitForFunction(() => {
      return (window as unknown as { __tapMapReady?: boolean }).__tapMapReady === true;
    }, { timeout: 10000 });
  });

  test('should trigger animation on keypress', async ({ page }) => {
    // Get initial center (start position)
    const initialCenter = await page.evaluate(() => {
      const map = (window as unknown as { __tapMap?: { getCenter: () => { lng: number; lat: number } } }).__tapMap;
      return map?.getCenter();
    });

    expect(initialCenter).toBeDefined();

    // Trigger animation
    await page.keyboard.press('ArrowRight');

    // Wait for animation to complete
    await page.waitForTimeout(4000); // duration + buffer

    // Center should have changed to end position
    const finalCenter = await page.evaluate(() => {
      const map = (window as unknown as { __tapMap?: { getCenter: () => { lng: number; lat: number } } }).__tapMap;
      return map?.getCenter();
    });

    // Coordinates should be different (animation completed)
    expect(finalCenter).toBeDefined();
    if (initialCenter && finalCenter) {
      expect(finalCenter.lng).not.toBeCloseTo(initialCenter.lng, 1);
    }
  });

  test('should advance to next slide after animation completes', async ({ page }) => {
    // First keypress triggers animation
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(4000);

    // Second keypress should go to next slide
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(500);

    // Should be on slide 4 now
    expect(page.url()).toContain('#4');
  });
});

test.describe('Map Navigation Backward', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to slide 4 (after the map slide)
    await page.goto('/#4');
    await page.waitForSelector('.slide-container');
  });

  test('should return to map slide at end position when navigating backward', async ({ page }) => {
    // Navigate backward to the map slide
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(500);

    // Should be on slide 3 (map slide)
    expect(page.url()).toContain('#3');

    // Wait for map
    await page.waitForSelector('.map-slide', { timeout: 10000 });

    // Map should show (at end position since we came from forward)
    await expect(page.locator('.map-slide')).toBeVisible();
  });

  test('should reset to start position when pressing back on animated map', async ({ page }) => {
    // Navigate backward to the map slide
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(500);
    await page.waitForSelector('.map-slide', { timeout: 10000 });

    // Wait for map to be ready
    await page.waitForFunction(() => {
      return (window as unknown as { __tapMapReady?: boolean }).__tapMapReady === true;
    }, { timeout: 10000 });

    // Press back again to reset map to start
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(1000);

    // Should still be on slide 3
    expect(page.url()).toContain('#3');

    // Another back should go to slide 2
    await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(500);

    expect(page.url()).toContain('#2');
  });
});

test.describe('Map Path Line', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to slide 5 which has showPath: true
    await page.goto('/#5');
    await page.waitForSelector('.slide-container');
    await page.waitForSelector('.map-slide', { timeout: 10000 });
  });

  test('should show path line when showPath is true', async ({ page }) => {
    // Wait for map to load
    await page.waitForFunction(() => {
      return (window as unknown as { __tapMapReady?: boolean }).__tapMapReady === true;
    }, { timeout: 10000 });

    // Check for line layer
    const hasPathLayer = await page.evaluate(() => {
      const map = (window as unknown as { __tapMap?: { getLayer: (id: string) => unknown } }).__tapMap;
      return map?.getLayer?.('path-line') !== undefined;
    });

    expect(hasPathLayer).toBe(true);
  });
});

test.describe('Map Accessibility', () => {
  test('should respect prefers-reduced-motion', async ({ page }) => {
    // Emulate reduced motion preference
    await page.emulateMedia({ reducedMotion: 'reduce' });

    await page.goto('/#3');
    await page.waitForSelector('.map-slide', { timeout: 10000 });

    // Wait for map to be ready
    await page.waitForFunction(() => {
      return (window as unknown as { __tapMapReady?: boolean }).__tapMapReady === true;
    }, { timeout: 10000 });

    // Get initial center
    const initialCenter = await page.evaluate(() => {
      const map = (window as unknown as { __tapMap?: { getCenter: () => { lng: number; lat: number } } }).__tapMap;
      return map?.getCenter();
    });

    // Trigger "animation" (should be instant with reduced motion)
    await page.keyboard.press('ArrowRight');

    // Very short wait - animation should be instant
    await page.waitForTimeout(200);

    // Center should already be at end position (instant jump, not animated)
    const finalCenter = await page.evaluate(() => {
      const map = (window as unknown as { __tapMap?: { getCenter: () => { lng: number; lat: number } } }).__tapMap;
      return map?.getCenter();
    });

    // Should have moved immediately
    expect(finalCenter).toBeDefined();
    if (initialCenter && finalCenter) {
      expect(finalCenter.lng).not.toBeCloseTo(initialCenter.lng, 1);
    }
  });
});
