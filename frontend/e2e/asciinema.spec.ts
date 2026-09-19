import { test, expect } from '@playwright/test';
import { spawn, type ChildProcess } from 'child_process';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(__dirname, '../..');

/**
 * The asciinema player is bundled (frontend/src/lib/utils/asciinema.ts), not
 * loaded from a CDN, precisely because tap is used on stage with unreliable
 * network. This spec is the end-to-end proof: it runs its own dev server
 * (testdata/themes.md, which has an asciinema slide, on a port of its own so
 * it doesn't collide with the shared server the rest of frontend/e2e uses),
 * blocks every network request whose host isn't localhost, and confirms the
 * asciinema slide still renders and plays.
 */

const PORT = 3401;
const BASE_URL = `http://localhost:${PORT}`;

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

/** The 1-based slide number of the deck's asciinema slide, resolved at runtime by content rather than a hardcoded index, so a deck edit can't silently point this spec at the wrong slide. */
async function findAsciinemaSlideNumber(): Promise<number> {
	const response = await fetch(`${BASE_URL}/api/presentation`);
	const data = (await response.json()) as { slides: { html: string; slots: Record<string, string> }[] };
	const index = data.slides.findIndex((slide) => {
		const blob = slide.html + Object.values(slide.slots).join('');
		return blob.includes('language-asciinema');
	});
	if (index === -1) {
		throw new Error('no asciinema slide found in testdata/themes.md');
	}
	return index + 1;
}

test.describe('asciinema player (bundled, no CDN, network blocked)', () => {
	test.beforeAll(async () => {
		// `go run` spawns the compiled binary as its own child process, which
		// `server.kill()` alone would leave running (and the port held) after
		// this spec exits. `detached: true` puts the server in its own
		// process group so afterAll can kill that whole group.
		server = spawn('go', ['run', './cmd/tap', 'dev', 'testdata/themes.md', '--port', String(PORT), '--headless'], {
			cwd: REPO_ROOT,
			stdio: 'ignore',
			detached: true
		});
		await waitForServer();
	});

	test.afterAll(() => {
		if (server.pid) {
			process.kill(-server.pid, 'SIGKILL');
		}
	});

	test('plays with every non-localhost network request blocked', async ({ page }) => {
		test.setTimeout(30_000);

		const blockedHosts: string[] = [];
		await page.route('**/*', (route) => {
			const url = new URL(route.request().url());
			const isLocal = url.hostname === 'localhost' || url.hostname === '127.0.0.1';
			if (!isLocal) {
				blockedHosts.push(url.hostname);
				void route.abort();
				return;
			}
			void route.continue();
		});

		const slideNumber = await findAsciinemaSlideNumber();
		await page.goto(`${BASE_URL}/?print=true#${slideNumber}`);
		await page.waitForSelector('.slide[data-layout]');

		const wrapper = page.locator('.asciinema-player-wrapper');
		await expect(wrapper).toBeVisible();
		// An error state (missing src, failed player creation, or the old
		// CDN-load failure this spec guards against) renders
		// .asciinema-player-wrapper.error instead of the real player.
		await expect(page.locator('.asciinema-player-wrapper.error')).toHaveCount(0);

		// The real asciinema-player renders its terminal grid (.ap-terminal)
		// inside the outer .ap-player container once it has loaded and parsed
		// the recording. Assert on .ap-terminal alone: an "or" locator
		// matching both elements trips Playwright's strict mode, since
		// .ap-terminal nests inside .ap-player and both are present at once.
		await expect(wrapper.locator('.ap-terminal')).toBeVisible({ timeout: 10_000 });

		// The player renders and plays with zero non-local requests: nothing
		// is blocked, because nothing ever leaves localhost.
		expect(blockedHosts).toEqual([]);
	});
});
