/**
 * After an in-place update, a list of slides re-renders only the slides
 * whose content hash changed.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render } from '@testing-library/react';
import { Slide } from './Slide';
import { resolveLayout } from '../layouts/registry';
import {
	loadPresentation,
	resetPresentation,
	slideKey,
	updatePresentationInPlace,
	usePresentationStore
} from '$lib/stores/presentation';
import type { LayoutProps, Presentation, Slide as SlideData } from '$lib/types';

vi.mock('../layouts/registry', async (importOriginal) => {
	const actual = await importOriginal<typeof import('../layouts/registry')>();
	return { ...actual, resolveLayout: vi.fn(actual.resolveLayout) };
});

const renderedSlideNumbers: number[] = [];

function CountingLayout({ slide }: LayoutProps) {
	renderedSlideNumbers.push(slide.index + 1);
	return <p>{slide.html}</p>;
}

function makeSlide(index: number, hash: string): SlideData {
	return { index, layout: 'default', html: hash, slots: {}, slotOrder: [], fragmentCount: 0, steps: 0, hash };
}

function makeDeck(revision: string, hashes: string[]): Presentation {
	return { config: { title: 'Deck' }, revision, slides: hashes.map((hash, index) => makeSlide(index, hash)) };
}

function SlideList() {
	const slides = usePresentationStore((state) => state.presentation?.slides ?? []);
	return (
		<>
			{slides.map((slide) => (
				<Slide
					key={slideKey(slide)}
					slide={slide}
					active={false}
					printMode={false}
					fragmentIndex={-1}
					step={0}
					total={slides.length}
				/>
			))}
		</>
	);
}

beforeEach(() => {
	resetPresentation();
	renderedSlideNumbers.length = 0;
	vi.mocked(resolveLayout).mockReturnValue({ component: CountingLayout, slots: ['default'] });
});

afterEach(() => {
	cleanup();
	vi.mocked(resolveLayout).mockReset();
});

describe('Slide after an in-place update', () => {
	it('re-renders only the slide whose content changed', () => {
		loadPresentation(makeDeck('r1', ['a', 'b', 'c']));
		render(<SlideList />);
		expect(new Set(renderedSlideNumbers)).toEqual(new Set([1, 2, 3]));

		renderedSlideNumbers.length = 0;
		act(() => {
			updatePresentationInPlace(makeDeck('r2', ['a', 'b2', 'c']));
		});

		expect(new Set(renderedSlideNumbers)).toEqual(new Set([2]));
	});

	it('re-renders nothing when no slide changed', () => {
		loadPresentation(makeDeck('r1', ['a', 'b']));
		render(<SlideList />);

		renderedSlideNumbers.length = 0;
		act(() => {
			updatePresentationInPlace(makeDeck('r2', ['a', 'b']));
		});

		expect(renderedSlideNumbers).toEqual([]);
	});
});
