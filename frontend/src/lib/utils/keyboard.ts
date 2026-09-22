/**
 * Keyboard navigation for Tap presentations.
 * Handles all keyboard shortcuts for slide navigation and presentation controls.
 */

import { nextSlide, prevSlide, goToFirstSlide, goToLastSlide, cycleTheme } from '$lib/stores/presentation';
import { HELP_KEY } from './shortcuts';
import { useConnectionStore } from '$lib/stores/websocket';

// ============================================================================
// Types
// ============================================================================

/**
 * Options for keyboard navigation setup.
 */
export interface KeyboardOptions {
	/**
	 * Callback when overview mode should be toggled.
	 */
	onToggleOverview?: () => void;

	/**
	 * Callback when presenter view should be opened.
	 */
	onOpenPresenter?: () => void;

	/**
	 * Callback to check if overview is currently open.
	 */
	isOverviewOpen?: () => boolean;

	/**
	 * Callback when the shortcut overlay should be toggled.
	 */
	onToggleHelp?: () => void;

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
// Internal State
// ============================================================================

// Assumes a single active listener; a second concurrent setupKeyboardHandler()
// call overwrites this rather than stacking.
let currentOptions: KeyboardOptions = {};

// ============================================================================
// Navigation Keys
// ============================================================================

/**
 * Keys that advance to next slide/fragment.
 */
const ADVANCE_KEYS = ['ArrowRight', 'ArrowDown', ' ', 'Enter', 'PageDown'];

/**
 * Keys that go back to previous slide/fragment.
 */
const RETREAT_KEYS = ['ArrowLeft', 'ArrowUp', 'Backspace', 'PageUp'];

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Check if the active element is an input-like element.
 * Skip keyboard navigation when user is typing.
 */
function isInputFocused(): boolean {
	if (typeof document === 'undefined') {
		return false;
	}

	const activeElement = document.activeElement;
	if (!activeElement) {
		return false;
	}

	const tagName = activeElement.tagName.toLowerCase();
	if (tagName === 'input' || tagName === 'textarea' || tagName === 'select') {
		return true;
	}

	// Check for contenteditable
	if (activeElement.getAttribute('contenteditable') === 'true') {
		return true;
	}

	return false;
}

/**
 * Toggle fullscreen mode for the document.
 */
async function toggleFullscreen(): Promise<void> {
	if (typeof document === 'undefined') {
		return;
	}

	try {
		if (!document.fullscreenElement) {
			await document.documentElement.requestFullscreen();
		} else {
			await document.exitFullscreen();
		}
	} catch {
		// Fullscreen may not be supported or user denied
		console.warn('Fullscreen toggle failed');
	}
}

/**
 * Open the presenter view in a new window.
 */
function openPresenterView(): void {
	if (typeof window === 'undefined') {
		return;
	}

	// Check if there's a custom handler
	if (currentOptions.onOpenPresenter) {
		currentOptions.onOpenPresenter();
		return;
	}

	// Default: open /presenter in a new window
	const presenterURL = new URL('/presenter', window.location.href);
	// Copy the current hash to the presenter view
	presenterURL.hash = window.location.hash;
	window.open(presenterURL.toString(), 'tap-presenter', 'width=1024,height=768');
}

/**
 * Check if we're currently in fullscreen mode.
 */
function isFullscreen(): boolean {
	if (typeof document === 'undefined') {
		return false;
	}
	return !!document.fullscreenElement;
}

// ============================================================================
// Main Handler
// ============================================================================

/**
 * Handle keyboard events for presentation navigation.
 */
function handleKeyDown(event: KeyboardEvent): void {
	// Escape blurs a focused editable element instead of falling through to
	// isInputFocused()'s early return below, which would otherwise make
	// Escape do nothing at all - not even overview/fullscreen - while an
	// input or textarea has focus. This is the one key a focused editable
	// element does not swallow: it takes the keyboard back from the
	// element on this same press, and nothing else on that same press (no
	// overview toggle, no fullscreen exit).
	if (event.key === 'Escape' && isInputFocused()) {
		event.preventDefault();
		(document.activeElement as HTMLElement | null)?.blur();
		return;
	}

	// Skip if input is focused
	if (isInputFocused()) {
		return;
	}

	const key = event.key;

	// While the shortcut overlay is open, only ? and Escape reach it (to
	// close it). Every other key is ignored, the same way the overview
	// swallows keys.
	const isHelp = currentOptions.isHelpOpen?.() ?? false;
	if (isHelp) {
		if (key === HELP_KEY || key === 'Escape') {
			event.preventDefault();
			currentOptions.onToggleHelp?.();
		}
		return;
	}

	// Handle overview mode specially
	const isOverview = currentOptions.isOverviewOpen?.() ?? false;

	// Escape key - close overview or exit fullscreen
	if (key === 'Escape') {
		event.preventDefault();
		if (isOverview && currentOptions.onToggleOverview) {
			currentOptions.onToggleOverview();
			return;
		}
		if (isFullscreen()) {
			void toggleFullscreen();
			return;
		}
		return;
	}

	// In overview mode, certain keys should be ignored (let overview handle them)
	if (isOverview) {
		// Overview handles its own arrow key navigation for grid selection
		// Only pass through Enter to select and close
		return;
	}

	// ? - toggle the shortcut overlay
	if (key === HELP_KEY && currentOptions.onToggleHelp) {
		event.preventDefault();
		currentOptions.onToggleHelp();
		return;
	}

	// Advance keys
	if (ADVANCE_KEYS.includes(key)) {
		event.preventDefault();
		nextSlide();
		currentOptions.onNavigate?.();
		return;
	}

	// Retreat keys
	if (RETREAT_KEYS.includes(key)) {
		event.preventDefault();
		prevSlide();
		currentOptions.onNavigate?.();
		return;
	}

	// Home - go to the first slide a talk shows
	if (key === 'Home') {
		event.preventDefault();
		goToFirstSlide();
		currentOptions.onNavigate?.();
		return;
	}

	// End - go to the last slide a talk shows
	if (key === 'End') {
		event.preventDefault();
		goToLastSlide();
		currentOptions.onNavigate?.();
		return;
	}

	// S - open presenter view
	if (key === 's' || key === 'S') {
		event.preventDefault();
		openPresenterView();
		return;
	}

	// O - toggle overview
	if (key === 'o' || key === 'O') {
		event.preventDefault();
		if (currentOptions.onToggleOverview) {
			currentOptions.onToggleOverview();
		}
		return;
	}

	// T - cycle through themes, except during tap present, where a stray T
	// must not change what the audience sees.
	if (key === 't' || key === 'T') {
		if (useConnectionStore.getState().presentMode) {
			return;
		}
		event.preventDefault();
		cycleTheme();
		return;
	}

	// F - toggle fullscreen
	if (key === 'f' || key === 'F') {
		event.preventDefault();
		void toggleFullscreen();
		return;
	}
}

// ============================================================================
// Public API
// ============================================================================

/**
 * Set up keyboard navigation for the presentation.
 * Returns a cleanup function to remove the event listener.
 *
 * @param options - Configuration options for keyboard behavior
 * @returns Cleanup function to remove the event listener
 *
 * @example
 * ```typescript
 * import { setupKeyboardNavigation } from '$lib/utils/keyboard';
 *
 * useEffect(() => {
 *   const cleanup = setupKeyboardNavigation({
 *     onToggleOverview: () => setOverviewOpen((open) => !open),
 *     isOverviewOpen: () => overviewOpen,
 *   });
 *   return cleanup;
 * }, [overviewOpen]);
 * ```
 */
export function setupKeyboardNavigation(options: KeyboardOptions = {}): () => void {
	if (typeof window === 'undefined') {
		return () => {};
	}

	currentOptions = options;

	window.addEventListener('keydown', handleKeyDown);

	return () => {
		window.removeEventListener('keydown', handleKeyDown);
		currentOptions = {};
	};
}

/**
 * Manually trigger the next slide/fragment action.
 * Useful for touch controls or custom UI.
 */
export function triggerNext(): boolean {
	return nextSlide();
}

/**
 * Manually trigger the previous slide/fragment action.
 * Useful for touch controls or custom UI.
 */
export function triggerPrev(): boolean {
	return prevSlide();
}

/**
 * Manually trigger fullscreen toggle.
 */
export function triggerFullscreen(): Promise<void> {
	return toggleFullscreen();
}

/**
 * Check if currently in fullscreen mode.
 */
export function checkFullscreen(): boolean {
	return isFullscreen();
}
