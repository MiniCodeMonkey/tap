/**
 * Helpers for slides with `skip: true`. A skipped slide stays in the deck,
 * so slide numbers in URLs and messages keep matching the markdown, but
 * presenting passes over it and slide counts leave it out.
 */

import type { Slide } from '$lib/types';

/** Whether a slide is left out of presenting. */
export function isSkipped(slide: Slide | null | undefined): boolean {
	return slide?.skip === true;
}

/** How many slides a talk shows: every slide that is not skipped. */
export function presentedSlideCount(slides: readonly Slide[]): number {
	return slides.filter((slide) => !isSkipped(slide)).length;
}

/**
 * How many presented slides come at or before index. For a skipped slide
 * this is the number of the presented slide before it.
 */
export function presentedSlidesThrough(slides: readonly Slide[], index: number): number {
	let count = 0;
	for (let position = 0; position <= index && position < slides.length; position++) {
		if (!isSkipped(slides[position])) {
			count++;
		}
	}
	return count;
}

/**
 * The 1-based number the audience sees for the slide at index: its place
 * among the slides that are not skipped. Null for a skipped slide and for
 * an index outside the deck.
 */
export function presentedSlideNumber(slides: readonly Slide[], index: number): number | null {
	const slide = slides[index];
	if (!slide || isSkipped(slide)) {
		return null;
	}
	return presentedSlidesThrough(slides, index);
}

/** The index of the first slide after fromIndex that is not skipped, or null. */
export function nextPresentedIndex(slides: readonly Slide[], fromIndex: number): number | null {
	for (let index = fromIndex + 1; index < slides.length; index++) {
		if (!isSkipped(slides[index])) {
			return index;
		}
	}
	return null;
}

/** The index of the last slide before fromIndex that is not skipped, or null. */
export function previousPresentedIndex(slides: readonly Slide[], fromIndex: number): number | null {
	for (let index = Math.min(fromIndex, slides.length) - 1; index >= 0; index--) {
		if (!isSkipped(slides[index])) {
			return index;
		}
	}
	return null;
}

/** The index of the first slide that is not skipped, or null. */
export function firstPresentedIndex(slides: readonly Slide[]): number | null {
	return nextPresentedIndex(slides, -1);
}

/** The index of the last slide that is not skipped, or null. */
export function lastPresentedIndex(slides: readonly Slide[]): number | null {
	return previousPresentedIndex(slides, slides.length);
}
