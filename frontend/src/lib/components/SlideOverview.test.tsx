import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render } from '@testing-library/react';
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
	describe('column count', () => {
		it('leaves the column count to the stylesheet', () => {
			// An inline custom property would beat every media query, which is
			// how the grid ended up five columns wide on a phone.
			const { container } = render(<SlideOverview slides={makeSlides(6)} isOpen />);
			const grid = container.querySelector('.thumbnail-grid') as HTMLElement;
			expect(grid.style.getPropertyValue('--grid-columns')).toBe('');
		});

		it('moves down by the measured number of columns', () => {
			vi.spyOn(window, 'getComputedStyle').mockReturnValue({
				gridTemplateColumns: '100px 100px'
			} as unknown as CSSStyleDeclaration);

			const { container } = render(<SlideOverview slides={makeSlides(9)} isOpen />);
			const dialog = container.querySelector('.slide-overview') as HTMLElement;

			fireEvent.keyDown(dialog, { key: 'ArrowDown' });
			expect(container.querySelectorAll('.thumbnail')[2]?.className).toContain('focused');

			vi.mocked(window.getComputedStyle).mockRestore();
		});
	});

	describe('lazy thumbnails', () => {
		/** Capture the observers a render creates, so a test can drive them. */
		function stubIntersectionObserver(): {
			trigger: (isIntersecting: boolean) => void;
			disconnects: () => number;
		} {
			const callbacks: ((entries: { isIntersecting: boolean }[]) => void)[] = [];
			let disconnected = 0;

			class StubObserver {
				constructor(callback: (entries: { isIntersecting: boolean }[]) => void) {
					callbacks.push(callback);
				}
				observe(): void {}
				unobserve(): void {}
				disconnect(): void {
					disconnected += 1;
				}
			}

			vi.stubGlobal('IntersectionObserver', StubObserver);

			return {
				trigger: (isIntersecting: boolean) => {
					for (const callback of callbacks) {
						callback([{ isIntersecting }]);
					}
				},
				disconnects: () => disconnected
			};
		}

		afterEach(() => {
			vi.unstubAllGlobals();
		});

		it('renders no slide content until a thumbnail comes near the viewport', () => {
			stubIntersectionObserver();
			const { container } = render(<SlideOverview slides={makeSlides(40)} isOpen />);

			expect(container.querySelectorAll('.thumbnail')).toHaveLength(40);
			expect(container.querySelector('.slide-container')).toBeNull();
		});

		it('renders the slide once the thumbnail intersects, and drops it again', async () => {
			const observer = stubIntersectionObserver();
			const { container } = render(<SlideOverview slides={makeSlides(3)} isOpen />);

			await act(async () => observer.trigger(true));
			expect(container.querySelectorAll('.slide-container').length).toBeGreaterThan(0);

			await act(async () => observer.trigger(false));
			expect(container.querySelector('.slide-container')).toBeNull();
		});

		it('disconnects its observers when the overview unmounts', async () => {
			const observer = stubIntersectionObserver();
			const { unmount } = render(<SlideOverview slides={makeSlides(3)} isOpen />);

			unmount();
			expect(observer.disconnects()).toBe(3);
		});

		it('renders every thumbnail where IntersectionObserver is missing', () => {
			vi.stubGlobal('IntersectionObserver', undefined);
			const { container } = render(<SlideOverview slides={makeSlides(3)} isOpen />);
			expect(container.querySelectorAll('.slide-container').length).toBeGreaterThan(0);
		});
	});

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

	describe('scrolling', () => {
		function spyOnScroll() {
			const calls: { element: Element; options: ScrollIntoViewOptions | undefined }[] = [];
			Element.prototype.scrollIntoView = vi.fn(function (this: Element, options?: boolean | ScrollIntoViewOptions) {
				calls.push({ element: this, options: typeof options === 'object' ? options : undefined });
			});
			return calls;
		}

		afterEach(() => {
			// jsdom has no scrollIntoView of its own; the component tolerates that.
			delete (Element.prototype as { scrollIntoView?: unknown }).scrollIntoView;
		});

		it('centers the current slide each time the overview opens', () => {
			usePresentationStore.setState({ presentation: { config: {}, slides: makeSlides(40) }, currentSlideIndex: 27 });
			const calls = spyOnScroll();

			const { container, rerender } = render(<SlideOverview slides={makeSlides(40)} isOpen />);
			const centered = () => calls.filter((call) => call.options?.block === 'center');
			expect(centered()).toHaveLength(1);
			expect(centered()[0].element).toBe(container.querySelectorAll('.thumbnail')[27]);

			rerender(<SlideOverview slides={makeSlides(40)} isOpen={false} />);
			act(() => usePresentationStore.setState({ currentSlideIndex: 3 }));
			rerender(<SlideOverview slides={makeSlides(40)} isOpen />);

			expect(centered()).toHaveLength(2);
			expect(centered()[1].element).toBe(container.querySelectorAll('.thumbnail')[3]);
		});

		it('never scrolls to the previous opening\'s focus position on reopen', () => {
			usePresentationStore.setState({ presentation: { config: {}, slides: makeSlides(40) }, currentSlideIndex: 27 });
			const calls = spyOnScroll();

			const { container, rerender } = render(<SlideOverview slides={makeSlides(40)} isOpen />);
			rerender(<SlideOverview slides={makeSlides(40)} isOpen={false} />);
			act(() => usePresentationStore.setState({ currentSlideIndex: 1 }));
			calls.length = 0;
			rerender(<SlideOverview slides={makeSlides(40)} isOpen />);

			const thumbnails = container.querySelectorAll('.thumbnail');
			expect(calls.length).toBeGreaterThan(0);
			expect(calls.every((call) => call.element === thumbnails[1])).toBe(true);
		});

		it('keeps the focus ring in view as the arrow keys move it', () => {
			usePresentationStore.setState({ presentation: { config: {}, slides: makeSlides(40) }, currentSlideIndex: 0 });
			const calls = spyOnScroll();

			const { container } = render(<SlideOverview slides={makeSlides(40)} isOpen />);
			const dialog = container.querySelector('.slide-overview')!;
			fireEvent.keyDown(dialog, { key: 'End' });

			const last = calls[calls.length - 1];
			expect(last.element).toBe(container.querySelectorAll('.thumbnail')[39]);
			expect(last.options?.block).toBe('nearest');
		});

		it('opens without error where scrollIntoView does not exist', () => {
			usePresentationStore.setState({ presentation: { config: {}, slides: makeSlides(3) }, currentSlideIndex: 2 });

			expect(() => render(<SlideOverview slides={makeSlides(3)} isOpen />)).not.toThrow();
		});
	});

	it('dims a skipped slide and numbers the others among the presented slides', () => {
		const slides = makeSlides(3).map((slide, index) => ({ ...slide, skip: index === 1 }));
		const { container } = render(<SlideOverview slides={slides} isOpen />);

		const thumbnails = container.querySelectorAll('.thumbnail');
		expect(thumbnails[1].classList.contains('skipped')).toBe(true);
		expect(thumbnails[0].classList.contains('skipped')).toBe(false);
		expect(thumbnails[1].querySelector('.thumbnail-number')?.textContent).toBe('Skipped');
		expect(thumbnails[2].querySelector('.thumbnail-number')?.textContent).toBe('2');
		// A skipped slide has no presented number, so its aria-label states
		// no number at all, the same as its visible "Skipped" label - never
		// a deck position that could coincide with another slide's presented
		// number (here, slide 3's presented number is also 2).
		expect(thumbnails[1].getAttribute('aria-label')).toBe('Skipped slide');
		expect(thumbnails[2].getAttribute('aria-label')).toBe('Slide 2');
		const ariaLabels = Array.from(thumbnails).map((thumbnail) => thumbnail.getAttribute('aria-label'));
		expect(new Set(ariaLabels).size).toBe(ariaLabels.length);
	});
});
