import { afterEach, describe, expect, it, vi } from 'vitest';
import { ANIMATION_TIMEOUT_MS, IMAGE_TIMEOUT_MS, STYLESHEET_TIMEOUT_MS, createDomProbes } from './probes';

function track(promise: Promise<void>): { done: () => boolean } {
	let finished = false;
	void promise.then(() => {
		finished = true;
	});
	return { done: () => finished };
}

afterEach(() => {
	vi.useRealTimers();
	document.body.innerHTML = '';
	document.head.querySelectorAll('link').forEach((link) => link.remove());
});

describe('images', () => {
	it('waits for an image that has not loaded, until it loads', async () => {
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: false, configurable: true });
		document.body.appendChild(image);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false }).images());
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		image.dispatchEvent(new Event('load'));
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('counts a broken image as done', async () => {
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: false, configurable: true });
		document.body.appendChild(image);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false }).images());
		image.dispatchEvent(new Event('error'));
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('gives up on an image after IMAGE_TIMEOUT_MS', async () => {
		vi.useFakeTimers();
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: false, configurable: true });
		document.body.appendChild(image);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false }).images());
		await vi.advanceTimersByTimeAsync(IMAGE_TIMEOUT_MS - 1);
		expect(waiting.done()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(waiting.done()).toBe(true);
	});

	it('does not wait for an image that is complete', async () => {
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: true, configurable: true });
		document.body.appendChild(image);

		await expect(createDomProbes({ includeInfiniteAnimations: false }).images()).resolves.toBeUndefined();
	});
});

describe('stylesheets and fonts', () => {
	it('waits for a stylesheet to load, and then counts it as settled', async () => {
		const link = document.createElement('link');
		link.rel = 'stylesheet';
		document.head.appendChild(link);
		const probes = createDomProbes({ includeInfiniteAnimations: false });
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

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false }).fonts());
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
		const probes = createDomProbes({ includeInfiniteAnimations: false, document: fakeDocument });
		expect(probes.settledNow()).toBe(false);

		const waiting = track(probes.fonts());
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		fonts.status = 'loaded';
		resolveFonts();
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
		expect(probes.settledNow()).toBe(true);
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
			createDomProbes({ includeInfiniteAnimations: false, document: documentWith([running.animation]) }).animations()
		);
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		running.finish();
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('ignores an animation that repeats forever on a live page', async () => {
		const looping = fakeAnimation(Infinity);
		await expect(
			createDomProbes({ includeInfiniteAnimations: false, document: documentWith([looping.animation]) }).animations()
		).resolves.toBeUndefined();
	});

	it('waits up to ANIMATION_TIMEOUT_MS for an animation that repeats forever on a print page', async () => {
		vi.useFakeTimers();
		const looping = fakeAnimation(Infinity);
		const waiting = track(
			createDomProbes({ includeInfiniteAnimations: true, document: documentWith([looping.animation]) }).animations()
		);
		await vi.advanceTimersByTimeAsync(ANIMATION_TIMEOUT_MS - 1);
		expect(waiting.done()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(waiting.done()).toBe(true);
	});
});
