import { test, expect, type Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

/**
 * Entrance animations belong to arriving at a slide. A live update (a file
 * save under `tap dev`, or an edit sent by Tap Desktop) replaces the slide's
 * content in place and must not replay them, while navigating to a slide
 * still plays them. The editorial theme animates every `.slot > *` in.
 */

const dirname = path.dirname(fileURLToPath(import.meta.url));
const sampleMdPath = path.join(dirname, '../../testdata/sample.md');

/** Finite CSS animations running anywhere in the slide's slots. */
function runningEntranceAnimations(page: Page): Promise<string[]> {
  return page.evaluate(() =>
    Array.from(document.querySelectorAll('.slide .slot'))
      .flatMap((slot) => slot.getAnimations({ subtree: true }))
      .filter(
        (animation) =>
          'animationName' in animation &&
          animation.playState === 'running' &&
          animation.effect?.getComputedTiming().endTime !== Infinity,
      )
      .map((animation) => (animation as CSSAnimation).animationName),
  );
}

test.describe('Live update animations', () => {
  let originalContent: string;

  test.beforeAll(() => {
    originalContent = fs.readFileSync(sampleMdPath, 'utf-8');
  });

  test.afterEach(() => {
    fs.writeFileSync(sampleMdPath, originalContent);
  });

  test('a live update does not replay entrance animations, and navigating plays them', async ({ page }) => {
    const editorial = originalContent.replace(/^theme: .*$/m, 'theme: editorial');
    fs.writeFileSync(sampleMdPath, editorial);

    // The watcher picks the theme change up shortly after the write.
    await expect(async () => {
      await page.goto('/#3');
      await expect(page.locator('[data-theme="editorial"]')).toHaveCount(1, { timeout: 1000 });
    }).toPass({ timeout: 10000 });
    await expect(page.locator('.slide-content')).toContainText('Core Features');

    // Arriving at the slide played its entrance animations; let them end.
    await expect.poll(() => runningEntranceAnimations(page), { timeout: 5000 }).toEqual([]);
    await page.keyboard.press('ArrowRight');
    const fragments = page.locator('.slide [data-fragment-index]');
    await expect(fragments.first()).toHaveClass(/fragment-visible/);
    await page.waitForTimeout(600);

    fs.writeFileSync(sampleMdPath, editorial.replace('Tap is designed for developers', 'Tap is built for developers'));
    await expect(page.locator('.slide-content')).toContainText('Tap is built for developers', {
      timeout: 5000,
    });

    // Sampled over a few frames, well inside the theme's entrance duration.
    for (let sample = 0; sample < 5; sample++) {
      expect(await runningEntranceAnimations(page)).toEqual([]);
      await page.waitForTimeout(40);
    }
    await expect(page).toHaveURL(/#3$/);
    await expect(fragments.first()).toHaveClass(/fragment-visible/);
    await expect(fragments.last()).toHaveClass(/fragment-hidden/);

    // Revealing the last fragment, then moving on, arrives at the next slide
    // with its entrance animations playing.
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');
    await expect(page).toHaveURL(/#4$/);
    await expect.poll(async () => (await runningEntranceAnimations(page)).length, { timeout: 3000 }).toBeGreaterThan(0);
  });
});
