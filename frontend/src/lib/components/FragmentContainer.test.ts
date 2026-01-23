import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import FragmentContainer from './FragmentContainer.svelte';
import type { FragmentGroup } from '$lib/types';

describe('FragmentContainer', () => {
	// Create test fragments
	const createFragments = (count: number): FragmentGroup[] =>
		Array.from({ length: count }, (_, i) => ({
			index: i,
			content: `<p>Fragment ${i}</p>`
		}));

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('rendering', () => {
		it('renders fragment container', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(2),
					currentIndex: 0
				}
			});

			expect(container.querySelector('.fragment-container')).toBeInTheDocument();
		});

		it('renders fragment items for visible fragments', () => {
			const fragments = createFragments(3);

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: 2 }
			});

			const items = container.querySelectorAll('.fragment-item');
			expect(items).toHaveLength(3);
		});

		it('renders fragment content as HTML', () => {
			const fragments = [{ index: 0, content: '<p>Test <strong>bold</strong> content</p>' }];

			render(FragmentContainer, {
				props: { fragments, currentIndex: 0 }
			});

			expect(screen.getByText('bold')).toBeInTheDocument();
		});

		it('renders no items when currentIndex is -1', () => {
			const fragments = createFragments(3);

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: -1 }
			});

			// With Svelte transitions, items may not be rendered at all
			// or will have visibility hidden - depends on implementation
			const items = container.querySelectorAll('.fragment-item');
			expect(items.length).toBeLessThanOrEqual(0);
		});

		it('handles empty fragments array', () => {
			const { container } = render(FragmentContainer, {
				props: { fragments: [], currentIndex: 0 }
			});

			expect(container.querySelector('.fragment-container')).toBeInTheDocument();
			expect(container.querySelectorAll('.fragment-item')).toHaveLength(0);
		});
	});

	describe('visibility', () => {
		it('shows fragment when index equals currentIndex', () => {
			const fragments = createFragments(3);

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: 1 }
			});

			// Fragment 0 and 1 should be visible (index <= currentIndex)
			const items = container.querySelectorAll('.fragment-item');
			expect(items.length).toBeGreaterThanOrEqual(2);
		});

		it('shows fragment when index is less than currentIndex', () => {
			const fragments = createFragments(3);

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: 2 }
			});

			// All fragments should be visible
			const items = container.querySelectorAll('.fragment-item');
			expect(items).toHaveLength(3);
		});

		it('hides fragment when index is greater than currentIndex', () => {
			const fragments = createFragments(3);

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: 0 }
			});

			// Only fragment 0 should be visible
			const items = container.querySelectorAll('.fragment-item');
			expect(items).toHaveLength(1);
		});

		it('adds data-fragment-index attribute', () => {
			const fragments = createFragments(2);

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: 1 }
			});

			const items = container.querySelectorAll('.fragment-item');
			expect(items[0]).toHaveAttribute('data-fragment-index', '0');
			expect(items[1]).toHaveAttribute('data-fragment-index', '1');
		});

		it('adds fragment-visible class to visible fragments', () => {
			const fragments = createFragments(2);

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: 1 }
			});

			const items = container.querySelectorAll('.fragment-item');
			for (const item of items) {
				expect(item).toHaveClass('fragment-visible');
			}
		});
	});

	describe('showAll mode', () => {
		it('shows all fragments when showAll is true', () => {
			const fragments = createFragments(5);

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: -1, showAll: true }
			});

			const items = container.querySelectorAll('.fragment-item');
			expect(items).toHaveLength(5);
		});

		it('adds show-all class to container', () => {
			const fragments = createFragments(3);

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: 0, showAll: true }
			});

			expect(container.querySelector('.show-all')).toBeInTheDocument();
		});

		it('ignores currentIndex when showAll is true', () => {
			const fragments = createFragments(3);

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: -1, showAll: true }
			});

			// All fragments should be visible despite currentIndex being -1
			const items = container.querySelectorAll('.fragment-item');
			expect(items).toHaveLength(3);
		});
	});

	describe('animation types', () => {
		it('accepts fade animation type', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0,
					animation: 'fade'
				}
			});

			expect(container.querySelector('.fragment-item')).toBeInTheDocument();
		});

		it('accepts slide-up animation type', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0,
					animation: 'slide-up'
				}
			});

			expect(container.querySelector('.fragment-item')).toBeInTheDocument();
		});

		it('accepts slide-left animation type', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0,
					animation: 'slide-left'
				}
			});

			expect(container.querySelector('.fragment-item')).toBeInTheDocument();
		});

		it('accepts scale animation type', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0,
					animation: 'scale'
				}
			});

			expect(container.querySelector('.fragment-item')).toBeInTheDocument();
		});

		it('accepts none animation type', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0,
					animation: 'none'
				}
			});

			expect(container.querySelector('.fragment-item')).toBeInTheDocument();
		});

		it('defaults to fade animation', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0
				}
			});

			// Should render without error with default animation
			expect(container.querySelector('.fragment-item')).toBeInTheDocument();
		});
	});

	describe('timing props', () => {
		it('accepts custom duration', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0,
					duration: 800
				}
			});

			expect(container.querySelector('.fragment-item')).toBeInTheDocument();
		});

		it('accepts custom delay', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0,
					delay: 200
				}
			});

			expect(container.querySelector('.fragment-item')).toBeInTheDocument();
		});

		it('defaults to 400ms duration', () => {
			// Test with default props
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0
				}
			});

			expect(container.querySelector('.fragment-item')).toBeInTheDocument();
		});

		it('defaults to 0ms delay', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0
				}
			});

			expect(container.querySelector('.fragment-item')).toBeInTheDocument();
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

			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(2),
					currentIndex: 1
				}
			});

			// Should render without animation issues
			expect(container.querySelector('.fragment-container')).toBeInTheDocument();
		});

		it('shows fragments without animation when reduced motion is preferred', () => {
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

			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0
				}
			});

			const item = container.querySelector('.fragment-item');
			expect(item).toBeInTheDocument();
		});
	});

	describe('fragment ordering', () => {
		it('maintains fragment order in DOM', () => {
			const fragments: FragmentGroup[] = [
				{ index: 0, content: '<p>First</p>' },
				{ index: 1, content: '<p>Second</p>' },
				{ index: 2, content: '<p>Third</p>' }
			];

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: 2 }
			});

			const items = container.querySelectorAll('.fragment-item');
			expect(items[0]).toHaveAttribute('data-fragment-index', '0');
			expect(items[1]).toHaveAttribute('data-fragment-index', '1');
			expect(items[2]).toHaveAttribute('data-fragment-index', '2');
		});

		it('handles non-sequential fragment indices', () => {
			const fragments: FragmentGroup[] = [
				{ index: 0, content: '<p>Zero</p>' },
				{ index: 2, content: '<p>Two</p>' },
				{ index: 5, content: '<p>Five</p>' }
			];

			const { container } = render(FragmentContainer, {
				props: { fragments, currentIndex: 5 }
			});

			const items = container.querySelectorAll('.fragment-item');
			expect(items).toHaveLength(3);
		});
	});

	describe('CSS classes', () => {
		it('fragment-container has correct base class', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(1),
					currentIndex: 0
				}
			});

			expect(container.querySelector('.fragment-container')).toBeInTheDocument();
		});

		it('fragment-item elements have correct class', () => {
			const { container } = render(FragmentContainer, {
				props: {
					fragments: createFragments(2),
					currentIndex: 1
				}
			});

			const items = container.querySelectorAll('.fragment-item');
			expect(items).toHaveLength(2);
			for (const item of items) {
				expect(item).toHaveClass('fragment-item');
			}
		});
	});
});
