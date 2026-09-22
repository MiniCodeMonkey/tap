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

/** Resolves once no blocker is held. */
export function whenNoBlockers(): Promise<void> {
	if (held.size === 0) {
		return Promise.resolve();
	}
	return new Promise((resolve) => {
		const unsubscribe = subscribeToBlockers(() => {
			if (held.size === 0) {
				unsubscribe();
				resolve();
			}
		});
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
