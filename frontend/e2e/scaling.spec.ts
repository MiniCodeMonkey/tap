import { test, expect, type Page } from '@playwright/test';

/**
 * Regression coverage for the fixed 1920px-wide scaling canvas: every slide
 * renders at the same absolute layout (1920 x 1920/aspectRatio px) and is
 * then scaled down as one unit to fit whatever container it is shown in, so
 * a deck looks identical on every screen. `.slide-container`'s flex layout
 * lets the frame (`SlideCanvas`'s own `.slide`, a flex child) shrink below
 * its inline size whenever the container is narrower than 1920px, which
 * would distort the aspect ratio and wrap text on any viewport under
 * 1920x1080 - exactly the sizes real laptops, projectors and presenter
 * panels use. This spec covers a representative small, exact-fit, and
 * oversized viewport rather than every possible screen size.
 */

const VIEWPORTS = [
	{ width: 1024, height: 768 },
	{ width: 1920, height: 1080 },
	{ width: 3840, height: 2160 }
];

/** Rendered geometry of the per-slide root and the scale it was drawn at. */
interface SlideGeometry {
	layoutWidthPx: number;
	rect: { top: number; left: number; right: number; bottom: number; width: number; height: number };
	scale: number;
	headingHeightPx: number | null;
}

async function readSlideGeometry(page: Page): Promise<SlideGeometry> {
	return page.evaluate(() => {
		const slide = document.querySelector('.slide[data-layout]');
		if (!slide) throw new Error('no .slide[data-layout] found');
		const frame = slide.closest('.slide-container')?.querySelector(':scope > .slide') as HTMLElement | null;
		if (!frame) throw new Error('no scaling frame found');

		// getComputedStyle(...).width on the per-slide root reports its own
		// *layout* box, unaffected by the frame's scale() transform (a
		// transform never changes layout size, only paint), so this is the
		// same 1920px at every viewport when the fix holds.
		const layoutWidthPx = parseFloat(getComputedStyle(slide).width);

		const rect = frame.getBoundingClientRect();

		const transform = getComputedStyle(frame).transform;
		let scale = 1;
		if (transform && transform !== 'none') {
			// matrix(a, b, c, d, tx, ty); a is the x-scale factor.
			const match = /matrix\(([^,]+),/.exec(transform);
			if (match) scale = parseFloat(match[1]);
		}

		const heading = document.querySelector('.slide[data-layout] h1, .slide[data-layout] h2') as HTMLElement | null;
		const headingHeightPx = heading ? heading.getBoundingClientRect().height / scale : null;

		return {
			layoutWidthPx,
			rect: { top: rect.top, left: rect.left, right: rect.right, bottom: rect.bottom, width: rect.width, height: rect.height },
			scale,
			headingHeightPx
		};
	});
}

/**
 * The frame's own rect (relative to `containerSelector`'s box) and that
 * container's bounds, for any `.slide-container` on the page - the
 * audience canvas, a presenter panel, or an overview thumbnail. The one
 * measurement block every "does this frame scale correctly inside its
 * container" test needs, whatever the container.
 */
async function readFrameWithinContainer(
	page: Page,
	containerSelector: string
): Promise<{ bounds: { width: number; height: number }; rect: SlideGeometry['rect'] } | null> {
	return page.evaluate((selector) => {
		const container = document.querySelector(selector);
		const frame = container?.querySelector(':scope > .slide');
		const containerRect = container?.getBoundingClientRect();
		const frameRect = frame?.getBoundingClientRect();
		if (!containerRect || !frameRect) return null;
		return {
			bounds: { width: containerRect.width, height: containerRect.height },
			rect: {
				top: frameRect.top - containerRect.top,
				left: frameRect.left - containerRect.left,
				right: frameRect.right - containerRect.left,
				bottom: frameRect.bottom - containerRect.top,
				width: frameRect.width,
				height: frameRect.height
			}
		};
	}, containerSelector);
}

/**
 * Common assertions for a rendered slide frame, regardless of what page or
 * container it lives in: it keeps its aspect ratio, fits inside the given
 * bounds, is centered within them, and touches the bounds on at least one
 * axis (it is scaled to fill, not shrunk further than necessary).
 */
function assertFits(rect: SlideGeometry['rect'], bounds: { width: number; height: number }, aspect = 16 / 9): void {
	const measuredAspect = rect.width / rect.height;
	expect(Math.abs(measuredAspect - aspect) / aspect).toBeLessThan(0.01);

	expect(rect.left).toBeGreaterThanOrEqual(-2);
	expect(rect.top).toBeGreaterThanOrEqual(-2);
	expect(rect.right).toBeLessThanOrEqual(bounds.width + 2);
	expect(rect.bottom).toBeLessThanOrEqual(bounds.height + 2);

	const leftGap = rect.left;
	const rightGap = bounds.width - rect.right;
	const topGap = rect.top;
	const bottomGap = bounds.height - rect.bottom;
	expect(Math.abs(leftGap - rightGap)).toBeLessThan(2);
	expect(Math.abs(topGap - bottomGap)).toBeLessThan(2);

	// At least one axis is flush (within 2px): the canvas is scaled to fill
	// its container on the constraining axis, not shrunk further than that.
	const touchesAnAxis = leftGap < 2 || topGap < 2;
	expect(touchesAnAxis).toBe(true);
}

test.describe('fixed 1920px canvas scales as one unit at any screen size', () => {
	for (const viewport of VIEWPORTS) {
		test(`audience view at ${viewport.width}x${viewport.height}: fills the viewport, keeps aspect ratio, and scales text in proportion`, async ({
			page
		}) => {
			// Slide 3 ("Core Features") is a text-heavy bullet slide: the
			// clearest place to see word-wrap caused by a shrunk canvas.
			// Read the heading at the 1920x1080 reference size first - the
			// canvas's own native size - then resize down/up to the target
			// viewport and compare, so a broken scale shows up as the
			// heading's rendered height no longer tracking the frame's.
			await page.setViewportSize({ width: 1920, height: 1080 });
			await page.goto('/#3');
			await page.waitForSelector('.slide[data-layout]');
			await page.evaluate(() => document.fonts.ready);
			const reference = await readSlideGeometry(page);
			expect(reference.headingHeightPx).not.toBeNull();

			await page.setViewportSize(viewport);
			// Let the ResizeObserver-driven scale recalculation settle.
			await page.evaluate(
				() =>
					new Promise<void>((resolve) => {
						requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
					})
			);
			const geometry = await readSlideGeometry(page);

			expect(geometry.layoutWidthPx).toBeCloseTo(1920, 0);
			assertFits(geometry.rect, viewport);

			expect(geometry.headingHeightPx).not.toBeNull();
			const ratio = (geometry.headingHeightPx as number) / (reference.headingHeightPx as number);
			expect(Math.abs(ratio - 1)).toBeLessThan(0.02);
		});
	}

	test('presenter view current-slide panel scales the frame within its own bounds at 1440x900', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto('/presenter#3');
		await page.waitForSelector('.presenter-current-slide-panel .slide[data-layout]');
		await page.evaluate(() => document.fonts.ready);

		const measurements = await readFrameWithinContainer(page, '.presenter-current-slide-panel .slide-container');
		expect(measurements).not.toBeNull();
		if (measurements) {
			assertFits(measurements.rect, measurements.bounds);
		}
	});

	test('presenter view current-slide panel scales the frame within its own bounds at 1024x768', async ({ page }) => {
		await page.setViewportSize({ width: 1024, height: 768 });
		await page.goto('/presenter#3');
		await page.waitForSelector('.presenter-current-slide-panel .slide[data-layout]');
		await page.evaluate(() => document.fonts.ready);

		const measurements = await readFrameWithinContainer(page, '.presenter-current-slide-panel .slide-container');
		expect(measurements).not.toBeNull();
		if (measurements) {
			assertFits(measurements.rect, measurements.bounds);
		}
	});

	test('the letterbox area outside the canvas is always black, not the theme background, at 1024x768', async ({
		page
	}) => {
		// Regression test for the letterbox bars taking the theme's color
		// (green-black in terminal) instead of black: check the computed
		// background of the point at the top center of the page, which sits
		// outside the scaled 1920x1080 canvas at this non-16:9 viewport, for
		// both the base theme and a theme with a strongly colored background.
		await page.setViewportSize({ width: 1024, height: 768 });

		for (const theme of ['base', 'terminal']) {
			const url = theme === 'base' ? '/#3' : '/?theme=terminal#3';
			await page.goto(url);
			await page.waitForSelector('.slide[data-layout]');
			await page.evaluate(() => document.fonts.ready);

			const background = await page.evaluate(() => {
				const element = document.elementFromPoint(window.innerWidth / 2, 5);
				return element ? getComputedStyle(element).backgroundColor : null;
			});

			expect(background, `letterbox background for theme "${theme}"`).toBe('rgb(0, 0, 0)');
		}
	});

	test('an overview thumbnail scales the frame within its small container', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto('/');
		await page.waitForSelector('.slide[data-layout]');
		await page.keyboard.press('o');
		await page.waitForSelector('.slide-overview .thumbnail');

		const measurements = await readFrameWithinContainer(page, '.slide-overview .thumbnail .slide-container');
		expect(measurements).not.toBeNull();
		if (measurements) {
			assertFits(measurements.rect, measurements.bounds);
		}
	});
});
