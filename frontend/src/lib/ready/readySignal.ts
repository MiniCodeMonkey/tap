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
 *
 * While it works, the page also keeps window.__tapReadyState (see
 * ReadyState): which step of the ready logic it is in, since when, and
 * why the last settle round did not settle. Nothing reads it to decide
 * anything; it is there so a test that times out waiting for ready can
 * say where the page stopped.
 */

import { BLOCKER_TIMEOUT_MS, heldBlockerAges, heldBlockers, whenNoBlockers } from './blockers';
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

/**
 * The step the ready logic is in. "off" while the deck has not loaded (no
 * cycle runs), then one of a settle round's waits, "check" while a round
 * decides whether it settled, and a cycle's outcome: "published",
 * "gave-up" once MAX_SETTLE_ROUNDS ran out, or "cancelled" when a cycle
 * ended without a newer one taking over.
 */
export type ReadyPhase =
	| 'off'
	| 'fonts'
	| 'images'
	| 'blockers'
	| 'animations'
	| 'paint'
	| 'check'
	| 'published'
	| 'gave-up'
	| 'cancelled';

/**
 * What window.__tapReadyState holds. Times are milliseconds since the page
 * started loading (performance.now()); the *Ms getters are ages at the
 * moment they are read, so JSON.stringify of the object reports them as
 * of the read.
 */
export interface ReadyState {
	phase: ReadyPhase;
	/** When the page entered `phase`. */
	phaseAt: number;
	/** How many cycles this page load has started. */
	cycles: number;
	/** When the newest cycle started. */
	cycleAt: number;
	/** The newest cycle's round, counting from 1. 0 before its first round. */
	round: number;
	/** How many times this page load has published ready. */
	published: number;
	/** Whether the newest publish found the tapReady message handler to post to. */
	posted: boolean;
	/** What kept the newest cycle's latest finished round from settling: blocker kinds, and "loading" when a stylesheet or font was loading. */
	unsettled: string[];
	readonly phaseMs: number;
	readonly cycleMs: number;
	readonly pageMs: number;
	/** The blockers held right now, with how long each has been held. */
	readonly blockers: { kind: string; heldMs: number }[];
}

interface ReadyWindow {
	__tapReady?: ReadyPayload | null;
	__tapReadyState?: ReadyState;
	webkit?: {
		messageHandlers?: {
			tapReady?: { postMessage(message: ReadyPayload): void };
		};
	};
}

function readyWindow(): ReadyWindow | null {
	return typeof window === 'undefined' ? null : (window as unknown as ReadyWindow);
}

function now(): number {
	return typeof performance === 'undefined' ? Date.now() : performance.now();
}

function createReadyState(): ReadyState {
	return {
		phase: 'off',
		phaseAt: now(),
		cycles: 0,
		cycleAt: 0,
		round: 0,
		published: 0,
		posted: false,
		unsettled: [],
		get phaseMs() {
			return Math.round(now() - this.phaseAt);
		},
		get cycleMs() {
			return this.cycles === 0 ? 0 : Math.round(now() - this.cycleAt);
		},
		get pageMs() {
			return Math.round(now());
		},
		get blockers() {
			return heldBlockerAges();
		}
	};
}

/** The page's ready state, created on first use and kept on window as __tapReadyState. */
export function readyState(): ReadyState {
	const target = readyWindow();
	if (!target) {
		return fallbackState;
	}
	target.__tapReadyState ??= createReadyState();
	return target.__tapReadyState;
}

const fallbackState = createReadyState();

function enterPhase(phase: ReadyPhase): void {
	const state = readyState();
	state.phase = phase;
	state.phaseAt = now();
}

/** Records that no cycle runs because the deck has not loaded. A cycle still running stops writing the state. */
export function markReadyOff(): void {
	currentCycle += 1;
	clearReady();
	enterPhase('off');
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
	const state = readyState();
	state.published += 1;
	state.posted = target.webkit?.messageHandlers?.tapReady !== undefined;
	enterPhase('published');
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
export async function waitUntilSettled(
	probes: ReadyProbes,
	isCancelled: () => boolean,
	record: SettleRecorder = ignoreProgress
): Promise<boolean> {
	for (let round = 0; round < MAX_SETTLE_ROUNDS; round += 1) {
		record.round(round + 1);
		record.phase('fonts');
		await probes.fonts();
		record.phase('images');
		await probes.images();
		record.phase('blockers');
		await whenNoBlockers(BLOCKER_TIMEOUT_MS);
		record.phase('animations');
		await probes.animations();
		record.phase('paint');
		await probes.paint();
		if (isCancelled()) {
			return false;
		}
		record.phase('check');
		const blockers = heldBlockers();
		const loaded = probes.settledNow();
		if (blockers.length === 0 && loaded) {
			return true;
		}
		record.unsettled(loaded ? blockers : [...blockers, 'loading']);
	}
	return false;
}

/** Where waitUntilSettled reports its progress. */
export interface SettleRecorder {
	round(round: number): void;
	phase(phase: ReadyPhase): void;
	unsettled(reasons: string[]): void;
}

const ignoreProgress: SettleRecorder = { round: () => {}, phase: () => {}, unsettled: () => {} };

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
	const state = readyState();
	state.cycles += 1;
	state.cycleAt = now();
	state.round = 0;
	state.unsettled = [];
	// Only the newest cycle writes the state, so a cancelled cycle's
	// leftover progress never overwrites where the current one is.
	const isNewest = (): boolean => cycle === currentCycle;
	const record: SettleRecorder = {
		round: (round) => {
			if (isNewest()) state.round = round;
		},
		phase: (phase) => {
			if (isNewest()) enterPhase(phase);
		},
		unsettled: (reasons) => {
			if (isNewest()) state.unsettled = reasons;
		}
	};
	void waitUntilSettled(probes, isCancelled, record).then((settled) => {
		if (settled && !isCancelled()) {
			publishReady(payload);
		} else if (isNewest()) {
			enterPhase(cancelled ? 'cancelled' : 'gave-up');
		}
	});

	return () => {
		cancelled = true;
	};
}
