import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import SlideRenderer from './SlideRenderer.svelte';
import type { Slide } from '$lib/types';

describe('SlideRenderer', () => {
	// Create a basic slide for testing
	const createSlide = (overrides: Partial<Slide> = {}): Slide => ({
		index: 0,
		layout: 'default',
		html: '<h1>Test Slide</h1><p>Test content</p>',
		...overrides
	});

	beforeEach(() => {
		// Reset mocks before each test
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('rendering', () => {
		it('renders slide HTML content', () => {
			const slide = createSlide({
				html: '<h1>Hello World</h1><p>Test paragraph</p>'
			});

			render(SlideRenderer, { props: { slide } });

			expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Hello World');
			expect(screen.getByText('Test paragraph')).toBeInTheDocument();
		});

		it('applies layout class based on slide layout', () => {
			const slide = createSlide({ layout: 'title' });

			const { container } = render(SlideRenderer, { props: { slide } });

			expect(container.querySelector('.layout-title')).toBeInTheDocument();
		});

		it('applies different layout classes for each layout type', () => {
			const layouts = ['default', 'title', 'section', 'two-column', 'code-focus', 'quote', 'big-stat'] as const;

			for (const layout of layouts) {
				cleanup();
				const slide = createSlide({ layout });
				const { container } = render(SlideRenderer, { props: { slide } });

				expect(container.querySelector(`.layout-${layout}`)).toBeInTheDocument();
			}
		});

		it('does not render when active is false', () => {
			const slide = createSlide();

			const { container } = render(SlideRenderer, {
				props: { slide, active: false }
			});

			expect(container.querySelector('.slide-renderer')).not.toBeInTheDocument();
		});

		it('renders when active is true', () => {
			const slide = createSlide();

			const { container } = render(SlideRenderer, {
				props: { slide, active: true }
			});

			expect(container.querySelector('.slide-renderer')).toBeInTheDocument();
		});
	});

	describe('background handling', () => {
		it('applies color background', () => {
			const slide = createSlide({
				background: { type: 'color', value: '#ff0000' }
			});

			const { container } = render(SlideRenderer, { props: { slide } });
			const slideEl = container.querySelector('.slide-renderer');

			expect(slideEl).toHaveStyle({ backgroundColor: '#ff0000' });
		});

		it('applies gradient background', () => {
			const slide = createSlide({
				background: { type: 'gradient', value: 'linear-gradient(45deg, red, blue)' }
			});

			const { container } = render(SlideRenderer, { props: { slide } });
			const slideEl = container.querySelector('.slide-renderer');

			expect(slideEl).toHaveAttribute('style');
			expect(slideEl?.getAttribute('style')).toContain('linear-gradient(45deg, red, blue)');
		});

		it('applies image background', () => {
			const slide = createSlide({
				background: { type: 'image', value: '/images/bg.jpg' }
			});

			const { container } = render(SlideRenderer, { props: { slide } });
			const slideEl = container.querySelector('.slide-renderer');

			expect(slideEl).toHaveAttribute('style');
			// Check for url with the image path (quotes may be single or double)
			expect(slideEl?.getAttribute('style')).toContain('/images/bg.jpg');
			expect(slideEl?.getAttribute('style')).toContain('background-size: cover');
		});

		it('handles missing background gracefully', () => {
			const slide = createSlide();
			// No background property

			const { container } = render(SlideRenderer, { props: { slide } });
			const slideEl = container.querySelector('.slide-renderer');

			// Should render without error
			expect(slideEl).toBeInTheDocument();
		});
	});

	describe('fragment handling', () => {
		it('adds has-fragments class when slide has fragments', () => {
			const slide = createSlide({
				fragments: [
					{ index: 0, content: '<p>Fragment 1</p>' },
					{ index: 1, content: '<p>Fragment 2</p>' }
				]
			});

			const { container } = render(SlideRenderer, { props: { slide } });

			expect(container.querySelector('.has-fragments')).toBeInTheDocument();
		});

		it('does not add has-fragments class when slide has no fragments', () => {
			const slide = createSlide();
			// No fragments property

			const { container } = render(SlideRenderer, { props: { slide } });

			expect(container.querySelector('.has-fragments')).not.toBeInTheDocument();
		});

		it('wraps fragments with visibility classes', () => {
			const slide = createSlide({
				fragments: [
					{ index: 0, content: '<p>Fragment 0</p>' },
					{ index: 1, content: '<p>Fragment 1</p>' },
					{ index: 2, content: '<p>Fragment 2</p>' }
				]
			});

			// visibleFragments = 1 means fragments 0 and 1 are visible
			const { container } = render(SlideRenderer, {
				props: { slide, visibleFragments: 1 }
			});

			const fragmentElements = container.querySelectorAll('.fragment');
			expect(fragmentElements).toHaveLength(3);

			// Fragment 0 should be visible (index <= 1)
			expect(fragmentElements[0]).toHaveClass('fragment-visible');
			// Fragment 1 should be visible (index <= 1)
			expect(fragmentElements[1]).toHaveClass('fragment-visible');
			// Fragment 2 should be hidden (index > 1)
			expect(fragmentElements[2]).toHaveClass('fragment-hidden');
		});

		it('hides all fragments when visibleFragments is -1', () => {
			const slide = createSlide({
				fragments: [
					{ index: 0, content: '<p>Fragment 0</p>' },
					{ index: 1, content: '<p>Fragment 1</p>' }
				]
			});

			const { container } = render(SlideRenderer, {
				props: { slide, visibleFragments: -1 }
			});

			const fragmentElements = container.querySelectorAll('.fragment');
			for (const el of fragmentElements) {
				expect(el).toHaveClass('fragment-hidden');
			}
		});

		it('shows all fragments when visibleFragments equals max index', () => {
			const slide = createSlide({
				fragments: [
					{ index: 0, content: '<p>Fragment 0</p>' },
					{ index: 1, content: '<p>Fragment 1</p>' },
					{ index: 2, content: '<p>Fragment 2</p>' }
				]
			});

			const { container } = render(SlideRenderer, {
				props: { slide, visibleFragments: 2 }
			});

			const fragmentElements = container.querySelectorAll('.fragment');
			for (const el of fragmentElements) {
				expect(el).toHaveClass('fragment-visible');
			}
		});

		it('renders fragment content correctly', () => {
			const slide = createSlide({
				fragments: [
					{ index: 0, content: '<p>First fragment content</p>' },
					{ index: 1, content: '<p>Second fragment content</p>' }
				]
			});

			render(SlideRenderer, { props: { slide, visibleFragments: 1 } });

			expect(screen.getByText('First fragment content')).toBeInTheDocument();
			expect(screen.getByText('Second fragment content')).toBeInTheDocument();
		});
	});

	describe('transitions', () => {
		it('defaults to fade transition', () => {
			const slide = createSlide();
			// slide.transition is undefined

			// We can't easily test the actual transition function,
			// but we can verify the component renders without error
			const { container } = render(SlideRenderer, { props: { slide } });
			expect(container.querySelector('.slide-renderer')).toBeInTheDocument();
		});

		it('uses slide transition from slide data', () => {
			const slide = createSlide({ transition: 'zoom' });

			const { container } = render(SlideRenderer, { props: { slide } });
			expect(container.querySelector('.slide-renderer')).toBeInTheDocument();
		});

		it('uses custom transition duration', () => {
			const slide = createSlide();

			const { container } = render(SlideRenderer, {
				props: { slide, transitionDuration: 800 }
			});

			expect(container.querySelector('.slide-renderer')).toBeInTheDocument();
		});

		it('handles direction prop for directional transitions', () => {
			const slide = createSlide({ transition: 'slide' });

			// Test forward direction
			let { container } = render(SlideRenderer, {
				props: { slide, direction: 'forward' }
			});
			expect(container.querySelector('.slide-renderer')).toBeInTheDocument();

			cleanup();

			// Test backward direction
			({ container } = render(SlideRenderer, {
				props: { slide, direction: 'backward' }
			}));
			expect(container.querySelector('.slide-renderer')).toBeInTheDocument();
		});
	});

	describe('reduced motion', () => {
		it('respects reduced motion preference', () => {
			// Mock reduced motion preference
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

			const slide = createSlide({ transition: 'slide' });

			const { container } = render(SlideRenderer, { props: { slide } });

			// Should render without error when reduced motion is preferred
			expect(container.querySelector('.slide-renderer')).toBeInTheDocument();
		});
	});
});
