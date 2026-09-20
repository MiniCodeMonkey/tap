/**
 * Unit tests for touch (swipe) navigation.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { setupTouchNavigation } from './touch';
import * as presentationStore from '$lib/stores/presentation';

// ============================================================================
// Mock Setup
// ============================================================================

vi.mock('$lib/stores/presentation', () => ({
	nextSlide: vi.fn(() => true),
	prevSlide: vi.fn(() => true)
}));

/**
 * jsdom has no Touch or TouchEvent constructor, so dispatch a plain Event
 * carrying the two touch lists the handlers read.
 */
function dispatchTouch(
	type: 'touchstart' | 'touchmove' | 'touchend' | 'touchcancel',
	points: { x: number; y: number }[],
	target: EventTarget = document.body
): void {
	const touches = points.map((point) => ({ clientX: point.x, clientY: point.y }));
	const event = new Event(type, { bubbles: true });
	Object.defineProperty(event, 'touches', {
		value: type === 'touchend' ? [] : touches
	});
	Object.defineProperty(event, 'changedTouches', { value: touches });
	Object.defineProperty(event, 'target', { value: target });
	window.dispatchEvent(event);
}

/** Start at (x, y), lift at (x + dx, y + dy). */
function swipe(dx: number, dy: number, target: EventTarget = document.body): void {
	dispatchTouch('touchstart', [{ x: 200, y: 200 }], target);
	dispatchTouch('touchend', [{ x: 200 + dx, y: 200 + dy }], target);
}

describe('touch navigation', () => {
	let cleanup: () => void;

	beforeEach(() => {
		vi.clearAllMocks();
		cleanup = setupTouchNavigation();
	});

	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	describe('direction', () => {
		it('advances on a swipe to the left', () => {
			swipe(-120, 0);
			expect(presentationStore.nextSlide).toHaveBeenCalledTimes(1);
			expect(presentationStore.prevSlide).not.toHaveBeenCalled();
		});

		it('goes back on a swipe to the right', () => {
			swipe(120, 0);
			expect(presentationStore.prevSlide).toHaveBeenCalledTimes(1);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});
	});

	describe('gestures that must not navigate', () => {
		it('ignores a tap', () => {
			swipe(0, 0);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
			expect(presentationStore.prevSlide).not.toHaveBeenCalled();
		});

		it('ignores a short drag below the distance threshold', () => {
			swipe(-30, 0);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});

		it('ignores a vertical scroll', () => {
			swipe(-20, -200);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});

		it('ignores a diagonal drag that is mostly vertical', () => {
			swipe(-60, -120);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});

		it('ignores a two-finger gesture', () => {
			dispatchTouch('touchstart', [
				{ x: 200, y: 200 },
				{ x: 260, y: 200 }
			]);
			dispatchTouch('touchend', [{ x: 60, y: 200 }]);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});

		it('cancels when a second finger lands mid-swipe', () => {
			dispatchTouch('touchstart', [{ x: 200, y: 200 }]);
			dispatchTouch('touchmove', [
				{ x: 150, y: 200 },
				{ x: 300, y: 200 }
			]);
			dispatchTouch('touchend', [{ x: 60, y: 200 }]);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});

		it('ignores a swipe that took too long', () => {
			vi.useFakeTimers();
			dispatchTouch('touchstart', [{ x: 200, y: 200 }]);
			vi.advanceTimersByTime(2000);
			dispatchTouch('touchend', [{ x: 60, y: 200 }]);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});

		it('ignores a swipe on a link or button', () => {
			const button = document.createElement('button');
			document.body.appendChild(button);
			swipe(-120, 0, button);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
			button.remove();
		});

		it('ignores a touchend with no origin', () => {
			dispatchTouch('touchend', [{ x: 60, y: 200 }]);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});

		it('ignores a swipe after touchcancel', () => {
			dispatchTouch('touchstart', [{ x: 200, y: 200 }]);
			dispatchTouch('touchcancel', [{ x: 200, y: 200 }]);
			dispatchTouch('touchend', [{ x: 60, y: 200 }]);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});
	});

	describe('overlays', () => {
		it('ignores swipes while the overview is open', () => {
			cleanup();
			cleanup = setupTouchNavigation({ isOverviewOpen: () => true });
			swipe(-120, 0);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});

		it('ignores swipes while the shortcut overlay is open', () => {
			cleanup();
			cleanup = setupTouchNavigation({ isHelpOpen: () => true });
			swipe(-120, 0);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});
	});

	describe('two-finger tap', () => {
		it('toggles the overview', () => {
			const onToggleOverview = vi.fn();
			cleanup();
			cleanup = setupTouchNavigation({ onToggleOverview });
			dispatchTouch('touchstart', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			dispatchTouch('touchend', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			expect(onToggleOverview).toHaveBeenCalledTimes(1);
		});

		it('toggles the overview closed again while it is open', () => {
			const onToggleOverview = vi.fn();
			cleanup();
			cleanup = setupTouchNavigation({ onToggleOverview, isOverviewOpen: () => true });
			dispatchTouch('touchstart', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			dispatchTouch('touchend', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			expect(onToggleOverview).toHaveBeenCalledTimes(1);
		});

		it('does not toggle on a pinch', () => {
			const onToggleOverview = vi.fn();
			cleanup();
			cleanup = setupTouchNavigation({ onToggleOverview });
			dispatchTouch('touchstart', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			dispatchTouch('touchmove', [
				{ x: 60, y: 340 },
				{ x: 140, y: 340 }
			]);
			dispatchTouch('touchend', [
				{ x: 60, y: 340 },
				{ x: 140, y: 340 }
			]);
			expect(onToggleOverview).not.toHaveBeenCalled();
		});

		it('does not toggle when the fingers rested too long', () => {
			vi.useFakeTimers();
			const onToggleOverview = vi.fn();
			cleanup();
			cleanup = setupTouchNavigation({ onToggleOverview });
			dispatchTouch('touchstart', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			vi.advanceTimersByTime(1000);
			dispatchTouch('touchend', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			expect(onToggleOverview).not.toHaveBeenCalled();
		});

		it('does not toggle while the shortcut overlay is open', () => {
			const onToggleOverview = vi.fn();
			cleanup();
			cleanup = setupTouchNavigation({ onToggleOverview, isHelpOpen: () => true });
			dispatchTouch('touchstart', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			dispatchTouch('touchend', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			expect(onToggleOverview).not.toHaveBeenCalled();
		});

		it('does not also change slide', () => {
			const onToggleOverview = vi.fn();
			cleanup();
			cleanup = setupTouchNavigation({ onToggleOverview });
			dispatchTouch('touchstart', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			dispatchTouch('touchend', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
			expect(presentationStore.prevSlide).not.toHaveBeenCalled();
		});

		it('fires once even though lifting two fingers ends twice', () => {
			const onToggleOverview = vi.fn();
			cleanup();
			cleanup = setupTouchNavigation({ onToggleOverview });
			dispatchTouch('touchstart', [
				{ x: 180, y: 200 },
				{ x: 260, y: 200 }
			]);
			dispatchTouch('touchend', [{ x: 180, y: 200 }]);
			dispatchTouch('touchend', [{ x: 260, y: 200 }]);
			expect(onToggleOverview).toHaveBeenCalledTimes(1);
		});
	});

	describe('callbacks', () => {
		it('calls onNavigate after a swipe', () => {
			const onNavigate = vi.fn();
			cleanup();
			cleanup = setupTouchNavigation({ onNavigate });
			swipe(-120, 0);
			expect(onNavigate).toHaveBeenCalledTimes(1);
		});

		it('does not call onNavigate when the gesture is ignored', () => {
			const onNavigate = vi.fn();
			cleanup();
			cleanup = setupTouchNavigation({ onNavigate });
			swipe(-10, 0);
			expect(onNavigate).not.toHaveBeenCalled();
		});
	});

	describe('cleanup', () => {
		it('stops navigating once cleaned up', () => {
			cleanup();
			swipe(-120, 0);
			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
			cleanup = () => {};
		});
	});
});
