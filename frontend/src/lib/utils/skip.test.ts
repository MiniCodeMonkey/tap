import { describe, expect, it } from 'vitest';
import type { Slide } from '$lib/types';
import {
	firstPresentedIndex,
	isSkipped,
	lastPresentedIndex,
	nextPresentedIndex,
	presentedSlideCount,
	presentedSlideNumber,
	presentedSlidesThrough,
	previousPresentedIndex
} from './skip';

function makeSlides(skipped: boolean[]): Slide[] {
	return skipped.map((skip, index) => ({
		index,
		layout: 'default',
		html: '',
		slots: {},
		slotOrder: [],
		fragmentCount: 0,
		steps: 0,
		skip
	}));
}

describe('skip helpers', () => {
	const slides = makeSlides([true, false, true, false, false, true]);

	it('knows which slides are skipped', () => {
		expect(slides.map((slide) => isSkipped(slide))).toEqual([true, false, true, false, false, true]);
		expect(isSkipped(null)).toBe(false);
		expect(isSkipped(makeSlides([false])[0])).toBe(false);
	});

	it('counts the slides a talk shows', () => {
		expect(presentedSlideCount(slides)).toBe(3);
		expect(presentedSlideCount([])).toBe(0);
	});

	it('numbers a slide among the slides a talk shows', () => {
		expect(presentedSlideNumber(slides, 1)).toBe(1);
		expect(presentedSlideNumber(slides, 3)).toBe(2);
		expect(presentedSlideNumber(slides, 4)).toBe(3);
		expect(presentedSlideNumber(slides, 2)).toBeNull();
		expect(presentedSlideNumber(slides, 9)).toBeNull();
	});

	it('counts the presented slides up to a skipped one', () => {
		expect(presentedSlidesThrough(slides, 2)).toBe(1);
		expect(presentedSlidesThrough(slides, 5)).toBe(3);
	});

	it('finds the next and previous presented slides', () => {
		expect(nextPresentedIndex(slides, 1)).toBe(3);
		expect(nextPresentedIndex(slides, 4)).toBeNull();
		expect(previousPresentedIndex(slides, 3)).toBe(1);
		expect(previousPresentedIndex(slides, 1)).toBeNull();
		expect(firstPresentedIndex(slides)).toBe(1);
		expect(lastPresentedIndex(slides)).toBe(4);
		expect(firstPresentedIndex(makeSlides([true, true]))).toBeNull();
	});
});
