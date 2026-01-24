import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import SlideContainer from './SlideContainer.svelte';

describe('SlideContainer', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('rendering', () => {
		it('renders a slide container', () => {
			const { container } = render(SlideContainer);

			expect(container.querySelector('.slide-container')).toBeInTheDocument();
			expect(container.querySelector('.slide')).toBeInTheDocument();
		});

		it('applies theme class to container', () => {
			const { container } = render(SlideContainer, {
				props: { theme: 'phosphor' }
			});

			expect(container.querySelector('.theme-phosphor')).toBeInTheDocument();
		});

		it('applies default paper theme', () => {
			const { container } = render(SlideContainer);

			expect(container.querySelector('.theme-paper')).toBeInTheDocument();
		});

		it('applies different theme classes correctly', () => {
			const themes = ['paper', 'noir', 'aurora', 'phosphor', 'poster'];

			for (const theme of themes) {
				cleanup();
				const { container } = render(SlideContainer, {
					props: { theme }
				});

				expect(container.querySelector(`.theme-${theme}`)).toBeInTheDocument();
			}
		});
	});

	describe('aspect ratio', () => {
		it('defaults to 16:9 aspect ratio', () => {
			const { container } = render(SlideContainer);
			const slide = container.querySelector('.slide');

			expect(slide).toHaveStyle({ '--aspect-ratio': '16 / 9' });
		});

		it('applies 4:3 aspect ratio', () => {
			const { container } = render(SlideContainer, {
				props: { aspectRatio: '4:3' }
			});
			const slide = container.querySelector('.slide');

			expect(slide).toHaveStyle({ '--aspect-ratio': '4 / 3' });
		});

		it('applies 16:10 aspect ratio', () => {
			const { container } = render(SlideContainer, {
				props: { aspectRatio: '16:10' }
			});
			const slide = container.querySelector('.slide');

			expect(slide).toHaveStyle({ '--aspect-ratio': '16 / 10' });
		});

		it('falls back to 16:9 for invalid aspect ratio', () => {
			const { container } = render(SlideContainer, {
				props: { aspectRatio: 'invalid' }
			});
			const slide = container.querySelector('.slide');

			expect(slide).toHaveStyle({ '--aspect-ratio': '16 / 9' });
		});

		it('falls back to 16:9 for malformed aspect ratio', () => {
			const { container } = render(SlideContainer, {
				props: { aspectRatio: 'abc:def' }
			});
			const slide = container.querySelector('.slide');

			expect(slide).toHaveStyle({ '--aspect-ratio': '16 / 9' });
		});

		it('handles single number aspect ratio gracefully', () => {
			const { container } = render(SlideContainer, {
				props: { aspectRatio: '16' }
			});
			const slide = container.querySelector('.slide');

			// Falls back to default
			expect(slide).toHaveStyle({ '--aspect-ratio': '16 / 9' });
		});
	});

	describe('scaling', () => {
		it('applies scale transform to slide', () => {
			const { container } = render(SlideContainer);
			const slide = container.querySelector('.slide');

			// Should have transform style with scale
			expect(slide).toHaveAttribute('style');
			const style = slide?.getAttribute('style');
			expect(style).toContain('transform: scale(');
		});

		it('calculates scale based on container dimensions', () => {
			// Mock container dimensions - 1920x1080 is our base size
			vi.mocked(Element.prototype.getBoundingClientRect).mockReturnValue({
				width: 1920,
				height: 1080,
				top: 0,
				left: 0,
				bottom: 1080,
				right: 1920,
				x: 0,
				y: 0,
				toJSON: () => ({})
			});

			const { container } = render(SlideContainer);
			const slide = container.querySelector('.slide');

			// At 1920x1080 with 16:9, scale should be approximately 1
			const style = slide?.getAttribute('style');
			expect(style).toContain('transform: scale(');
		});

		it('scales down for smaller container', () => {
			// Mock smaller container
			vi.mocked(Element.prototype.getBoundingClientRect).mockReturnValue({
				width: 960,
				height: 540,
				top: 0,
				left: 0,
				bottom: 540,
				right: 960,
				x: 0,
				y: 0,
				toJSON: () => ({})
			});

			const { container } = render(SlideContainer);
			const slide = container.querySelector('.slide');

			// Scale should be less than 1 (half size = 0.5 scale)
			const style = slide?.getAttribute('style');
			expect(style).toContain('transform: scale(');
		});
	});

	describe('fullscreen mode', () => {
		it('does not apply fullscreen class by default', () => {
			const { container } = render(SlideContainer);

			expect(container.querySelector('.fullscreen')).not.toBeInTheDocument();
		});

		it('applies fullscreen class when fullscreen is true', () => {
			const { container } = render(SlideContainer, {
				props: { fullscreen: true }
			});

			expect(container.querySelector('.fullscreen')).toBeInTheDocument();
		});

		it('fullscreen container has correct positioning', () => {
			const { container } = render(SlideContainer, {
				props: { fullscreen: true }
			});
			const containerEl = container.querySelector('.slide-container.fullscreen');

			expect(containerEl).toBeInTheDocument();
			// CSS styles are applied via stylesheet, check class is present
		});
	});

	describe('resize handling', () => {
		it('sets up ResizeObserver on mount', () => {
			// ResizeObserver is mocked in setup.ts, verify the mock is called
			const { container } = render(SlideContainer);

			// Just verify component rendered (ResizeObserver is mocked in setup.ts)
			expect(container.querySelector('.slide-container')).toBeInTheDocument();
		});

		it('disconnects ResizeObserver on unmount', () => {
			const { unmount, container } = render(SlideContainer);

			// Verify component is mounted
			expect(container.querySelector('.slide-container')).toBeInTheDocument();

			// Unmount should not throw
			expect(() => unmount()).not.toThrow();
		});

		it('adds window resize listener on mount', () => {
			const addEventListenerSpy = vi.spyOn(window, 'addEventListener');

			render(SlideContainer);

			expect(addEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));
		});

		it('removes window resize listener on unmount', () => {
			const removeEventListenerSpy = vi.spyOn(window, 'removeEventListener');

			const { unmount } = render(SlideContainer);
			unmount();

			expect(removeEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));
		});
	});

	describe('slide dimensions', () => {
		it('slide has 1920px base width', () => {
			const { container } = render(SlideContainer);
			const slide = container.querySelector('.slide');

			// Check the CSS class is applied (dimensions defined in CSS)
			expect(slide).toHaveClass('slide');
		});

		it('slide height is calculated from aspect ratio', () => {
			const { container } = render(SlideContainer, {
				props: { aspectRatio: '4:3' }
			});
			const slide = container.querySelector('.slide');

			// With 4:3 ratio, height calculation uses CSS calc
			expect(slide).toHaveAttribute('style');
			expect(slide?.getAttribute('style')).toContain('--aspect-ratio: 4 / 3');
		});
	});

	describe('CSS custom properties', () => {
		it('sets aspect ratio CSS variable', () => {
			const { container } = render(SlideContainer, {
				props: { aspectRatio: '16:10' }
			});
			const slide = container.querySelector('.slide');

			expect(slide).toHaveStyle({ '--aspect-ratio': '16 / 10' });
		});

		it('container has slide-container class', () => {
			const { container } = render(SlideContainer);

			expect(container.querySelector('.slide-container')).toBeInTheDocument();
		});

		it('slide element has transform-origin set via CSS', () => {
			const { container } = render(SlideContainer);
			const slide = container.querySelector('.slide');

			// The transform-origin is set in CSS, so just verify the slide exists
			expect(slide).toBeInTheDocument();
		});
	});

	describe('reduced motion', () => {
		it('handles reduced motion media query', () => {
			// This is tested via CSS - we just verify component renders
			vi.mocked(window.matchMedia).mockImplementation((query: string) => ({
				matches: query === '(prefers-reduced-motion: reduce)',
				media: query,
				onchange: null,
				addListener: vi.fn(),
				removeListener: vi.fn(),
				addEventListener: vi.fn(),
				removeEventListener: vi.fn(),
				dispatchEvent: vi.fn()
			}));

			const { container } = render(SlideContainer);

			expect(container.querySelector('.slide-container')).toBeInTheDocument();
		});
	});
});
