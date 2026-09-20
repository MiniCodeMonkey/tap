/**
 * Keeps the screen awake while a deck is on it.
 *
 * A deck is watched, not touched: a phone propped up as a prompter, or a
 * laptop on a lectern, would otherwise dim and lock partway through a
 * slide. The Screen Wake Lock API asks the device not to.
 *
 * The browser drops the lock whenever the page stops being visible -- a tab
 * switch, a lock screen, an app switch -- and does not give it back on its
 * own, so it is asked for again every time the page becomes visible.
 *
 * Unsupported browsers, and a refusal (a device on low battery refuses),
 * are not errors: the deck works exactly as before, the screen just dims
 * the way the device wants it to.
 */

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

	async function request(): Promise<void> {
		if (released || !('wakeLock' in navigator)) {
			return;
		}

		try {
			const next = await navigator.wakeLock.request('screen');
			if (released) {
				void next.release();
				return;
			}
			sentinel = next;
		} catch {
			// Refused (low battery, a policy, an unsupported surface). The
			// deck does not depend on this.
		}
	}

	function handleVisibilityChange(): void {
		if (document.visibilityState === 'visible') {
			void request();
		}
	}

	void request();
	document.addEventListener('visibilitychange', handleVisibilityChange);

	return {
		release: () => {
			released = true;
			document.removeEventListener('visibilitychange', handleVisibilityChange);
			void sentinel?.release();
			sentinel = null;
		},
		current: () => sentinel
	};
}
