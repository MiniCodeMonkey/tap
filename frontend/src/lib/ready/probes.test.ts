import { afterEach, describe, expect, it, vi } from 'vitest';
import { resetBlockersForTests } from './blockers';
import { ANIMATION_TIMEOUT_MS, IMAGE_TIMEOUT_MS, PAINT_TIMEOUT_MS, STYLESHEET_TIMEOUT_MS, createDomProbes } from './probes';
import { clearReady, startReadyCycle, type ReadyPayload } from './readySignal';

function track(promise: Promise<void>): { done: () => boolean } {
	let finished = false;
	void promise.then(() => {
		finished = true;
	});
	return { done: () => finished };
}

afterEach(() => {
	vi.useRealTimers();
	vi.unstubAllGlobals();
	document.body.innerHTML = '';
	document.head.querySelectorAll('link').forEach((link) => link.remove());
});

describe('images', () => {
	it('waits for an image that has not loaded, until it loads', async () => {
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: false, configurable: true });
		document.body.appendChild(image);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false, requirePaint: false }).images());
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		image.dispatchEvent(new Event('load'));
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('counts a broken image as done', async () => {
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: false, configurable: true });
		document.body.appendChild(image);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false, requirePaint: false }).images());
		image.dispatchEvent(new Event('error'));
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('gives up on an image after IMAGE_TIMEOUT_MS', async () => {
		vi.useFakeTimers();
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: false, configurable: true });
		document.body.appendChild(image);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false, requirePaint: false }).images());
		await vi.advanceTimersByTimeAsync(IMAGE_TIMEOUT_MS - 1);
		expect(waiting.done()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(waiting.done()).toBe(true);
	});

	it('does not wait for an image that is complete', async () => {
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: true, configurable: true });
		document.body.appendChild(image);

		await expect(createDomProbes({ includeInfiniteAnimations: false, requirePaint: false }).images()).resolves.toBeUndefined();
	});
});

describe('stylesheets and fonts', () => {
	it('waits for a stylesheet to load, and then counts it as settled', async () => {
		const link = document.createElement('link');
		link.rel = 'stylesheet';
		document.head.appendChild(link);
		const probes = createDomProbes({ includeInfiniteAnimations: false, requirePaint: false });
		expect(probes.settledNow()).toBe(false);

		const waiting = track(probes.fonts());
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		link.dispatchEvent(new Event('load'));
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
		expect(probes.settledNow()).toBe(true);
	});

	it('gives up on a stylesheet after STYLESHEET_TIMEOUT_MS', async () => {
		vi.useFakeTimers();
		const link = document.createElement('link');
		link.rel = 'stylesheet';
		document.head.appendChild(link);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false, requirePaint: false }).fonts());
		await vi.advanceTimersByTimeAsync(STYLESHEET_TIMEOUT_MS);
		expect(waiting.done()).toBe(true);
	});

	it('waits for document.fonts.ready, and is not settled while fonts load', async () => {
		let resolveFonts!: () => void;
		const fonts = {
			status: 'loading',
			ready: new Promise<void>((resolve) => {
				resolveFonts = resolve;
			})
		};
		const fakeDocument = { querySelectorAll: () => [], fonts } as unknown as Document;
		const probes = createDomProbes({ includeInfiniteAnimations: false, requirePaint: false, document: fakeDocument });
		expect(probes.settledNow()).toBe(false);

		const waiting = track(probes.fonts());
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		fonts.status = 'loaded';
		resolveFonts();
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
		expect(probes.settledNow()).toBe(true);
	});

	it('gives up on document.fonts.ready after STYLESHEET_TIMEOUT_MS, and is still not settled', async () => {
		vi.useFakeTimers();
		const fonts = { status: 'loading', ready: new Promise<void>(() => {}) };
		const fakeDocument = { querySelectorAll: () => [], fonts } as unknown as Document;
		const probes = createDomProbes({ includeInfiniteAnimations: false, requirePaint: false, document: fakeDocument });

		const waiting = track(probes.fonts());
		await vi.advanceTimersByTimeAsync(STYLESHEET_TIMEOUT_MS - 1);
		expect(waiting.done()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(waiting.done()).toBe(true);
		expect(probes.settledNow()).toBe(false);
	});
});

describe('animations', () => {
	function fakeAnimation(endTime: number): { animation: unknown; finish: () => void } {
		let finish!: () => void;
		const finished = new Promise<void>((resolve) => {
			finish = resolve;
		});
		return { animation: { effect: { getComputedTiming: () => ({ endTime }) }, finished }, finish };
	}

	function documentWith(animations: unknown[]): Document {
		return { querySelectorAll: () => [], getAnimations: () => animations } as unknown as Document;
	}

	it('waits for a running animation to finish', async () => {
		const running = fakeAnimation(400);
		const waiting = track(
			createDomProbes({ includeInfiniteAnimations: false, requirePaint: false, document: documentWith([running.animation]) }).animations()
		);
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		running.finish();
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('ignores an animation that repeats forever on a live page', async () => {
		const looping = fakeAnimation(Infinity);
		await expect(
			createDomProbes({ includeInfiniteAnimations: false, requirePaint: false, document: documentWith([looping.animation]) }).animations()
		).resolves.toBeUndefined();
	});

	it('waits up to ANIMATION_TIMEOUT_MS for an animation that repeats forever on a print page', async () => {
		vi.useFakeTimers();
		const looping = fakeAnimation(Infinity);
		const waiting = track(
			createDomProbes({ includeInfiniteAnimations: true, requirePaint: true, document: documentWith([looping.animation]) }).animations()
		);
		await vi.advanceTimersByTimeAsync(ANIMATION_TIMEOUT_MS - 1);
		expect(waiting.done()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(waiting.done()).toBe(true);
	});
});

describe('paint', () => {
	/** An animation frame queue the test runs by hand, as a browser does when it draws. */
	function controllableFrames(): { drawFrame: () => void; pending: () => number } {
		let callbacks: FrameRequestCallback[] = [];
		vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
			callbacks.push(callback);
			return callbacks.length;
		});
		return {
			drawFrame: () => {
				const due = callbacks;
				callbacks = [];
				for (const callback of due) {
					callback(0);
				}
			},
			pending: () => callbacks.length
		};
	}

	/** A document whose visibility the test controls, with working event listeners. */
	function documentWithVisibility(state: DocumentVisibilityState): {
		document: Document;
		hide: () => void;
	} {
		const events = new EventTarget();
		const target = {
			querySelectorAll: () => [],
			visibilityState: state,
			addEventListener: events.addEventListener.bind(events),
			removeEventListener: events.removeEventListener.bind(events)
		};
		return {
			document: target as unknown as Document,
			hide: () => {
				target.visibilityState = 'hidden';
				events.dispatchEvent(new Event('visibilitychange'));
			}
		};
	}

	it('waits for two animation frames on a visible page', async () => {
		const frames = controllableFrames();
		const page = documentWithVisibility('visible');

		const waiting = track(
			createDomProbes({ includeInfiniteAnimations: false, requirePaint: false, document: page.document }).paint()
		);
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		frames.drawFrame();
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		frames.drawFrame();
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('does not wait for a paint on a live page that is already hidden', async () => {
		controllableFrames();
		const page = documentWithVisibility('hidden');

		await expect(
			createDomProbes({ includeInfiniteAnimations: false, requirePaint: false, document: page.document }).paint()
		).resolves.toBeUndefined();
	});

	it('stops waiting for a paint when a live page is hidden mid-wait', async () => {
		controllableFrames();
		const page = documentWithVisibility('visible');

		const waiting = track(
			createDomProbes({ includeInfiniteAnimations: false, requirePaint: false, document: page.document }).paint()
		);
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		page.hide();
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('gives up waiting for a paint after PAINT_TIMEOUT_MS on a visible live page that draws no frames', async () => {
		vi.useFakeTimers();
		const frames = controllableFrames();
		const page = documentWithVisibility('visible');

		const waiting = track(
			createDomProbes({ includeInfiniteAnimations: false, requirePaint: false, document: page.document }).paint()
		);
		await vi.advanceTimersByTimeAsync(PAINT_TIMEOUT_MS - 1);
		expect(waiting.done()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(waiting.done()).toBe(true);
		expect(frames.pending()).toBe(1);
	});

	it('never gives up waiting for a paint on a capture page that draws no frames', async () => {
		vi.useFakeTimers();
		controllableFrames();
		const page = documentWithVisibility('visible');

		const waiting = track(
			createDomProbes({ includeInfiniteAnimations: false, requirePaint: true, document: page.document }).paint()
		);
		await vi.advanceTimersByTimeAsync(60_000);
		expect(waiting.done()).toBe(false);
	});

	it('waits for a real paint on a capture page, hidden or not', async () => {
		const frames = controllableFrames();
		const page = documentWithVisibility('hidden');

		const waiting = track(
			createDomProbes({ includeInfiniteAnimations: false, requirePaint: true, document: page.document }).paint()
		);
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		frames.drawFrame();
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		frames.drawFrame();
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});
});

describe('ready on a visible page that draws no frames', () => {
	const payload: ReadyPayload = { revision: 'r1', slide: 1, step: 0 };

	function readyValue(): ReadyPayload | null | undefined {
		return (window as unknown as { __tapReady?: ReadyPayload | null }).__tapReady;
	}

	/** A page the window server reports visible whose browser never runs an animation frame. */
	function visibleDocumentWithoutFrames(): Document {
		vi.stubGlobal('requestAnimationFrame', () => 0);
		const events = new EventTarget();
		return {
			querySelectorAll: () => [],
			visibilityState: 'visible',
			addEventListener: events.addEventListener.bind(events),
			removeEventListener: events.removeEventListener.bind(events)
		} as unknown as Document;
	}

	afterEach(() => {
		clearReady();
		resetBlockersForTests();
	});

	it('publishes ready on a live page once PAINT_TIMEOUT_MS runs out', async () => {
		vi.useFakeTimers();
		resetBlockersForTests();
		const probes = createDomProbes({ includeInfiniteAnimations: false, requirePaint: false, document: visibleDocumentWithoutFrames() });

		const cancel = startReadyCycle(payload, probes);
		await vi.advanceTimersByTimeAsync(PAINT_TIMEOUT_MS - 1);
		expect(readyValue()).toBeNull();
		await vi.advanceTimersByTimeAsync(1);
		expect(readyValue()).toEqual(payload);
		cancel();
	});

	it('never publishes ready on a print page, however long it waits', async () => {
		vi.useFakeTimers();
		resetBlockersForTests();
		const probes = createDomProbes({ includeInfiniteAnimations: true, requirePaint: true, document: visibleDocumentWithoutFrames() });

		const cancel = startReadyCycle(payload, probes);
		await vi.advanceTimersByTimeAsync(120_000);
		expect(readyValue()).toBeNull();
		cancel();
	});
});
