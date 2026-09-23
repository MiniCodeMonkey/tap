import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { SlideTransition } from './SlideTransition';
import { heldBlockers, resetBlockersForTests } from '$lib/ready/blockers';

function mockMatchMedia(matches: boolean): void {
	vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => ({
		matches,
		media: query,
		onchange: null,
		addListener: vi.fn(),
		removeListener: vi.fn(),
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		dispatchEvent: vi.fn()
	}));
}

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
	window.history.replaceState(null, '', '/');
});

describe('SlideTransition', () => {
	it('renders its children', () => {
		render(
			<SlideTransition slideKey={0} transition="fade" direction="forward">
				<div>slide one</div>
			</SlideTransition>
		);

		expect(screen.getByText('slide one')).toBeInTheDocument();
	});

	it('renders the transition frame', () => {
		const { container } = render(
			<SlideTransition slideKey={0} transition="fade" direction="forward">
				<div>content</div>
			</SlideTransition>
		);

		expect(container.querySelector('.slide-transition-frame')).toBeInTheDocument();
	});

	it('renders children with no animation wrapper in print mode', () => {
		const { container } = render(
			<SlideTransition slideKey={0} transition="slide" direction="forward" printMode>
				<div>printed</div>
			</SlideTransition>
		);

		expect(screen.getByText('printed')).toBeInTheDocument();
		expect(container.querySelector('.slide-transition-frame')).toBeInTheDocument();
	});

	it('renders without animation when reduced motion is preferred', () => {
		mockMatchMedia(true);

		render(
			<SlideTransition slideKey={0} transition="zoom" direction="forward">
				<div>reduced</div>
			</SlideTransition>
		);

		expect(screen.getByText('reduced')).toBeInTheDocument();
	});

	it('renders children when transition is none', () => {
		render(
			<SlideTransition slideKey={0} transition="none" direction="forward">
				<div>instant</div>
			</SlideTransition>
		);

		expect(screen.getByText('instant')).toBeInTheDocument();
	});

	it('renders each of the animated transition types', () => {
		for (const transition of ['fade', 'slide', 'push', 'zoom'] as const) {
			const { unmount } = render(
				<SlideTransition slideKey={0} transition={transition} direction="forward">
					<div>{transition}</div>
				</SlideTransition>
			);

			expect(screen.getByText(transition)).toBeInTheDocument();
			unmount();
		}
	});

	it('swaps content when slideKey changes', () => {
		// Reduced motion, so the swap is synchronous: this test is about the
		// content swapping, not about the animated transition itself, which
		// AnimatePresence would otherwise hold "first" through until a real
		// exit animation finishes.
		mockMatchMedia(true);

		const { rerender } = render(
			<SlideTransition slideKey={0} transition="fade" direction="forward">
				<div>first</div>
			</SlideTransition>
		);
		expect(screen.getByText('first')).toBeInTheDocument();

		rerender(
			<SlideTransition slideKey={1} transition="fade" direction="forward">
				<div>second</div>
			</SlideTransition>
		);

		expect(screen.getByText('second')).toBeInTheDocument();
	});
});

describe('SlideTransition and the ready signal', () => {
	beforeEach(() => {
		resetBlockersForTests();
	});

	it('holds an animations blocker while it moves to the next slide', async () => {
		const { rerender } = render(
			<SlideTransition slideKey={0} transition="fade" direction="forward">
				<div>first</div>
			</SlideTransition>
		);
		expect(heldBlockers()).toEqual([]);

		rerender(
			<SlideTransition slideKey={1} transition="fade" direction="forward">
				<div>second</div>
			</SlideTransition>
		);
		expect(heldBlockers()).toEqual(['animations']);

		await waitFor(() => expect(heldBlockers()).toEqual([]), { timeout: 3000 });
	});

	it('renders the incoming slide once a real transition completes', async () => {
		const { rerender } = render(
			<SlideTransition slideKey={0} transition="fade" direction="forward">
				<div>first</div>
			</SlideTransition>
		);
		expect(screen.getByText('first')).toBeInTheDocument();

		rerender(
			<SlideTransition slideKey={1} transition="fade" direction="forward">
				<div>second</div>
			</SlideTransition>
		);

		await waitFor(() => expect(screen.getByText('second')).toBeInTheDocument(), { timeout: 3000 });
	});

	it('holds nothing in print mode', () => {
		const { rerender } = render(
			<SlideTransition slideKey={0} transition="fade" direction="forward" printMode>
				<div>first</div>
			</SlideTransition>
		);
		rerender(
			<SlideTransition slideKey={1} transition="fade" direction="forward" printMode>
				<div>second</div>
			</SlideTransition>
		);
		expect(heldBlockers()).toEqual([]);
	});

	it('holds nothing when the transition is none', () => {
		const { rerender } = render(
			<SlideTransition slideKey={0} transition="none" direction="forward">
				<div>first</div>
			</SlideTransition>
		);
		rerender(
			<SlideTransition slideKey={1} transition="none" direction="forward">
				<div>second</div>
			</SlideTransition>
		);
		expect(heldBlockers()).toEqual([]);
	});
});
