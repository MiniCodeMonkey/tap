/**
 * Picks the largest font size at which a block of text still fits its box.
 *
 * The caller supplies the measurement, so the search itself is pure and the
 * DOM work stays in the hook that uses it. Sizes are snapped to a step so the
 * result is one of the same values the A- and A+ buttons produce.
 */

export interface FitTextOptions {
	/** Smallest size to consider, and the answer when nothing fits. */
	minSize: number;
	/** Largest size to consider. */
	maxSize: number;
	/** Resolution of the search, in the same unit as the sizes. */
	step: number;
	/** True when the text fits its box at this size. */
	fits: (size: number) => boolean;
}

export function findFittingSize({ minSize, maxSize, step, fits }: FitTextOptions): number {
	if (maxSize <= minSize) return minSize;

	const stepCount = Math.max(1, Math.round((maxSize - minSize) / step));
	const sizeAt = (index: number): number => Math.min(maxSize, minSize + index * step);

	// The common case during a talk is notes that fit at the cap, so try the
	// cap before searching.
	if (fits(sizeAt(stepCount))) return sizeAt(stepCount);

	let low = 0;
	let high = stepCount - 1;
	while (low < high) {
		const middle = Math.ceil((low + high) / 2);
		if (fits(sizeAt(middle))) {
			low = middle;
		} else {
			high = middle - 1;
		}
	}
	return sizeAt(low);
}

/**
 * Bounds for fitting, and the scale applied to the result.
 *
 * Fitting searches up to NOTES_FIT_MAX_SIZE rather than the manual reading
 * size: the manual size answers "how big do I want to read" and tops out at
 * 3rem, while fitting has to be free to fill a notes-first panel across a
 * laptop screen.
 *
 * A- and A+ then scale that result rather than moving a ceiling. A ceiling
 * would do nothing until it dropped below the size the current slide already
 * fits at, and it would mean different things on a slide with two lines of
 * notes and one with twenty. A scale is a notch smaller everywhere.
 */
export const NOTES_FIT_MIN_SIZE = 1;
export const NOTES_FIT_MAX_SIZE = 6;
export const NOTES_FIT_SCALE_MIN = 0.5;
export const NOTES_FIT_SCALE_MAX = 1;
export const NOTES_FIT_SCALE_STEP = 0.1;
export const NOTES_FIT_SCALE_STORAGE_KEY = 'tap-presenter-notes-fit-scale';

export function clampFitScale(scale: number): number {
	const bounded = Math.min(NOTES_FIT_SCALE_MAX, Math.max(NOTES_FIT_SCALE_MIN, scale));
	// Stepping by 0.1 accumulates float noise; one decimal is the resolution.
	return Math.round(bounded * 10) / 10;
}

export function readStoredFitScale(): number {
	try {
		const stored = Number.parseFloat(
			window.localStorage.getItem(NOTES_FIT_SCALE_STORAGE_KEY) ?? ''
		);
		return Number.isFinite(stored) ? clampFitScale(stored) : NOTES_FIT_SCALE_MAX;
	} catch {
		// Storage can be unavailable (private window, blocked site data).
		return NOTES_FIT_SCALE_MAX;
	}
}

export function writeStoredFitScale(scale: number): void {
	try {
		window.localStorage.setItem(NOTES_FIT_SCALE_STORAGE_KEY, String(scale));
	} catch {
		// The scale then lasts only until reload.
	}
}
