/**
 * The things a slide waits on before it counts as rendered. A deck
 * component whose bundle is still loading, a map that has not drawn its
 * tiles, a slide transition that is still running: each one holds a
 * blocker and releases it when it is done. The ready signal (see
 * readySignal.ts) fires only when no blocker is held.
 */

import { useLayoutEffect } from 'react';

/** What a blocker waits on. */
export type ReadyBlockerKind = 'fonts' | 'images' | 'map' | 'component' | 'error-card' | 'animations';

/**
 * How long whenNoBlockers waits for every blocker to release before giving
 * up. Shorter than some blockers can legitimately run: MapSlide.tsx holds
 * one for up to 10000ms while a map's tiles load, and a fly-to hold is its
 * animation duration plus another timeout on top, routinely longer still.
 * A round that times out with a blocker held simply loops (see
 * waitUntilSettled in readySignal.ts), so this is not a bug, but it does
 * mean the real worst case for a cycle that keeps re-arming a blocker is
 * MAX_SETTLE_ROUNDS times this timeout, which can run past tap's 30
 * second per-slide export timeout (internal/pdf/ready.go).
 */
export const BLOCKER_TIMEOUT_MS = 5000;

const held = new Map<number, ReadyBlockerKind>();
const listeners = new Set<() => void>();
let nextBlockerId = 1;

function notify(): void {
	for (const listener of [...listeners]) {
		listener();
	}
}

/**
 * Holds a blocker of `kind` until the returned function runs. Calling the
 * returned function again does nothing.
 */
export function holdReady(kind: ReadyBlockerKind): () => void {
	const id = nextBlockerId;
	nextBlockerId += 1;
	held.set(id, kind);
	notify();
	return () => {
		if (held.delete(id)) {
			notify();
		}
	};
}

/** The kind of every blocker held right now, one entry per blocker. */
export function heldBlockers(): ReadyBlockerKind[] {
	return [...held.values()];
}

/** Calls `listener` after every hold and release. Returns the unsubscribe function. */
export function subscribeToBlockers(listener: () => void): () => void {
	listeners.add(listener);
	return () => {
		listeners.delete(listener);
	};
}

/**
 * Resolves once no blocker is held, or after `timeoutMs`, whichever comes
 * first. A blocker that is never released gives up instead of waiting
 * forever, so a settle round always ends.
 */
export function whenNoBlockers(timeoutMs: number): Promise<void> {
	if (held.size === 0) {
		return Promise.resolve();
	}
	return new Promise((resolve) => {
		const timer = setTimeout(finish, timeoutMs);
		const unsubscribe = subscribeToBlockers(() => {
			if (held.size === 0) {
				finish();
			}
		});
		function finish(): void {
			clearTimeout(timer);
			unsubscribe();
			resolve();
		}
	});
}

/**
 * Holds a blocker of `kind` while `holding` is true. A layout effect, so
 * the blocker is held by the time the page's own effects start a ready
 * cycle for the same commit.
 */
export function useReadyHold(kind: ReadyBlockerKind, holding: boolean): void {
	useLayoutEffect(() => {
		if (!holding) {
			return undefined;
		}
		return holdReady(kind);
	}, [kind, holding]);
}

/** Drops every blocker and listener (for testing). */
export function resetBlockersForTests(): void {
	held.clear();
	listeners.clear();
}
