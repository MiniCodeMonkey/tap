import { test, expect, type Page } from '@playwright/test';
import { spawn, type ChildProcess } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
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

/**
 * A deck component animates in with Motion, which replays whenever the
 * component mounts again. Runs against its own dev server on a scratch copy
 * of examples/components (so the example itself is never edited), with
 * `transition: none` so navigating keeps the same Slide mounted, the path
 * where a reused component would otherwise go unnoticed.
 */
test.describe('Live update deck components', () => {
  const port = 3414;
  const baseURL = `http://localhost:${port}`;
  const latencyChartSlide = 4;
  let deckDirectory: string;
  let deckPath: string;
  let deckSource: string;
  let server: ChildProcess;

  test.beforeAll(async () => {
    deckDirectory = fs.mkdtempSync(path.join(os.tmpdir(), 'tap-live-update-'));
    fs.cpSync(path.join(dirname, '../../examples/components'), deckDirectory, { recursive: true });
    deckPath = path.join(deckDirectory, 'deck.md');
    deckSource = fs.readFileSync(deckPath, 'utf-8').replace(/^transition: .*$/m, 'transition: none');
    fs.writeFileSync(deckPath, deckSource);
    server = spawn('go', ['run', './cmd/tap', 'dev', deckPath, '--port', String(port), '--headless'], {
      cwd: path.join(dirname, '../..'),
      stdio: 'ignore',
      detached: true,
      env: { ...process.env, TAP_HUB_STATE_RETENTION: '0s' },
    });
    const deadline = Date.now() + 60_000;
    while (Date.now() < deadline) {
      try {
        if ((await fetch(`${baseURL}/api/presentation`)).ok) return;
      } catch {
        // Not up yet.
      }
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
    throw new Error(`dev server at ${baseURL} did not come up in time`);
  });

  test.afterAll(() => {
    if (server?.pid) process.kill(-server.pid, 'SIGKILL');
    fs.rmSync(deckDirectory, { recursive: true, force: true });
  });

  /** The chart's first bar: marks it, or reports whether it is still the marked element, and its height. */
  function firstBar(page: Page, mark = false): Promise<{ marked: boolean; height: number }> {
    return page.evaluate((shouldMark) => {
      const bar = document.querySelector<HTMLElement & { liveUpdateMark?: boolean }>(
        '.deck-component-root div[style*="height"]'
      );
      if (!bar) return { marked: false, height: -1 };
      if (shouldMark) bar.liveUpdateMark = true;
      return { marked: bar.liveUpdateMark === true, height: bar.getBoundingClientRect().height };
    }, mark);
  }

  test('a text edit keeps the component mounted without replaying its entrance, and navigating replays it', async ({
    page,
  }) => {
    await page.goto(`${baseURL}/#${latencyChartSlide}`);
    await expect(page.locator('.slide-content')).toContainText('What It Bought Us');
    // The bars grow in over half a second; let them finish.
    await expect.poll(async () => (await firstBar(page)).height, { timeout: 5000 }).toBeGreaterThan(100);
    await page.waitForTimeout(700);
    const settled = await firstBar(page, true);

    fs.writeFileSync(deckPath, deckSource.replace('## What It Bought Us', '## What It Bought Us, Measured'));
    await expect(page.locator('.slide-content')).toContainText('What It Bought Us, Measured', { timeout: 10000 });

    // Sampled over a few frames, well inside the bars' entrance duration.
    for (let sample = 0; sample < 5; sample++) {
      const bar = await firstBar(page);
      expect(bar.marked).toBe(true);
      expect(bar.height).toBeCloseTo(settled.height, 0);
      await page.waitForTimeout(40);
    }

    // The next slide shows the same component file: it mounts fresh and grows in.
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');
    await expect(page).toHaveURL(new RegExp(`#${latencyChartSlide + 1}$`));
    await expect.poll(async () => (await firstBar(page)).marked, { timeout: 3000 }).toBe(false);
    expect((await firstBar(page)).height).toBeLessThan(settled.height / 2);
  });
});
