/**
 * Touch navigation for Tap presentations, for a phone or tablet where there
 * is no keyboard to press.
 *
 * A horizontal swipe moves between slides: swipe left (content dragged
 * towards the left edge) advances, the same direction a book page turns,
 * and swipe right goes back.
 *
 * A two-finger tap toggles the overview, the touch equivalent of `o`. A
 * pinch would read better, but the page allows browser zoom and these
 * listeners are passive, so a pinch belongs to the browser.
 */

import { nextSlide, prevSlide } from '$lib/stores/presentation';

// ============================================================================
// Types
// ============================================================================

/**
 * Options for touch navigation setup.
 */
export interface TouchOptions {
	/**
	 * Callback when overview mode should be toggled.
	 */
	onToggleOverview?: () => void;

	/**
	 * Callback to check if overview is currently open.
	 * The overview scrolls and picks its own slide, so swipes are ignored
	 * while it is up.
	 */
	isOverviewOpen?: () => boolean;

	/**
	 * Callback to check if the shortcut overlay is currently open.
	 */
	isHelpOpen?: () => boolean;

	/**
	 * Callback after slide navigation occurs.
	 * Use this to broadcast slide changes to other views.
	 */
	onNavigate?: () => void;
}

// ============================================================================
// Tuning
// ============================================================================

/**
 * How far a finger must travel horizontally, in CSS pixels, before it counts
 * as a swipe. Short enough to feel responsive on a phone, long enough that a
 * tap with a slightly moving finger does nothing.
 */
const MIN_DISTANCE_PX = 50;

/**
 * How much more horizontal than vertical the movement must be. A scroll down
 * a long slide drifts sideways a little; this keeps that from turning pages.
 */
const DIRECTION_RATIO = 1.5;

/**
 * The longest a swipe may take, in milliseconds. A slow drag is usually
 * someone scrolling or selecting text, not turning a page.
 */
const MAX_DURATION_MS = 800;

/**
 * The longest a two-finger tap may take, in milliseconds. Longer than this
 * and the fingers were resting on the screen, not tapping it.
 */
const TWO_FINGER_TAP_MS = 400;

/**
 * How far the midpoint between two fingers may drift, in CSS pixels, and
 * still count as a tap rather than a pinch or a two-finger pan.
 */
const TWO_FINGER_DRIFT_PX = 30;

/**
 * How long to swallow the synthetic click a browser fires after a touch
 * sequence, in milliseconds. Without this the click lands on whatever the
 * gesture just opened -- the overview's backdrop -- and closes it again, so
 * the grid appears for one frame and vanishes.
 */
const GHOST_CLICK_MS = 500;

// ============================================================================
// Internal State
// ============================================================================

// Assumes a single active listener; a second concurrent
// setupTouchNavigation() call overwrites this rather than stacking.
let currentOptions: TouchOptions = {};

interface TouchOrigin {
	x: number;
	y: number;
	startedAt: number;
}

let origin: TouchOrigin | null = null;

interface TwoFingerOrigin {
	x: number;
	y: number;
	startedAt: number;
}

// Lifting two fingers fires touchend twice, so the gesture is consumed on
// the first of them and this goes back to null.
let twoFinger: TwoFingerOrigin | null = null;

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Check if the touch started on an element that wants the gesture for
 * itself: a form control, an editable region, or anything that scrolls
 * sideways (a wide code block, a table).
 */
function isInteractiveTarget(target: EventTarget | null): boolean {
	if (!(target instanceof Element)) {
		return false;
	}

	if (target.closest('input, textarea, select, button, a, [contenteditable="true"]')) {
		return true;
	}

	// An ancestor that scrolls horizontally owns the swipe.
	let element: Element | null = target;
	while (element && element !== document.body) {
		if (element.scrollWidth > element.clientWidth + 1) {
			const overflowX = window.getComputedStyle(element).overflowX;
			if (overflowX === 'auto' || overflowX === 'scroll') {
				return true;
			}
		}
		element = element.parentElement;
	}

	return false;
}

/**
 * Swallow the next click, whoever it lands on, for a moment.
 * Capture phase, so it never reaches the element's own handler.
 */
function swallowNextClick(): void {
	if (typeof window === 'undefined') {
		return;
	}

	let timer: ReturnType<typeof setTimeout>;

	const swallow = (event: MouseEvent): void => {
		event.preventDefault();
		event.stopPropagation();
		release();
	};

	const release = (): void => {
		clearTimeout(timer);
		window.removeEventListener('click', swallow, true);
	};

	window.addEventListener('click', swallow, true);
	timer = setTimeout(release, GHOST_CLICK_MS);
}

/**
 * Whether an overlay is up that should swallow the gesture.
 */
function isOverlayOpen(): boolean {
	return (currentOptions.isOverviewOpen?.() ?? false) || (currentOptions.isHelpOpen?.() ?? false);
}

// ============================================================================
// Handlers
// ============================================================================

/** The midpoint between the first two touches. */
function centroid(touches: TouchList): { x: number; y: number } | null {
	const first = touches[0];
	const second = touches[1];
	if (!first || !second) {
		return null;
	}
	return { x: (first.clientX + second.clientX) / 2, y: (first.clientY + second.clientY) / 2 };
}

function handleTouchStart(event: TouchEvent): void {
	// Two fingers is its own gesture, and never a page turn.
	if (event.touches.length === 2) {
		origin = null;
		const middle = centroid(event.touches);
		twoFinger = middle ? { ...middle, startedAt: Date.now() } : null;
		return;
	}

	if (event.touches.length !== 1) {
		origin = null;
		twoFinger = null;
		return;
	}

	if (isOverlayOpen() || isInteractiveTarget(event.target)) {
		origin = null;
		return;
	}

	const touch = event.touches[0];
	if (!touch) {
		origin = null;
		return;
	}

	origin = { x: touch.clientX, y: touch.clientY, startedAt: Date.now() };
}

function handleTouchMove(event: TouchEvent): void {
	// A second finger landing mid-gesture cancels the swipe.
	if (event.touches.length !== 1) {
		origin = null;
	}

	// A pinch or a two-finger pan is not a tap.
	if (twoFinger && event.touches.length === 2) {
		const middle = centroid(event.touches);
		if (
			!middle ||
			Math.abs(middle.x - twoFinger.x) > TWO_FINGER_DRIFT_PX ||
			Math.abs(middle.y - twoFinger.y) > TWO_FINGER_DRIFT_PX
		) {
			twoFinger = null;
		}
	}
}

function handleTouchEnd(event: TouchEvent): void {
	const start = origin;
	origin = null;

	// A two-finger tap toggles the overview, open or closed. The help
	// overlay still swallows it, the way it swallows every key but ? and Esc.
	const twoFingerStart = twoFinger;
	twoFinger = null;
	if (twoFingerStart) {
		if (
			Date.now() - twoFingerStart.startedAt <= TWO_FINGER_TAP_MS &&
			!(currentOptions.isHelpOpen?.() ?? false)
		) {
			swallowNextClick();
			currentOptions.onToggleOverview?.();
		}
		return;
	}

	if (!start || isOverlayOpen()) {
		return;
	}

	const touch = event.changedTouches[0];
	if (!touch) {
		return;
	}

	if (Date.now() - start.startedAt > MAX_DURATION_MS) {
		return;
	}

	const deltaX = touch.clientX - start.x;
	const deltaY = touch.clientY - start.y;

	if (Math.abs(deltaX) < MIN_DISTANCE_PX) {
		return;
	}

	if (Math.abs(deltaX) < Math.abs(deltaY) * DIRECTION_RATIO) {
		return;
	}

	if (deltaX < 0) {
		nextSlide();
	} else {
		prevSlide();
	}

	currentOptions.onNavigate?.();
}

function handleTouchCancel(): void {
	origin = null;
	twoFinger = null;
}

// ============================================================================
// Public API
// ============================================================================

/**
 * Set up swipe navigation for the presentation.
 * Returns a cleanup function to remove the event listeners.
 *
 * Listeners are passive: the gesture is measured, never prevented, so
 * vertical scrolling and pinch-zoom keep working as they should.
 *
 * @param options - Configuration options for touch behavior
 * @returns Cleanup function to remove the event listeners
 *
 * @example
 * ```typescript
 * import { setupTouchNavigation } from '$lib/utils/touch';
 *
 * useEffect(() => {
 *   const cleanup = setupTouchNavigation({
 *     onNavigate: broadcastPresentationState,
 *     isOverviewOpen: () => overviewOpenRef.current
 *   });
 *   return cleanup;
 * }, []);
 * ```
 */
export function setupTouchNavigation(options: TouchOptions = {}): () => void {
	if (typeof window === 'undefined') {
		return () => {};
	}

	currentOptions = options;
	origin = null;
	twoFinger = null;

	const passive = { passive: true } as const;
	window.addEventListener('touchstart', handleTouchStart, passive);
	window.addEventListener('touchmove', handleTouchMove, passive);
	window.addEventListener('touchend', handleTouchEnd, passive);
	window.addEventListener('touchcancel', handleTouchCancel, passive);

	return () => {
		window.removeEventListener('touchstart', handleTouchStart);
		window.removeEventListener('touchmove', handleTouchMove);
		window.removeEventListener('touchend', handleTouchEnd);
		window.removeEventListener('touchcancel', handleTouchCancel);
		currentOptions = {};
		origin = null;
		twoFinger = null;
	};
}
