/**
 * Keyboard navigation for Tap presentations.
 * Handles all keyboard shortcuts for slide navigation and presentation controls.
 */

import { nextSlide, prevSlide, goToSlide, totalSlides } from '$lib/stores/presentation';
import { get } from 'svelte/store';

// ============================================================================
// Types
// ============================================================================

/**
 * State for overview and fullscreen modes.
 */
export interface KeyboardState {
	isOverviewOpen: boolean;
	isFullscreen: boolean;
	onToggleOverview?: () => void;
	onOpenPresenter?: () => void;
}

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
}

// ============================================================================
// Internal State
// ============================================================================

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
	// Skip if input is focused
	if (isInputFocused()) {
		return;
	}

	const key = event.key;

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

	// Advance keys
	if (ADVANCE_KEYS.includes(key)) {
		event.preventDefault();
		nextSlide();
		return;
	}

	// Retreat keys
	if (RETREAT_KEYS.includes(key)) {
		event.preventDefault();
		prevSlide();
		return;
	}

	// Home - go to first slide
	if (key === 'Home') {
		event.preventDefault();
		goToSlide(0);
		return;
	}

	// End - go to last slide
	if (key === 'End') {
		event.preventDefault();
		const total = get(totalSlides);
		if (total > 0) {
			goToSlide(total - 1);
		}
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
 * // In your Svelte component
 * onMount(() => {
 *   const cleanup = setupKeyboardNavigation({
 *     onToggleOverview: () => { overviewOpen = !overviewOpen },
 *     isOverviewOpen: () => overviewOpen,
 *   });
 *   return cleanup;
 * });
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
