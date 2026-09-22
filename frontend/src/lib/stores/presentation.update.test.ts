/**
 * Replacing the deck in place, after an "update" message: the position is
 * kept and clamped, and unchanged slides keep their objects.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	loadPresentation,
	resetPresentation,
	slideKey,
	updatePresentationInPlace,
	usePresentationStore
} from './presentation';
import type { Presentation, Slide } from '$lib/types';

function makeSlide(index: number, hash: string | undefined, overrides: Partial<Slide> = {}): Slide {
	return {
		index,
		layout: 'default',
		html: `<p>${hash ?? 'no hash'}</p>`,
		slots: {},
		slotOrder: [],
		fragmentCount: 0,
		steps: 0,
		hash,
		...overrides
	};
}

function makeDeck(revision: string, slides: Slide[]): Presentation {
	return { config: { title: 'Deck' }, slides, revision };
}

const replaceState = vi.fn();

beforeEach(() => {
	vi.stubGlobal('window', {
		location: { hash: '', search: '', pathname: '/' },
		history: { replaceState }
	});
	replaceState.mockClear();
	resetPresentation();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('updatePresentationInPlace', () => {
	it('keeps the slide, fragment and step', () => {
		loadPresentation(
			makeDeck('r1', [makeSlide(0, 'a'), makeSlide(1, 'b', { steps: 3, fragmentCount: 2 })])
		);
		usePresentationStore.setState({ currentSlideIndex: 1, currentStep: 2, currentFragmentIndex: 1 });

		updatePresentationInPlace(
			makeDeck('r2', [makeSlide(0, 'a2'), makeSlide(1, 'b', { steps: 3, fragmentCount: 2 })])
		);

		const state = usePresentationStore.getState();
		expect(state.presentation?.revision).toBe('r2');
		expect(state.currentSlideIndex).toBe(1);
		expect(state.currentStep).toBe(2);
		expect(state.currentFragmentIndex).toBe(1);
	});

	it('clamps the step when the slide loses steps', () => {
		loadPresentation(makeDeck('r1', [makeSlide(0, 'a', { steps: 4 })]));
		usePresentationStore.setState({ currentStep: 4 });

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a2', { steps: 1 })]));

		expect(usePresentationStore.getState().currentStep).toBe(1);
	});

	it('clamps the step from the fetched counts even when the slide keeps its old hash and object', () => {
		// A slide's hash covers only its own content, not the deck around it,
		// so an unchanged hash at this index does not guarantee its step
		// count is unchanged too. The clamp must still read the freshly
		// fetched slide's steps, not the reused object's.
		loadPresentation(makeDeck('r1', [makeSlide(0, 'a', { steps: 4 })]));
		usePresentationStore.setState({ currentStep: 4 });

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a', { steps: 1 })]));

		expect(usePresentationStore.getState().currentStep).toBe(1);
	});

	it('clamps the fragment when the slide loses fragments', () => {
		loadPresentation(makeDeck('r1', [makeSlide(0, 'a', { fragmentCount: 3 })]));
		usePresentationStore.setState({ currentFragmentIndex: 2 });

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a2', { fragmentCount: 0 })]));

		expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
	});

	it('moves to the last slide when the current one was removed, and updates the URL hash', () => {
		loadPresentation(makeDeck('r1', [makeSlide(0, 'a'), makeSlide(1, 'b'), makeSlide(2, 'c')]));
		usePresentationStore.setState({ currentSlideIndex: 2 });
		replaceState.mockClear();

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a'), makeSlide(1, 'b')]));

		expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
		expect(replaceState).toHaveBeenCalledWith(null, '', '#2');
	});

	it('does not touch the URL hash when the slide stays the same', () => {
		loadPresentation(makeDeck('r1', [makeSlide(0, 'a'), makeSlide(1, 'b')]));
		usePresentationStore.setState({ currentSlideIndex: 1 });
		replaceState.mockClear();

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a2'), makeSlide(1, 'b')]));

		expect(replaceState).not.toHaveBeenCalled();
	});

	it('keeps the object of every slide whose hash did not change', () => {
		const first = makeDeck('r1', [makeSlide(0, 'a'), makeSlide(1, 'b'), makeSlide(2, 'c')]);
		loadPresentation(first);

		const next = makeDeck('r2', [makeSlide(0, 'a'), makeSlide(1, 'b2'), makeSlide(2, 'c')]);
		updatePresentationInPlace(next);

		const slides = usePresentationStore.getState().presentation?.slides ?? [];
		expect(slides[0]).toBe(first.slides[0]);
		expect(slides[1]).toBe(next.slides[1]);
		expect(slides[2]).toBe(first.slides[2]);
	});

	it('replaces a slide that has no hash', () => {
		const first = makeDeck('r1', [makeSlide(0, undefined)]);
		loadPresentation(first);

		const next = makeDeck('r2', [makeSlide(0, undefined)]);
		updatePresentationInPlace(next);

		expect(usePresentationStore.getState().presentation?.slides[0]).toBe(next.slides[0]);
	});

	it('replaces a slide with an empty hash, even when the previous slide also had an empty hash', () => {
		// An empty hash means SlideHash (internal/transformer) failed to
		// marshal the slide, so two empty hashes must never compare equal -
		// that would report a genuinely changed slide as unchanged and it
		// would never re-render.
		const first = makeDeck('r1', [makeSlide(0, '')]);
		loadPresentation(first);

		const next = makeDeck('r2', [makeSlide(0, '')]);
		updatePresentationInPlace(next);

		expect(usePresentationStore.getState().presentation?.slides[0]).toBe(next.slides[0]);
	});

	it('uses the new slide when a slide moved to another position', () => {
		const first = makeDeck('r1', [makeSlide(0, 'a'), makeSlide(1, 'b')]);
		loadPresentation(first);

		const next = makeDeck('r2', [makeSlide(0, 'b'), makeSlide(1, 'a')]);
		updatePresentationInPlace(next);

		const slides = usePresentationStore.getState().presentation?.slides ?? [];
		expect(slides[0]).toBe(next.slides[0]);
		expect(slides[1]).toBe(next.slides[1]);
	});

	it('keeps the config object when the config did not change', () => {
		const first = makeDeck('r1', [makeSlide(0, 'a')]);
		loadPresentation(first);

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a2')]));

		expect(usePresentationStore.getState().presentation?.config).toBe(first.config);
	});
});

describe('slideKey', () => {
	it('combines the position and the hash', () => {
		expect(slideKey(makeSlide(4, 'abc'))).toBe('4:abc');
	});

	it('falls back to the position for a slide with no hash', () => {
		expect(slideKey(makeSlide(4, undefined))).toBe('4');
	});
});
