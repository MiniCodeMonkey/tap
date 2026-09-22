/**
 * The one ready signal. tap export pdf, tap export images and Tap
 * Desktop's thumbnail renderer all wait for it before they capture a
 * slide. Once the slide on screen has settled (see waitUntilSettled), the
 * page sets window.__tapReady to {revision, slide, step}, dispatches a
 * "tap:ready" event on window with the same object as its detail, and
 * posts it to window.webkit.messageHandlers.tapReady when the page runs in
 * a WKWebView that registered that handler. window.__tapReady is null
 * while a slide is settling. `slide` is 1-based, and `step` is the number
 * of steps taken on that slide.
 */

import { BLOCKER_TIMEOUT_MS, heldBlockers, whenNoBlockers } from './blockers';
import type { ReadyProbes } from './probes';

export interface ReadyPayload {
	revision: string;
	slide: number;
	step: number;
}

/** The event dispatched on window when a slide has settled. */
export const READY_EVENT = 'tap:ready';

/**
 * A cycle runs at most this many rounds before giving up. Every wait a
 * round makes, including a held blocker's (see BLOCKER_TIMEOUT_MS), times
 * out on its own, so this cap is always reached even if fonts keep
 * starting to load or a blocker is never released.
 */
export const MAX_SETTLE_ROUNDS = 20;

interface ReadyWindow {
	__tapReady?: ReadyPayload | null;
	webkit?: {
		messageHandlers?: {
			tapReady?: { postMessage(message: ReadyPayload): void };
		};
	};
}

function readyWindow(): ReadyWindow | null {
	return typeof window === 'undefined' ? null : (window as unknown as ReadyWindow);
}

/** Marks the page as not settled. */
export function clearReady(): void {
	const target = readyWindow();
	if (target) {
		target.__tapReady = null;
	}
}

/** Reports a settled slide through all three channels. */
export function publishReady(payload: ReadyPayload): void {
	const target = readyWindow();
	if (!target) {
		return;
	}
	const message: ReadyPayload = { revision: payload.revision, slide: payload.slide, step: payload.step };
	target.__tapReady = message;
	window.dispatchEvent(new CustomEvent<ReadyPayload>(READY_EVENT, { detail: message }));
	target.webkit?.messageHandlers?.tapReady?.postMessage(message);
}

/**
 * Waits until the page has settled: each round waits for stylesheets and
 * fonts, images, every blocker's release, animations, and a paint. The
 * page has settled after a round that ends with no blocker held and
 * nothing loading. Resolves false when the cycle was cancelled, or when
 * MAX_SETTLE_ROUNDS ran out without a round that saw both no blocker held
 * and the probes settled - a page that never finishes loading a stylesheet
 * or a font fails the export loudly instead of being captured half drawn.
 */
export async function waitUntilSettled(probes: ReadyProbes, isCancelled: () => boolean): Promise<boolean> {
	for (let round = 0; round < MAX_SETTLE_ROUNDS; round += 1) {
		await probes.fonts();
		await probes.images();
		await whenNoBlockers(BLOCKER_TIMEOUT_MS);
		await probes.animations();
		await probes.paint();
		if (isCancelled()) {
			return false;
		}
		if (heldBlockers().length === 0 && probes.settledNow()) {
			return true;
		}
	}
	return false;
}

let currentCycle = 0;

/**
 * Starts waiting for the slide in `payload` to settle, and publishes
 * `payload` when it has. Clears the signal at once. Starting another
 * cycle, or calling the returned function, cancels this one, so an older
 * slide never reports ready over a newer one.
 */
export function startReadyCycle(payload: ReadyPayload, probes: ReadyProbes): () => void {
	currentCycle += 1;
	const cycle = currentCycle;
	let cancelled = false;
	const isCancelled = (): boolean => cancelled || cycle !== currentCycle;

	clearReady();
	void waitUntilSettled(probes, isCancelled).then((settled) => {
		if (settled && !isCancelled()) {
			publishReady(payload);
		}
	});

	return () => {
		cancelled = true;
	};
}
