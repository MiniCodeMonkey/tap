import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/react';
import { SlideOverview } from './SlideOverview';
import { resetPresentation, usePresentationStore } from '$lib/stores/presentation';
import { broadcastPresentationState } from '$lib/stores/websocket';
import type { Slide } from '$lib/types';

vi.mock('$lib/stores/websocket', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/stores/websocket')>();
	return {
		...actual,
		broadcastPresentationState: vi.fn()
	};
});

function makeSlides(count: number): Slide[] {
	return Array.from({ length: count }, (_, index) => ({
		index,
		layout: 'default',
		html: `<p>Slide ${index + 1}</p>`,
		slots: { default: `<p>Slide ${index + 1}</p>` },
		slotOrder: ['default'],
		fragmentCount: 0,
		steps: 0
	}));
}

afterEach(() => {
	cleanup();
	resetPresentation();
	vi.clearAllMocks();
});

describe('SlideOverview', () => {
	it('renders nothing when closed', () => {
		const { container } = render(<SlideOverview slides={makeSlides(3)} isOpen={false} />);
		expect(container.firstChild).toBeNull();
	});

	it('renders one thumbnail per slide', () => {
		const slides = makeSlides(6);
		const { container } = render(<SlideOverview slides={slides} isOpen />);

		expect(container.querySelectorAll('.thumbnail')).toHaveLength(6);
	});

	it('navigates to the clicked slide via goToSlide', () => {
		usePresentationStore.setState({ presentation: { config: {}, slides: makeSlides(4) }, currentSlideIndex: 0 });
		const slides = makeSlides(4);

		const { container } = render(<SlideOverview slides={slides} isOpen />);

		const thumbnails = container.querySelectorAll('.thumbnail');
		fireEvent.click(thumbnails[2]);

		expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
	});

	it('calls onClose after selecting a slide', () => {
		usePresentationStore.setState({ presentation: { config: {}, slides: makeSlides(4) }, currentSlideIndex: 0 });
		const slides = makeSlides(4);
		let closed = false;

		const { container } = render(<SlideOverview slides={slides} isOpen onClose={() => (closed = true)} />);

		fireEvent.click(container.querySelectorAll('.thumbnail')[1]);

		expect(closed).toBe(true);
	});

	it('broadcasts the new state after selecting a slide, mirroring keyboard navigation', () => {
		usePresentationStore.setState({ presentation: { config: {}, slides: makeSlides(4) }, currentSlideIndex: 0 });
		const slides = makeSlides(4);

		const { container } = render(<SlideOverview slides={slides} isOpen />);

		fireEvent.click(container.querySelectorAll('.thumbnail')[2]);

		expect(broadcastPresentationState).toHaveBeenCalledTimes(1);
	});

	it('marks the current slide with the current class', () => {
		usePresentationStore.setState({ presentation: { config: {}, slides: makeSlides(3) }, currentSlideIndex: 1 });
		const slides = makeSlides(3);

		const { container } = render(<SlideOverview slides={slides} isOpen />);

		const thumbnails = container.querySelectorAll('.thumbnail');
		expect(thumbnails[1]).toHaveClass('current');
		expect(thumbnails[0]).not.toHaveClass('current');
	});
});
