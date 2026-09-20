/**
 * Keeps the screen awake while a deck is on it.
 *
 * A deck is watched, not touched: a phone propped up as a prompter, or a
 * laptop on a lectern, would otherwise dim and lock partway through a
 * slide. The Screen Wake Lock API asks the device not to.
 *
 * A lock is not held once and kept. It goes away for several unrelated
 * reasons, and each one needs its own way back:
 *
 *   - The page stops being visible (a tab switch, a lock screen, an app
 *     switch). The browser drops the lock and does not give it back on its
 *     own, so it is asked for again on every `visibilitychange` to visible.
 *   - The platform takes the lock back by itself, without the page going
 *     anywhere. The sentinel fires `release`, and it is asked for again.
 *   - The request is refused. Safari in particular refuses a request made
 *     while the page is still settling after load or after coming back to
 *     the foreground, so a single attempt at mount is not enough: a
 *     refusal is retried a few times, and the first touch or click asks
 *     again, because a request made from a gesture is the one browsers
 *     grant most readily.
 *
 * Running out of retries is not an error. Unsupported browsers and a
 * standing refusal (a device on low battery refuses) are not errors
 * either: the deck works exactly as before, the screen just dims the way
 * the device wants it to.
 */

/** How long to wait before asking again after a refusal. */
const RETRY_DELAY_MS = 1_000;

/**
 * How many times in a row to retry a refusal before leaving it alone. The
 * count resets whenever the page becomes visible or the viewer touches the
 * screen, so a device that refuses forever is asked a handful of times,
 * not in a loop.
 */
const MAX_RETRIES = 5;

export interface WakeLockHandle {
	/** Releases the lock and stops re-acquiring it. */
	release: () => void;
	/** The live sentinel, or null. Exposed for the tests. */
	current: () => WakeLockSentinel | null;
}

export function setupWakeLock(): WakeLockHandle {
	if (typeof window === 'undefined' || typeof document === 'undefined') {
		return { release: () => {}, current: () => null };
	}

	let sentinel: WakeLockSentinel | null = null;
	let released = false;
	let requesting = false;
	let retriesLeft = MAX_RETRIES;
	let retryTimer: ReturnType<typeof setTimeout> | null = null;

	/** Whether a lock is already held, so there is nothing to ask for. */
	function isHeld(): boolean {
		return sentinel !== null && !sentinel.released;
	}

	function clearRetry(): void {
		if (retryTimer === null) return;
		clearTimeout(retryTimer);
		retryTimer = null;
	}

	function scheduleRetry(): void {
		if (released || retryTimer !== null || retriesLeft <= 0) return;
		retriesLeft -= 1;
		retryTimer = setTimeout(() => {
			retryTimer = null;
			void request();
		}, RETRY_DELAY_MS);
	}

	/** The platform took the lock back while the deck is still on screen. */
	function handleSentinelRelease(): void {
		sentinel = null;
		if (released || document.visibilityState !== 'visible') return;
		void request();
	}

	async function request(): Promise<void> {
		if (released || requesting || isHeld() || !('wakeLock' in navigator)) {
			return;
		}

		requesting = true;
		try {
			const next = await navigator.wakeLock.request('screen');
			if (released) {
				void next.release();
				return;
			}
			next.addEventListener('release', handleSentinelRelease);
			sentinel = next;
			clearRetry();
		} catch {
			// Refused (still settling after load, low battery, a policy, an
			// unsupported surface). The deck does not depend on this.
			scheduleRetry();
		} finally {
			requesting = false;
		}
	}

	/** Asks again from the top, with a fresh budget of retries. */
	function requestAfresh(): void {
		if (released || isHeld()) return;
		clearRetry();
		retriesLeft = MAX_RETRIES;
		void request();
	}

	function handleVisibilityChange(): void {
		if (document.visibilityState !== 'visible') {
			// The browser has already released the lock; drop the stale
			// sentinel so the next request is not skipped as redundant.
			sentinel = null;
			clearRetry();
			return;
		}
		requestAfresh();
	}

	function handleGesture(): void {
		requestAfresh();
	}

	void request();
	document.addEventListener('visibilitychange', handleVisibilityChange);
	document.addEventListener('pointerdown', handleGesture, { passive: true });
	document.addEventListener('keydown', handleGesture, { passive: true });

	return {
		release: () => {
			released = true;
			clearRetry();
			document.removeEventListener('visibilitychange', handleVisibilityChange);
			document.removeEventListener('pointerdown', handleGesture);
			document.removeEventListener('keydown', handleGesture);
			sentinel?.removeEventListener('release', handleSentinelRelease);
			void sentinel?.release();
			sentinel = null;
		},
		current: () => sentinel
	};
}
