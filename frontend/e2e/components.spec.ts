import { test, expect } from '@playwright/test';
import { spawn, type ChildProcess } from 'child_process';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(__dirname, '../..');

/**
 * Deck component end-to-end coverage, against its own dev servers (never
 * the shared one from playwright.config.ts): port 3410 for
 * examples/components/deck.md, port 3411 for
 * testdata/components-broken/deck.md, port 3412 for
 * testdata/components-fade/deck.md. TAP_HUB_STATE_RETENTION=0s keeps
 * each server's websocket hub from carrying slide/step state between
 * tests, the same way playwright.config.ts's shared server does.
 */

const EXAMPLE_PORT = 3410;
const EXAMPLE_URL = `http://localhost:${EXAMPLE_PORT}`;
const BROKEN_PORT = 3411;
const BROKEN_URL = `http://localhost:${BROKEN_PORT}`;
const FADE_PORT = 3412;
const FADE_URL = `http://localhost:${FADE_PORT}`;

// Slide numbers in examples/components/deck.md.
const ROLLING_DEPLOY_SLIDE = 3;
const LATENCY_CHART_SLIDE = 4;
const TWO_CHARTS_SLIDE = 5;

async function waitForServer(baseURL: string): Promise<void> {
	const deadline = Date.now() + 30_000;
	while (Date.now() < deadline) {
		try {
			const response = await fetch(`${baseURL}/api/presentation`);
			if (response.ok) return;
		} catch {
			// Not up yet.
		}
		await new Promise((r) => setTimeout(r, 200));
	}
	throw new Error(`dev server at ${baseURL} did not come up in time`);
}

function startDevServer(deck: string, port: number): ChildProcess {
	return spawn('go', ['run', './cmd/tap', 'dev', deck, '--port', String(port), '--headless'], {
		cwd: REPO_ROOT,
		stdio: 'ignore',
		detached: true,
		env: { ...process.env, TAP_HUB_STATE_RETENTION: '0s' }
	});
}

function stopDevServer(server: ChildProcess): void {
	if (server.pid) {
		process.kill(-server.pid, 'SIGKILL');
	}
}

test.describe('Deck components', () => {
	let exampleServer: ChildProcess;
	let brokenServer: ChildProcess;
	let fadeServer: ChildProcess;

	test.beforeAll(async () => {
		exampleServer = startDevServer('examples/components/deck.md', EXAMPLE_PORT);
		brokenServer = startDevServer('testdata/components-broken/deck.md', BROKEN_PORT);
		fadeServer = startDevServer('testdata/components-fade/deck.md', FADE_PORT);
		await Promise.all([waitForServer(EXAMPLE_URL), waitForServer(BROKEN_URL), waitForServer(FADE_URL)]);
	});

	test.afterAll(() => {
		stopDevServer(exampleServer);
		stopDevServer(brokenServer);
		stopDevServer(fadeServer);
	});

	test('a whole-slide component plays its own mount animation on the first render of the slide it loads on', async ({
		page
	}) => {
		// Regression test: SlideTransition wraps slides in
		// <AnimatePresence initial={false}>, which reaches every motion
		// element beneath it through presence context, including inside a
		// deck component - without a reset, that would skip a component's
		// own mount animation (e.g. a fade-in), so the page would load
		// straight into the animation's end state instead of playing it.
		// DeckComponent resets presence context to null for the component's
		// own tree, so its `initial` prop always governs.
		const start = Date.now();
		await page.goto(`${FADE_URL}/#1`);
		const target = page.locator('[data-testid="fade-target"]');
		await target.waitFor();

		const early = await target.evaluate((el) => parseFloat(getComputedStyle(el).opacity));
		expect(early).toBeLessThan(0.9);

		// The fixture's fade is 1s; wait past it from the same start point
		// this test measured "early" from, then confirm it settled.
		const elapsed = Date.now() - start;
		if (elapsed < 1500) await page.waitForTimeout(1500 - elapsed);
		const late = await target.evaluate((el) => parseFloat(getComputedStyle(el).opacity));
		expect(late).toBeCloseTo(1, 1);
	});

	test('ArrowRight advances the whole-slide component through its steps before the deck moves on', async ({
		page
	}) => {
		await page.goto(`${EXAMPLE_URL}/#${ROLLING_DEPLOY_SLIDE}`);
		await page.waitForSelector('[data-testid="server-0"]');

		// Step 0: every server is still on v1.
		await expect(page.locator('[data-testid="server-0"]')).toHaveAttribute('data-version', 'v1');
		await expect(page.locator('[data-testid="server-3"]')).toHaveAttribute('data-version', 'v1');

		// Steps 1 and 2 roll server 0 and then server 1 to v2. Each press
		// consumes a step and stays on the same slide; toHaveURL/toHaveAttribute
		// poll on their own, so no manual sleep is needed to let the deck or
		// the component's internal drain/restart timers (~950ms) settle.
		await page.keyboard.press('ArrowRight');
		await expect(page).toHaveURL(new RegExp(`#${ROLLING_DEPLOY_SLIDE}$`));

		await page.keyboard.press('ArrowRight');
		await expect(page).toHaveURL(new RegExp(`#${ROLLING_DEPLOY_SLIDE}$`));
		await expect(page.locator('[data-testid="server-0"]')).toHaveAttribute('data-version', 'v2', { timeout: 5000 });
		await expect(page.locator('[data-testid="server-1"]')).toHaveAttribute('data-version', 'v2', { timeout: 5000 });
		await expect(page.locator('[data-testid="server-2"]')).toHaveAttribute('data-version', 'v1');

		// Steps 3, 4, 5 finish the rollout; step 5 is the slide's last step.
		// No wait between presses: each one only changes the target step, and
		// the assertions below poll until whichever timer is still running
		// from the last press has settled.
		await page.keyboard.press('ArrowRight');
		await page.keyboard.press('ArrowRight');
		await page.keyboard.press('ArrowRight');
		await expect(page).toHaveURL(new RegExp(`#${ROLLING_DEPLOY_SLIDE}$`));
		for (let index = 0; index < 4; index++) {
			await expect(page.locator(`[data-testid="server-${index}"]`)).toHaveAttribute('data-version', 'v2', {
				timeout: 5000
			});
		}

		// One more ArrowRight has no step left to consume, so the deck moves on.
		await page.keyboard.press('ArrowRight');
		await expect(page).toHaveURL(new RegExp(`#${ROLLING_DEPLOY_SLIDE + 1}$`));
	});

	test('the presenter view mirrors the component step', async ({ browser }) => {
		const viewerContext = await browser.newContext();
		const viewerPage = await viewerContext.newPage();
		await viewerPage.goto(`${EXAMPLE_URL}/#${ROLLING_DEPLOY_SLIDE}`);
		await viewerPage.waitForSelector('[data-testid="server-0"]');

		const presenterContext = await browser.newContext();
		const presenterPage = await presenterContext.newPage();
		await presenterPage.goto(`${EXAMPLE_URL}/presenter#${ROLLING_DEPLOY_SLIDE}`);
		await presenterPage.waitForSelector('.presenter-view');

		// No DOM state exists yet to assert on for "the websocket has
		// connected and broadcast its initial slide state" - the same
		// reasoning frontend/e2e/hub-retention.spec.ts documents - so this
		// sleep cannot be expressed as a web-first assertion.
		await viewerPage.waitForTimeout(500);
		await presenterPage.waitForTimeout(500);

		await viewerPage.keyboard.press('ArrowRight');

		const presenterServerZero = presenterPage.locator('.presenter-current-slide-panel [data-testid="server-0"]');
		await expect(presenterServerZero).toHaveAttribute('data-version', 'v2', { timeout: 5000 });

		await viewerContext.close();
		await presenterContext.close();
	});

	test('print mode shows the final state for the whole-slide and the inline component', async ({ page }) => {
		await page.goto(`${EXAMPLE_URL}/?print=true#${ROLLING_DEPLOY_SLIDE}`);
		await page.waitForSelector('[data-testid="server-0"]');
		for (let index = 0; index < 4; index++) {
			await expect(page.locator(`[data-testid="server-${index}"]`)).toHaveAttribute('data-version', 'v2');
			await expect(page.locator(`[data-testid="server-${index}"]`)).toHaveAttribute('data-state', 'serving');
		}

		await page.goto(`${EXAMPLE_URL}/?print=true#${LATENCY_CHART_SLIDE}`);
		await page.waitForSelector('.deck-component-root');
		await expect(page.locator('.deck-component-root').first()).toContainText('88ms');
	});

	test('the broken deck shows an error card on the broken slide and ArrowRight still reaches the slide after it', async ({
		page
	}) => {
		await page.goto(`${BROKEN_URL}/#1`);
		await page.waitForSelector('.slide-container');
		await expect(page.locator('.slide-content')).toContainText('Before the broken slide');

		await page.keyboard.press('ArrowRight');
		await expect(page).toHaveURL(/#2$/);
		await expect(page.locator('.deck-error-card')).toBeVisible();
		await expect(page.locator('.deck-error-card')).toHaveAttribute('data-source', /Broken\.jsx/);

		await page.keyboard.press('ArrowRight');
		await expect(page).toHaveURL(/#3$/);
		await expect(page.locator('.slide-content')).toContainText('After the broken slide');
	});

	test('the inline component gets its own JSON props: two instances on one slide show different numbers', async ({
		page
	}) => {
		await page.goto(`${EXAMPLE_URL}/#${TWO_CHARTS_SLIDE}`);
		await page.waitForSelector('.deck-component-root');

		const charts = page.locator('.deck-component-root');
		await expect(charts).toHaveCount(2);
		await expect(charts.nth(0)).toContainText('530ms');
		await expect(charts.nth(1)).toContainText('410ms');
	});
});
