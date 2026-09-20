/**
 * Unit tests for keeping the screen awake while a deck is on it.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { setupWakeLock } from './wakeLock';

/**
 * A sentinel that records whether it was released, and that can fire the
 * `release` event the platform fires when it takes the lock back.
 */
function makeSentinel(): WakeLockSentinel & { released: boolean; drop: () => void } {
	const target = new EventTarget();
	const sentinel = {
		released: false,
		release: vi.fn(async () => {
			sentinel.released = true;
		}),
		addEventListener: target.addEventListener.bind(target),
		removeEventListener: target.removeEventListener.bind(target),
		/** Simulates the platform releasing the lock on its own. */
		drop: () => {
			sentinel.released = true;
			target.dispatchEvent(new Event('release'));
		}
	};
	return sentinel as unknown as WakeLockSentinel & { released: boolean; drop: () => void };
}

function setVisibility(state: DocumentVisibilityState): void {
	Object.defineProperty(document, 'visibilityState', { value: state, configurable: true });
}

describe('wake lock', () => {
	let request: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		request = vi.fn(async () => makeSentinel());
		Object.defineProperty(navigator, 'wakeLock', {
			value: { request },
			configurable: true,
			writable: true
		});
		setVisibility('visible');
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('asks for a screen lock as soon as the deck is open', async () => {
		const handle = setupWakeLock();
		await vi.waitFor(() => expect(request).toHaveBeenCalledWith('screen'));
		handle.release();
	});

	it('asks again when the page becomes visible, because the browser drops it', async () => {
		const handle = setupWakeLock();
		await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(1));

		setVisibility('hidden');
		document.dispatchEvent(new Event('visibilitychange'));
		expect(request).toHaveBeenCalledTimes(1);

		setVisibility('visible');
		document.dispatchEvent(new Event('visibilitychange'));
		await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(2));

		handle.release();
	});

	it('releases the lock and stops asking once released', async () => {
		const handle = setupWakeLock();
		await vi.waitFor(() => expect(handle.current()).not.toBeNull());
		const sentinel = handle.current() as WakeLockSentinel & { released: boolean };

		handle.release();
		expect(sentinel.release).toHaveBeenCalled();
		expect(handle.current()).toBeNull();

		setVisibility('visible');
		document.dispatchEvent(new Event('visibilitychange'));
		expect(request).toHaveBeenCalledTimes(1);
	});

	it('releases a lock that arrives after the deck closed', async () => {
		const sentinel = makeSentinel();
		let resolveRequest: (value: WakeLockSentinel) => void = () => {};
		request.mockImplementation(
			() =>
				new Promise<WakeLockSentinel>((resolve) => {
					resolveRequest = resolve;
				})
		);

		const handle = setupWakeLock();
		handle.release();
		resolveRequest(sentinel);

		await vi.waitFor(() => expect(sentinel.release).toHaveBeenCalled());
		expect(handle.current()).toBeNull();
	});

	it('does nothing where the browser has no wake lock', async () => {
		Object.defineProperty(navigator, 'wakeLock', { value: undefined, configurable: true });
		const handle = setupWakeLock();
		await Promise.resolve();
		expect(handle.current()).toBeNull();
		expect(() => handle.release()).not.toThrow();
	});

	it('carries on when the request is refused', async () => {
		request.mockRejectedValue(new Error('NotAllowedError'));
		const handle = setupWakeLock();
		await Promise.resolve();
		expect(handle.current()).toBeNull();
		handle.release();
	});

	it('takes the lock back when the platform drops it', async () => {
		const handle = setupWakeLock();
		await vi.waitFor(() => expect(handle.current()).not.toBeNull());
		const sentinel = handle.current() as WakeLockSentinel & { drop: () => void };

		sentinel.drop();

		await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(2));
		await vi.waitFor(() => expect(handle.current()).not.toBe(sentinel));
		handle.release();
	});

	it('asks again after a refusal instead of giving up on one attempt', async () => {
		vi.useFakeTimers();
		try {
			request.mockRejectedValueOnce(new Error('NotAllowedError'));

			const handle = setupWakeLock();
			await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(1));
			expect(handle.current()).toBeNull();

			await vi.advanceTimersByTimeAsync(5_000);

			await vi.waitFor(() => expect(handle.current()).not.toBeNull());
			handle.release();
		} finally {
			vi.useRealTimers();
		}
	});

	it('asks again on the first touch after a refusal', async () => {
		request.mockRejectedValueOnce(new Error('NotAllowedError'));

		const handle = setupWakeLock();
		await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(1));
		expect(handle.current()).toBeNull();

		document.dispatchEvent(new Event('pointerdown'));

		await vi.waitFor(() => expect(handle.current()).not.toBeNull());
		handle.release();
	});

	it('does not ask again on a touch while the lock is held', async () => {
		const handle = setupWakeLock();
		await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(1));

		document.dispatchEvent(new Event('pointerdown'));
		await Promise.resolve();

		expect(request).toHaveBeenCalledTimes(1);
		handle.release();
	});

	it('stops asking after release, whatever fires next', async () => {
		request.mockRejectedValueOnce(new Error('NotAllowedError'));
		const handle = setupWakeLock();
		await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(1));

		handle.release();

		document.dispatchEvent(new Event('pointerdown'));
		setVisibility('visible');
		document.dispatchEvent(new Event('visibilitychange'));
		await Promise.resolve();

		expect(request).toHaveBeenCalledTimes(1);
	});
});
