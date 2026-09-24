import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BLOCKER_TIMEOUT_MS, holdReady, resetBlockersForTests, type ReadyBlockerKind } from './blockers';
import type { ReadyProbes } from './probes';
import {
	MAX_SETTLE_ROUNDS,
	READY_EVENT,
	clearReady,
	markReadyOff,
	readyState,
	startReadyCycle,
	waitUntilSettled,
	type ReadyPayload
} from './readySignal';

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
	vi.useRealTimers();
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

	it('finishes as not settled within MAX_SETTLE_ROUNDS when a blocker is never released', async () => {
		vi.useFakeTimers();
		holdReady('component');

		const settledPromise = waitUntilSettled(instantProbes(), () => false);
		await vi.advanceTimersByTimeAsync(MAX_SETTLE_ROUNDS * BLOCKER_TIMEOUT_MS);

		await expect(settledPromise).resolves.toBe(false);
	});

	it('finishes as not settled within MAX_SETTLE_ROUNDS when no blocker is held but a probe never settles', async () => {
		// No blocker is ever held here: settledNow stays false the whole
		// time, as it would while a stylesheet or web font never finishes
		// loading. The cap must not treat "no blocker held" alone as
		// settled, or the exporter captures the slide with fallback fonts
		// instead of failing loudly.
		const settled = await waitUntilSettled(instantProbes({ settledNow: () => false }), () => false);

		expect(settled).toBe(false);
	});

	it('settles once a blocker still held after one timeout is released', async () => {
		vi.useFakeTimers();
		let release: (() => void) | null = holdReady('component');
		const fonts = vi.fn(() => {
			if (fonts.mock.calls.length === 2) {
				release?.();
			}
			return Promise.resolve();
		});

		const settledPromise = waitUntilSettled(instantProbes({ fonts }), () => false);
		await vi.advanceTimersByTimeAsync(BLOCKER_TIMEOUT_MS);

		await expect(settledPromise).resolves.toBe(true);
		expect(fonts).toHaveBeenCalledTimes(2);
	});
});

describe('__tapReadyState', () => {
	function pageState(): Record<string, unknown> {
		return JSON.parse(JSON.stringify((window as unknown as { __tapReadyState: unknown }).__tapReadyState)) as Record<
			string,
			unknown
		>;
	}

	it('names the wait a stalled cycle is in, and for how long', async () => {
		const paint = deferred();
		startReadyCycle(payload, instantProbes({ paint: () => paint.promise }));
		await settleMicrotasks();

		const state = pageState();
		expect(state.phase).toBe('paint');
		expect(state.round).toBe(1);
		expect(typeof state.phaseMs).toBe('number');
		expect(typeof state.pageMs).toBe('number');

		paint.resolve();
		await vi.waitFor(() => expect(pageState().phase).toBe('published'));
	});

	it('lists the blockers held while a cycle waits for them', async () => {
		const release = holdReady('component');
		startReadyCycle(payload, instantProbes());
		await settleMicrotasks();

		const state = pageState();
		expect(state.phase).toBe('blockers');
		expect(state.blockers).toEqual([{ kind: 'component', heldMs: expect.any(Number) }]);
		release();
	});

	it('says why a round did not settle', async () => {
		let rounds = 0;
		const settledNow = (): boolean => {
			rounds += 1;
			return rounds > 1;
		};
		const paint = vi.fn(() => (paint.mock.calls.length === 2 ? new Promise<void>(() => {}) : Promise.resolve()));
		startReadyCycle(payload, instantProbes({ settledNow, paint }));
		await settleMicrotasks();

		expect(pageState()).toMatchObject({ phase: 'paint', round: 2, unsettled: ['loading'] });
	});

	it('records a cycle that ran out of rounds as gave-up, not as still waiting', async () => {
		startReadyCycle(payload, instantProbes({ settledNow: () => false }));
		await vi.waitFor(() => expect(pageState().phase).toBe('gave-up'));
		expect(pageState().round).toBe(MAX_SETTLE_ROUNDS);
		expect(readyValue()).toBeNull();
	});

	it('counts publishes and whether the message handler was there to receive them', async () => {
		const before = readyState().published;
		const postMessage = vi.fn();
		(window as unknown as { webkit: unknown }).webkit = { messageHandlers: { tapReady: { postMessage } } };
		startReadyCycle(payload, instantProbes());
		await vi.waitFor(() => expect(pageState().phase).toBe('published'));

		expect(pageState()).toMatchObject({ published: before + 1, posted: true });
		expect(postMessage).toHaveBeenCalledTimes(1);
	});

	it('keeps a cancelled cycle from overwriting the newer one', async () => {
		const stalled = deferred();
		startReadyCycle(payload, instantProbes({ images: () => stalled.promise }));
		await settleMicrotasks();
		const cycles = readyState().cycles;

		const paint = deferred();
		startReadyCycle({ ...payload, slide: 3 }, instantProbes({ paint: () => paint.promise }));
		stalled.resolve();
		await settleMicrotasks();

		expect(pageState()).toMatchObject({ phase: 'paint', cycles: cycles + 1 });
	});

	it('reads off while the deck has not loaded, even after a cycle started', async () => {
		const paint = deferred();
		startReadyCycle(payload, instantProbes({ paint: () => paint.promise }))();
		markReadyOff();
		paint.resolve();
		await settleMicrotasks();

		expect(pageState().phase).toBe('off');
	});
});

describe('a cycle that runs out of rounds', () => {
	it('on a live page, publishes ready anyway with settled false', async () => {
		startReadyCycle(payload, instantProbes({ settledNow: () => false }), { publishUnsettled: true });

		await vi.waitFor(() => expect(readyValue()).toEqual({ ...payload, settled: false }));
		expect(readyState()).toMatchObject({ phase: 'published', settled: false, unsettled: ['loading'] });
	});

	it('on a page that requires a paint, publishes nothing', async () => {
		startReadyCycle(payload, instantProbes({ settledNow: () => false }), { publishUnsettled: false });

		await vi.waitFor(() => expect(readyState().phase).toBe('gave-up'));
		expect(readyValue()).toBeNull();
	});

	it('leaves settled out of a payload that did settle', async () => {
		startReadyCycle(payload, instantProbes(), { publishUnsettled: true });

		await vi.waitFor(() => expect(readyValue()).toEqual(payload));
		expect(readyValue()).not.toHaveProperty('settled');
		expect(readyState().settled).toBe(true);
	});

	it('does not publish for a cycle cancelled while its rounds ran out', async () => {
		const cancel = startReadyCycle(payload, instantProbes({ settledNow: () => false }), { publishUnsettled: true });
		cancel();
		await settleMicrotasks();

		expect(readyValue()).toBeNull();
	});
});
