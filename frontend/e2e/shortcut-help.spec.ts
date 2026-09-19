import { test, expect } from '@playwright/test';

test.describe('Shortcut overlay', () => {
  test('opens with ? and closes with ?, Escape, and a click outside in the audience view', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.slide-container');

    const overlay = page.locator('.shortcut-help');
    await expect(overlay).toHaveCount(0);

    await page.keyboard.press('Shift+Slash');
    await expect(overlay).toBeVisible();
    await expect(overlay).toContainText('Toggle the slide overview');
    await expect(overlay).not.toContainText('Reset the timer');

    // Navigation is ignored while the overlay is open.
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(300);
    expect(page.url().endsWith('#1') || !page.url().includes('#')).toBeTruthy();

    await page.keyboard.press('Shift+Slash');
    await expect(overlay).toHaveCount(0);

    await page.keyboard.press('Shift+Slash');
    await page.keyboard.press('Escape');
    await expect(overlay).toHaveCount(0);

    await page.keyboard.press('Shift+Slash');
    await page.mouse.click(5, 5);
    await expect(overlay).toHaveCount(0);
  });

  test('opens with ? and closes with Escape in the presenter view', async ({ page }) => {
    await page.goto('/presenter');
    await page.waitForSelector('.presenter-view');

    const overlay = page.locator('.shortcut-help');
    await page.keyboard.press('Shift+Slash');
    await expect(overlay).toBeVisible();
    await expect(overlay).toContainText('Reset the timer');
    await expect(overlay).not.toContainText('Toggle the slide overview');

    await page.keyboard.press('Escape');
    await expect(overlay).toHaveCount(0);
  });
});

test.describe('Presenter layout', () => {
  for (const width of [1024, 1440, 1920]) {
    test(`current slide is larger than the next slide and both fit at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto('/presenter');
      await page.waitForSelector('.presenter-view');

      const current = await page.locator('.presenter-slide-preview.current').boundingBox();
      const next = await page.locator('.presenter-slide-preview.next').boundingBox();
      const notes = await page.locator('.presenter-notes-panel').boundingBox();
      expect(current && next && notes).toBeTruthy();

      expect(current!.width * current!.height).toBeGreaterThan(next!.width * next!.height * 1.5);
      // Notes sit under the next slide, in the same column.
      expect(notes!.y).toBeGreaterThan(next!.y + next!.height - 1);
      expect(Math.abs(notes!.x - next!.x)).toBeLessThan(2);
      // Nothing overflows the window.
      for (const box of [current!, next!, notes!]) {
        expect(box.x + box.width).toBeLessThanOrEqual(width + 1);
        expect(box.y + box.height).toBeLessThanOrEqual(900 + 1);
      }
    });
  }

  test('stacks the notes before the next slide on a phone', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/presenter');
    await page.waitForSelector('.presenter-view');

    const next = await page.locator('.presenter-slide-preview.next').boundingBox();
    const notes = await page.locator('.presenter-notes-panel').boundingBox();
    expect(notes!.y).toBeLessThan(next!.y);
  });

  test('notes font size survives a reload', async ({ page }) => {
    await page.goto('/presenter');
    await page.waitForSelector('.presenter-notes-content');

    await page.getByLabel('Larger speaker notes').click();
    await page.keyboard.press('Equal');
    const size = await page.locator('.presenter-notes-content').evaluate((element) => element.style.fontSize);
    expect(size).toBe('1.75rem');

    await page.reload();
    await page.waitForSelector('.presenter-notes-content');
    await expect(page.locator('.presenter-notes-content')).toHaveAttribute('style', /font-size: 1\.75rem/);
  });
});
