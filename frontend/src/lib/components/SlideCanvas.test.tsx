import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { SlideCanvas } from './SlideCanvas';

afterEach(cleanup);

describe('SlideCanvas', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('rendering', () => {
		it('renders a slide container and slide', () => {
			const { container } = render(<SlideCanvas />);

			expect(container.querySelector('.slide-container')).toBeInTheDocument();
			expect(container.querySelector('.slide')).toBeInTheDocument();
		});

		it('lays the frame out at a fixed 1920px width and never lets it shrink as a flex item', () => {
			// .slide-container is a flex container; without flex: none the
			// frame (its direct child) can shrink below its inline
			// width/height whenever the container is narrower than 1920px
			// (any sub-1920 viewport, a presenter panel, an overview
			// thumbnail). Only the scale() transform, not layout, should
			// ever make the frame look smaller.
			const { container } = render(<SlideCanvas />);
			const slide = container.querySelector('.slide') as HTMLElement;

			expect(slide.style.width).toBe('1920px');
			expect(slide.style.height).toBe('1080px');
			// jsdom expands the `flex: 'none'` shorthand into its longhands.
			expect(slide.style.flex).toBe('0 0 auto');
		});

		it('sizes the frame for a 4:3 aspect ratio', () => {
			const { container } = render(<SlideCanvas aspectRatio="4:3" />);
			const slide = container.querySelector('.slide') as HTMLElement;

			expect(slide.style.width).toBe('1920px');
			expect(slide.style.height).toBe('1440px');
		});

		it('applies the given theme as data-theme on the 1920px frame, not the letterbox container', () => {
			// data-theme lives on the frame so a theme's background never
			// paints past the fixed canvas into the letterbox area around it
			// (see app.css and the SlideCanvas render comment).
			const { container } = render(<SlideCanvas theme="base" />);

			expect(container.querySelector('.slide')).toHaveAttribute('data-theme', 'base');
			expect(container.querySelector('.slide-container')).not.toHaveAttribute('data-theme');
		});

		it('defaults to the base theme', () => {
			const { container } = render(<SlideCanvas />);

			expect(container.querySelector('.slide')).toHaveAttribute('data-theme', 'base');
		});

		it('sets data-print="true" on the same element as data-theme when printMode is on', () => {
			const { container } = render(<SlideCanvas theme="base" printMode />);
			const frame = container.querySelector('.slide');

			expect(frame).toHaveAttribute('data-print', 'true');
			expect(frame).toHaveAttribute('data-theme', 'base');
		});

		it('omits data-print when printMode is off', () => {
			const { container } = render(<SlideCanvas />);

			expect(container.querySelector('.slide')).not.toHaveAttribute('data-print');
		});
	});

	describe('aspect ratio', () => {
		it('defaults to 16:9 aspect ratio', () => {
			const { container } = render(<SlideCanvas />);
			const slide = container.querySelector('.slide');

			expect(slide?.getAttribute('style')).toContain('aspect-ratio: 16 / 9');
		});

		it('applies a 4:3 aspect ratio', () => {
			const { container } = render(<SlideCanvas aspectRatio="4:3" />);
			const slide = container.querySelector('.slide');

			expect(slide?.getAttribute('style')).toContain('aspect-ratio: 4 / 3');
		});

		it('applies a 16:10 aspect ratio', () => {
			const { container } = render(<SlideCanvas aspectRatio="16:10" />);
			const slide = container.querySelector('.slide');

			expect(slide?.getAttribute('style')).toContain('aspect-ratio: 16 / 10');
		});

		it('falls back to 16:9 for an invalid aspect ratio', () => {
			const { container } = render(<SlideCanvas aspectRatio="invalid" />);
			const slide = container.querySelector('.slide');

			expect(slide?.getAttribute('style')).toContain('aspect-ratio: 16 / 9');
		});

		it('falls back to 16:9 for a malformed aspect ratio', () => {
			const { container } = render(<SlideCanvas aspectRatio="abc:def" />);
			const slide = container.querySelector('.slide');

			expect(slide?.getAttribute('style')).toContain('aspect-ratio: 16 / 9');
		});

		it('falls back to 16:9 for a single-number aspect ratio', () => {
			const { container } = render(<SlideCanvas aspectRatio="16" />);
			const slide = container.querySelector('.slide');

			expect(slide?.getAttribute('style')).toContain('aspect-ratio: 16 / 9');
		});
	});

	describe('scaling', () => {
		it('applies a scale transform to the slide', () => {
			const { container } = render(<SlideCanvas />);
			const slide = container.querySelector('.slide');

			expect(slide?.getAttribute('style')).toContain('transform: scale(');
		});

		it('scales down for a smaller container', () => {
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

			const { container } = render(<SlideCanvas />);
			const slide = container.querySelector('.slide');

			expect(slide?.getAttribute('style')).toContain('transform: scale(0.5)');
		});
	});

	describe('fullscreen mode', () => {
		it('does not apply the fullscreen class by default', () => {
			const { container } = render(<SlideCanvas />);

			expect(container.querySelector('.slide-container.fullscreen')).not.toBeInTheDocument();
		});

		it('applies the fullscreen class when fullscreen is true', () => {
			const { container } = render(<SlideCanvas fullscreen />);

			expect(container.querySelector('.slide-container.fullscreen')).toBeInTheDocument();
		});
	});

	describe('theme color overrides', () => {
		it('applies valid theme color overrides as CSS custom properties on the frame', () => {
			// Overrides go on the same element as data-theme (the frame): an
			// override on the letterbox container would only be inherited by
			// the frame, and the frame's own theme rule for that property
			// (explicitly set beats inherited, regardless of specificity)
			// would win over it otherwise.
			const { container } = render(<SlideCanvas themeColors={{ accent: '#ff0000' }} />);
			const frame = container.querySelector('.slide');

			expect(frame).toHaveStyle({ '--color-accent': '#ff0000' });
			// The standard design-spec tokens (read by useTheme()) follow the
			// same override, on the same element, not just the --color-*
			// bridge CSS reads.
			expect(frame).toHaveStyle({ '--accent': '#ff0000', '--accent-text': '#ff0000' });
		});

		it('skips an invalid color value with a warning', () => {
			const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
			const { container } = render(<SlideCanvas themeColors={{ accent: 'not-a-color' }} />);
			const frame = container.querySelector('.slide');

			expect(frame?.getAttribute('style') ?? '').not.toContain('--color-accent');
			expect(warnSpy).toHaveBeenCalled();
			warnSpy.mockRestore();
		});
	});

	describe('resize handling', () => {
		it('adds a window resize listener on mount', () => {
			const addEventListenerSpy = vi.spyOn(window, 'addEventListener');

			render(<SlideCanvas />);

			expect(addEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));
		});

		it('removes the window resize listener on unmount', () => {
			const removeEventListenerSpy = vi.spyOn(window, 'removeEventListener');

			const { unmount } = render(<SlideCanvas />);
			unmount();

			expect(removeEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));
		});

		it('does not throw when unmounted', () => {
			const { unmount } = render(<SlideCanvas />);

			expect(() => unmount()).not.toThrow();
		});
	});
});
