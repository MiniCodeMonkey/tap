/**
 * Hides the mouse pointer while a fullscreen deck sits still.
 *
 * Presenting on a TV or a projector leaves the pointer parked on the slide,
 * where it is as visible as anything on it. It hides after a short idle and
 * comes straight back on the next movement, so pointing at something still
 * works and nothing has to be toggled before or after a talk.
 *
 * Only while fullscreen: in a window the deck is being worked on, and a
 * vanishing pointer there would be a bug, not a feature.
 */

/** How long the pointer may sit still before it hides, in milliseconds. */
const IDLE_MS = 2500;

/** The class that hides it, defined in app.css. */
const HIDDEN_CLASS = 'cursor-hidden';

export interface CursorOptions {
	/** Overrides the idle delay. Used by the tests. */
	idleMs?: number;
}

/**
 * Start hiding the pointer after an idle period while fullscreen.
 * Returns a cleanup function that removes the listeners and shows it again.
 */
export function setupCursorAutoHide({ idleMs = IDLE_MS }: CursorOptions = {}): () => void {
	if (typeof window === 'undefined' || typeof document === 'undefined') {
		return () => {};
	}

	let timer: ReturnType<typeof setTimeout> | null = null;

	const show = (): void => document.body.classList.remove(HIDDEN_CLASS);
	const hide = (): void => document.body.classList.add(HIDDEN_CLASS);

	const clear = (): void => {
		if (timer !== null) {
			clearTimeout(timer);
			timer = null;
		}
	};

	/** Show the pointer now, and hide it again once it has sat still. */
	const restart = (): void => {
		clear();
		show();

		if (!document.fullscreenElement) {
			return;
		}

		timer = setTimeout(hide, idleMs);
	};

	// A touch is not a pointer sitting on the slide, so touch events are
	// deliberately not wired here: on a phone there is nothing to hide.
	window.addEventListener('mousemove', restart);
	window.addEventListener('mousedown', restart);
	window.addEventListener('wheel', restart, { passive: true });
	document.addEventListener('fullscreenchange', restart);

	restart();

	return () => {
		clear();
		show();
		window.removeEventListener('mousemove', restart);
		window.removeEventListener('mousedown', restart);
		window.removeEventListener('wheel', restart);
		document.removeEventListener('fullscreenchange', restart);
	};
}
