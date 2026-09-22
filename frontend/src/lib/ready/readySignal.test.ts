import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { holdReady, resetBlockersForTests, type ReadyBlockerKind } from './blockers';
import type { ReadyProbes } from './probes';
import { MAX_SETTLE_ROUNDS, READY_EVENT, clearReady, startReadyCycle, waitUntilSettled, type ReadyPayload } from './readySignal';

interface ReadyWindow {
	__tapReady?: ReadyPayload | null;
	webkit?: unknown;
}

function readyValue(): ReadyPayload | null | undefined {
	return (window as unknown as ReadyWindow).__tapReady;
}

function instantProbes(overrides: Partial<ReadyProbes> = {}): ReadyProbes {
	return {
		fonts: () => Promise.resolve(),
		images: () => Promise.resolve(),
		animations: () => Promise.resolve(),
		paint: () => Promise.resolve(),
		settledNow: () => true,
		...overrides
	};
}

function deferred(): { promise: Promise<void>; resolve: () => void } {
	let resolve!: () => void;
	const promise = new Promise<void>((settle) => {
		resolve = settle;
	});
	return { promise, resolve };
}

/** Lets every pending promise callback run. */
async function settleMicrotasks(): Promise<void> {
	await new Promise((resolve) => setTimeout(resolve, 0));
}

const payload: ReadyPayload = { revision: 'r1', slide: 2, step: 1 };

beforeEach(() => {
	resetBlockersForTests();
	clearReady();
});

afterEach(() => {
	delete (window as unknown as ReadyWindow).webkit;
});

describe('startReadyCycle', () => {
	it('publishes the payload once nothing blocks', async () => {
		startReadyCycle(payload, instantProbes());
		await vi.waitFor(() => expect(readyValue()).toEqual(payload));
	});

	it('sets __tapReady to null as soon as a cycle starts', async () => {
		startReadyCycle(payload, instantProbes());
		await vi.waitFor(() => expect(readyValue()).toEqual(payload));

		holdReady('component');
		startReadyCycle({ ...payload, slide: 3 }, instantProbes());
		expect(readyValue()).toBeNull();
	});

	it.each<ReadyBlockerKind>(['fonts', 'images', 'map', 'component', 'error-card', 'animations'])(
		'waits for a held %s blocker',
		async (kind) => {
			const release = holdReady(kind);
			startReadyCycle(payload, instantProbes());
			await settleMicrotasks();
			expect(readyValue()).toBeNull();

			release();
			await vi.waitFor(() => expect(readyValue()).toEqual(payload));
		}
	);

	it.each<keyof Omit<ReadyProbes, 'settledNow'>>(['fonts', 'images', 'animations', 'paint'])(
		'waits for the %s probe',
		async (probe) => {
			const pending = deferred();
			startReadyCycle(payload, instantProbes({ [probe]: () => pending.promise } as Partial<ReadyProbes>));
			await settleMicrotasks();
			expect(readyValue()).toBeNull();

			pending.resolve();
			await vi.waitFor(() => expect(readyValue()).toEqual(payload));
		}
	);

	it('runs another round when something started loading during the first one', async () => {
		const fonts = vi.fn(() => Promise.resolve());
		const settledNow = vi.fn().mockReturnValueOnce(false).mockReturnValue(true);
		startReadyCycle(payload, instantProbes({ fonts, settledNow }));

		await vi.waitFor(() => expect(readyValue()).toEqual(payload));
		expect(fonts).toHaveBeenCalledTimes(2);
	});

	it('runs another round when a blocker was held during the first one', async () => {
		let release: (() => void) | null = null;
		const paint = vi.fn(() => {
			if (paint.mock.calls.length === 1) {
				release = holdReady('component');
				setTimeout(() => release?.(), 0);
			}
			return Promise.resolve();
		});
		startReadyCycle(payload, instantProbes({ paint }));

		await vi.waitFor(() => expect(readyValue()).toEqual(payload));
		expect(paint).toHaveBeenCalledTimes(2);
	});

	it('never lets an earlier cycle publish over a later one', async () => {
		const slowFonts = deferred();
		startReadyCycle({ ...payload, slide: 1 }, instantProbes({ fonts: () => slowFonts.promise }));
		startReadyCycle({ ...payload, slide: 2 }, instantProbes());
		await vi.waitFor(() => expect(readyValue()?.slide).toBe(2));

		slowFonts.resolve();
		await settleMicrotasks();
		expect(readyValue()?.slide).toBe(2);
	});

	it('does not publish after it is cancelled', async () => {
		const cancel = startReadyCycle(payload, instantProbes());
		cancel();
		await settleMicrotasks();
		expect(readyValue()).toBeNull();
	});

	it('dispatches tap:ready with the payload', async () => {
		const listener = vi.fn();
		window.addEventListener(READY_EVENT, listener);
		startReadyCycle(payload, instantProbes());

		await vi.waitFor(() => expect(listener).toHaveBeenCalledTimes(1));
		expect((listener.mock.calls[0]?.[0] as CustomEvent<ReadyPayload>).detail).toEqual(payload);
		window.removeEventListener(READY_EVENT, listener);
	});

	it('posts the payload to the tapReady message handler when one exists', async () => {
		const postMessage = vi.fn();
		(window as unknown as ReadyWindow).webkit = { messageHandlers: { tapReady: { postMessage } } };
		startReadyCycle(payload, instantProbes());

		await vi.waitFor(() => expect(postMessage).toHaveBeenCalledWith(payload));
	});

	it('does not fail when webkit has no tapReady handler', async () => {
		(window as unknown as ReadyWindow).webkit = { messageHandlers: {} };
		startReadyCycle(payload, instantProbes());
		await vi.waitFor(() => expect(readyValue()).toEqual(payload));
	});
});

describe('waitUntilSettled', () => {
	it('never reports settled while a blocker is held on every round through MAX_SETTLE_ROUNDS', async () => {
		let release: (() => void) | null = null;
		const fonts = vi.fn(() => {
			release?.();
			return Promise.resolve();
		});
		const animations = vi.fn(() => {
			release = holdReady('component');
			return Promise.resolve();
		});

		const settled = await waitUntilSettled(instantProbes({ fonts, animations }), () => false);

		expect(settled).toBe(false);
		expect(fonts).toHaveBeenCalledTimes(MAX_SETTLE_ROUNDS);
		expect(animations).toHaveBeenCalledTimes(MAX_SETTLE_ROUNDS);
	});

	it('reports settled once the blocker held during an early round is released', async () => {
		let release: (() => void) | null = null;
		const paint = vi.fn(() => {
			if (paint.mock.calls.length === 1) {
				release = holdReady('component');
				setTimeout(() => release?.(), 0);
			}
			return Promise.resolve();
		});

		const settled = await waitUntilSettled(instantProbes({ paint }), () => false);

		expect(settled).toBe(true);
		expect(paint).toHaveBeenCalledTimes(2);
	});
});
