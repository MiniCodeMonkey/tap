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
