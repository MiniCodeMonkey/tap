import { test, expect } from '@playwright/test';
import { spawn, type ChildProcess } from 'child_process';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(__dirname, '../..');

/**
 * The hub keeps its last-known slide/fragment state for a retention period
 * after the last client disconnects (internal/server/websocket.go), so
 * reloading the only open window doesn't lose the current fragment. The
 * shared server the rest of frontend/e2e uses runs with
 * TAP_HUB_STATE_RETENTION=0s (see playwright.config.ts), so this spec runs
 * its own dev server, on its own port, with a real non-zero retention.
 */

const PORT = 3402;
const BASE_URL = `http://localhost:${PORT}`;
const RETENTION = '5s';

let server: ChildProcess;

async function waitForServer(): Promise<void> {
	const deadline = Date.now() + 30_000;
	while (Date.now() < deadline) {
		try {
			const response = await fetch(`${BASE_URL}/api/presentation`);
			if (response.ok) return;
		} catch {
			// Not up yet.
		}
		await new Promise((resolve) => setTimeout(resolve, 200));
	}
	throw new Error(`dev server on port ${PORT} did not come up in time`);
}

/** The 1-based slide number of the deck's first fragment-bearing slide, resolved at runtime rather than hardcoded. */
async function findFragmentSlideNumber(): Promise<number> {
	const response = await fetch(`${BASE_URL}/api/presentation`);
	const data = (await response.json()) as { slides: { fragmentCount: number }[] };
	const index = data.slides.findIndex((slide) => slide.fragmentCount > 0);
	if (index === -1) {
		throw new Error('no fragment-bearing slide found in testdata/sample.md');
	}
	return index + 1;
}

test.describe('hub state retention survives a reload of the only window', () => {
	test.beforeAll(async () => {
		server = spawn(
			'go',
			['run', './cmd/tap', 'dev', 'testdata/sample.md', '--port', String(PORT), '--headless'],
			{
				cwd: REPO_ROOT,
				stdio: 'ignore',
				detached: true,
				env: { ...process.env, TAP_HUB_STATE_RETENTION: RETENTION }
			}
		);
		await waitForServer();
	});

	test.afterAll(() => {
		if (server.pid) {
			process.kill(-server.pid, 'SIGKILL');
		}
	});

	test('reveals a fragment, reloads, and lands back on the same slide with the fragment still revealed', async ({
		page
	}) => {
		const slideNumber = await findFragmentSlideNumber();

		await page.goto(`${BASE_URL}/#${slideNumber}`);
		await page.waitForSelector('.slide[data-layout]');
		// Give the websocket a moment to connect and broadcast this slide's
		// initial state before advancing a fragment.
		await page.waitForTimeout(300);

		const firstFragment = page.locator('[data-fragment-index="0"]').first();
		await expect(firstFragment).toHaveClass(/fragment-hidden/);

		await page.keyboard.press('ArrowRight');
		await expect(firstFragment).toHaveClass(/fragment-visible/);
		// Let the slide/fragment state actually reach the server before
		// reloading: BroadcastSlide is fire-and-forget over the socket.
		await page.waitForTimeout(300);

		await page.reload();
		await page.waitForSelector('.slide[data-layout]');

		// Both are web-first assertions: they retry until the reconnect's
		// state message arrives, instead of sleeping a fixed amount first.
		await expect(page).toHaveURL(new RegExp(`#${slideNumber}$`));
		const firstFragmentAfterReload = page.locator('[data-fragment-index="0"]').first();
		await expect(firstFragmentAfterReload).toHaveClass(/fragment-visible/);
	});
});
