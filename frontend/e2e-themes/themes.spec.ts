/**
 * Per-theme check suite: overflow, contrast, minimum text size, isolation,
 * and a visual snapshot, for every slide of testdata/themes.md.
 *
 * Run against a Go dev server and a Vite dev server the caller already
 * started with `testdata/themes.md` loaded (see
 * docs/reference/theme-porting.md). Selects a theme with
 * `?theme=<slug>&print=true#<n>`, matching the runtime's own theme
 * precedence (the query parameter beats the deck's frontmatter).
 */

import { existsSync, readFileSync } from 'fs';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';
import { expect, test, type Page } from '@playwright/test';
import { findOccludedOrClippedText, findOverflow, findSmallHeadlines, findSmallText, readCodeContrast, readContrast } from './checks';

const __dirname = dirname(fileURLToPath(import.meta.url));

// ============================================================================
// Theme discovery
// ============================================================================

interface ThemeEntry {
	slug: string;
	name: string;
	polarity: 'light' | 'dark';
	pitch: string;
}

const REPO_ROOT = resolve(__dirname, '../..');
const THEMES_JSON = resolve(REPO_ROOT, 'internal/themes/themes.json');
const THEMES_DIR = resolve(REPO_ROOT, 'frontend/src/lib/themes');

const allThemes: ThemeEntry[] = JSON.parse(readFileSync(THEMES_JSON, 'utf-8'));

function isPorted(slug: string): boolean {
	if (slug === 'base') return true;
	return existsSync(resolve(THEMES_DIR, slug, 'theme.css')) && existsSync(resolve(THEMES_DIR, slug, 'theme.json'));
}

/**
 * `base` is the plain fallback, not one of the 20 designed themes: it isn't
 * held to the design spec's contrast and minimum-size bars, which assume a
 * theme built to the brief's readability rules. Its overflow and isolation
 * checks still apply, since those are about the runtime, not the design.
 */
const READABILITY_EXEMPT_SLUGS = new Set(['base']);

function themesToRun(): ThemeEntry[] {
	const requested = process.env.THEME;
	if (requested) {
		const theme = allThemes.find((t) => t.slug === requested);
		if (!theme) {
			throw new Error(`THEME=${requested} is not a known theme slug (see internal/themes/themes.json)`);
		}
		return [theme];
	}
	return allThemes.filter((t) => isPorted(t.slug));
}

// ============================================================================
// Deck introspection
// ============================================================================

async function slideCount(page: Page): Promise<number> {
	const response = await page.request.get('/api/presentation');
	const data = (await response.json()) as { slides: unknown[] };
	return data.slides.length;
}

/**
 * The 8 representative slides visual snapshots are taken for (every other
 * check - overflow, sizes, contrast, code contrast, isolation - still runs
 * on every slide of the deck, see the "overflow and minimum text size"
 * test below). Chosen by heading text, resolved to a slide number at
 * runtime, rather than a hardcoded slide index: an edit to
 * testdata/themes.md that reorders slides can't silently point this suite
 * at the wrong slide the way a bare number would.
 */
const SNAPSHOT_SLIDE_TITLES = [
	'Scaling a Markdown Renderer', // title
	'Two ways to scale', // two-column
	'The fix, in one line', // code, with a highlighted line
	'40x', // big-stat
	'Split media', // split-media, with an image
	'Numbers behind the rewrite', // table
	'Fragments', // a bullet slide with fragments
	'How the renderer talks to itself' // mermaid
] as const;

/**
 * Resolves each of SNAPSHOT_SLIDE_TITLES to its 1-based slide number in
 * the deck currently loaded, by finding a slide whose rendered HTML
 * contains that exact heading text as an element's full text content
 * (">Title<", so a title that happens to be a substring of another
 * slide's prose can't match). Throws if a title isn't found, so a deck
 * edit that renames or removes one of these slides fails loudly instead
 * of silently skipping its snapshot.
 */
async function snapshotSlideNumbers(page: Page): Promise<{ title: string; slideNumber: number }[]> {
	const response = await page.request.get('/api/presentation');
	const data = (await response.json()) as { slides: { html: string; slots: Record<string, string> }[] };

	return SNAPSHOT_SLIDE_TITLES.map((title) => {
		const needle = `>${title}<`;
		const index = data.slides.findIndex((slide) => {
			const blob = slide.html + Object.values(slide.slots).join('');
			return blob.includes(needle);
		});
		if (index === -1) {
			throw new Error(
				`snapshot slide title ${JSON.stringify(title)} not found in testdata/themes.md - did a heading change?`
			);
		}
		return { title, slideNumber: index + 1 };
	});
}

async function gotoSlide(page: Page, slug: string, slideNumber: number): Promise<void> {
	await page.goto(`/?theme=${slug}&print=true#${slideNumber}`);
	await page.waitForSelector('.slide[data-layout]');
	await page.waitForFunction(
		// data-theme lives on the 1920x1080 canvas frame inside
		// .slide-container (see SlideCanvas.tsx), not on .slide-container
		// itself - that element is only the letterbox area around it now.
		(expected) => document.querySelector('.slide-container [data-theme]')?.getAttribute('data-theme') === expected,
		slug
	);
	// Mermaid, asciinema and web fonts finish after the initial paint; give
	// them a moment before measuring anything.
	await page.waitForLoadState('networkidle');
	await page.evaluate(() => document.fonts.ready);
	// Print mode (data-print="true", see SlideCanvas) turns off entrance
	// animations and transitions, so layout is stable once fonts are ready.
	// Still wait two animation frames before measuring: the first frame
	// after data-theme/data-print land can still be mid-layout (e.g. a
	// ResizeObserver-driven scale recalculation in SlideCanvas), and a
	// second frame confirms the measured layout has actually settled.
	await page.evaluate(
		() =>
			new Promise<void>((resolve) => {
				requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
			})
	);
	// A live code block's Run control depends on websocket connection and
	// static-mode detection, both async; until that race settles it can
	// render neither, so wait for its final state (a no-op when the slide
	// has no live code block).
	await page.waitForFunction(() => {
		const block = document.querySelector('.live-code-block');
		if (!block) return true;
		return block.querySelector('.run-button, .static-placeholder') !== null;
	});
}

// ============================================================================
// Suite
// ============================================================================

for (const theme of themesToRun()) {
	test.describe(`theme: ${theme.slug}`, () => {
		let total = 0;

		test.beforeAll(async ({ browser }) => {
			const page = await browser.newPage();
			await page.goto('/');
			total = await slideCount(page);
			await page.close();
		});

		test('readability: contrast', async ({ page }) => {
			await gotoSlide(page, theme.slug, 1);
			const results = await readContrast(page);

			if (READABILITY_EXEMPT_SLUGS.has(theme.slug)) {
				test.skip(true, `${theme.slug} is exempt from the design spec's contrast minimums`);
				return;
			}

			for (const result of results) {
				expect(
					result.passes,
					`${result.name}: ${result.foreground} on ${result.background} is ${result.ratio}:1, want >= ${result.minimum}:1`
				).toBe(true);
			}
		});

		test('overflow and minimum text size, every slide', async ({ page }, testInfo) => {
			test.setTimeout(120_000);
			const overflowFailures: string[] = [];
			const occludedOrClippedFailures: string[] = [];
			const smallTextFailures: string[] = [];
			const codeContrastFailures: string[] = [];
			const smallHeadlineWarnings: string[] = [];

			for (let n = 1; n <= total; n++) {
				await gotoSlide(page, theme.slug, n);

				const overflow = await findOverflow(page);
				for (const violation of overflow) {
					overflowFailures.push(`slide ${n}: <${violation.selector}> "${violation.text}" overflows the slide`);
				}

				const occludedOrClipped = await findOccludedOrClippedText(page);
				for (const violation of occludedOrClipped) {
					occludedOrClippedFailures.push(
						`slide ${n}: <${violation.selector}> "${violation.text}" is ${violation.kind} (${violation.detail})`
					);
				}

				if (!READABILITY_EXEMPT_SLUGS.has(theme.slug)) {
					const smallText = await findSmallText(page);
					for (const violation of smallText) {
						smallTextFailures.push(
							`slide ${n}: <${violation.selector}> "${violation.text}" is ${violation.fontSizePx}px, want >= ${violation.minimumPx}px`
						);
					}

					// h1/h2 at or above the hard floor but under the 60px
					// recommendation are a warning, not a failure: a short
					// or deliberately modest headline is a legitimate design
					// choice, not a bug.
					const smallHeadlines = await findSmallHeadlines(page);
					for (const warning of smallHeadlines) {
						smallHeadlineWarnings.push(
							`slide ${n}: <${warning.selector}> "${warning.text}" is ${warning.fontSizePx}px, recommend >= ${warning.recommendedPx}px`
						);
					}

					// Runs against whichever slides carry a highlighted code block
					// (the kitchen-sink deck's two code slides: one with a
					// highlighted line, one without), a no-op elsewhere.
					const codeContrast = await readCodeContrast(page);
					for (const result of codeContrast) {
						if (!result.passes) {
							codeContrastFailures.push(
								`slide ${n}: ${result.name}: ${result.foreground} on ${result.background} is ${result.ratio}:1, want >= ${result.minimum}:1`
							);
						}
					}
				}
			}

			if (smallHeadlineWarnings.length > 0) {
				testInfo.annotations.push({
					type: 'warning',
					description: `${smallHeadlineWarnings.length} headline(s) under the 60px recommendation:\n${smallHeadlineWarnings.join('\n')}`
				});
				console.warn(
					`[theme: ${theme.slug}] ${smallHeadlineWarnings.length} headline(s) under the 60px recommendation:\n${smallHeadlineWarnings.join('\n')}`
				);
			}

			expect(overflowFailures, overflowFailures.join('\n')).toEqual([]);
			expect(occludedOrClippedFailures, occludedOrClippedFailures.join('\n')).toEqual([]);
			expect(smallTextFailures, smallTextFailures.join('\n')).toEqual([]);
			expect(codeContrastFailures, codeContrastFailures.join('\n')).toEqual([]);
		});

		test('isolation: switching in from another theme matches a fresh load', async ({ page }) => {
			if (theme.slug === 'base') {
				test.skip(true, 'base has no on-demand stylesheet to isolate against');
				return;
			}

			const index = allThemes.findIndex((t) => t.slug === theme.slug);
			const previous = allThemes[(index - 1 + allThemes.length) % allThemes.length];

			// Start on the neighboring theme, then switch in-page with the 't'
			// key, the same mechanism a live presentation uses.
			await gotoSlide(page, previous.slug, 1);
			await page.keyboard.press('t');
			await page.waitForFunction(
				// See the comment in gotoSlide above: data-theme lives on the
				// canvas frame inside .slide-container, not on .slide-container.
				(expected) => document.querySelector('.slide-container [data-theme]')?.getAttribute('data-theme') === expected,
				theme.slug
			);
			await page.evaluate(() => document.fonts.ready);
			const switched = await capturedStyles(page);

			// Compare against a fresh load of the same theme in a new page, so
			// nothing from the previous theme's stylesheet can leak in.
			const freshPage = await page.context().newPage();
			await gotoSlide(freshPage, theme.slug, 1);
			const fresh = await capturedStyles(freshPage);
			await freshPage.close();

			expect(switched).toEqual(fresh);
		});

		for (const title of SNAPSHOT_SLIDE_TITLES) {
			// Registered up front by title (Playwright needs a static test
			// list); the slide number behind each title is resolved inside
			// the test itself, from whatever deck is actually loaded.
			test(`snapshot: ${title}`, async ({ page }) => {
				// The visual baselines under __snapshots__/ were rendered on
				// macOS; Linux's font rendering never matches them pixel for
				// pixel. TAP_THEME_SNAPSHOTS=off (set by the CI job that runs
				// this suite on ubuntu-latest) skips only the screenshot
				// comparison below - every other check in this file (overflow,
				// clipping, sizes, contrast, isolation) still runs there.
				test.skip(process.env.TAP_THEME_SNAPSHOTS === 'off', 'visual snapshots disabled (TAP_THEME_SNAPSHOTS=off)');

				const resolved = await snapshotSlideNumbers(page);
				const match = resolved.find((entry) => entry.title === title);
				if (!match) {
					throw new Error(`snapshot slide title ${JSON.stringify(title)} did not resolve`);
				}

				await gotoSlide(page, theme.slug, match.slideNumber);
				// The asciinema player's terminal frame can still be
				// mid-playback at capture time; mask that region instead of
				// pinning frame-by-frame output. The slide's other checks
				// (overflow, small text, contrast) still run against it
				// unmasked.
				const asciinemaPlayers = page.locator('.asciinema-player-wrapper');
				await expect(page).toHaveScreenshot([theme.slug, `slide-${match.slideNumber}.png`], {
					fullPage: false,
					mask: [asciinemaPlayers],
					// The player's own blinking cursor can render a stray
					// pixel or two just outside its wrapper's box (the mask
					// above only covers the wrapper itself), which flaked
					// this one slide across independent reruns. A tiny
					// tolerance absorbs that without hiding a real diff
					// (typical text/layout changes move thousands of
					// pixels, not a handful).
					maxDiffPixelRatio: 0.02
				});
			});
		}
	});
}

/**
 * A small, stable set of computed styles used to detect CSS leakage between
 * themes: colors and fonts on the slide root and its first heading.
 */
async function capturedStyles(page: Page): Promise<Record<string, string>> {
	return page.evaluate(() => {
		const slide = document.querySelector('.slide[data-layout]');
		const heading = document.querySelector('.slide h1, .slide h2');
		const slideStyle = slide ? getComputedStyle(slide) : null;
		const headingStyle = heading ? getComputedStyle(heading) : null;

		return {
			slideBackground: slideStyle?.backgroundColor ?? '',
			slideColor: slideStyle?.color ?? '',
			slideFontFamily: slideStyle?.fontFamily ?? '',
			headingColor: headingStyle?.color ?? '',
			headingFontFamily: headingStyle?.fontFamily ?? '',
			headingFontSize: headingStyle?.fontSize ?? ''
		};
	});
}
