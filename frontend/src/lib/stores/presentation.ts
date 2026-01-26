/**
 * Presentation state store for slide navigation and fragment management.
 * Uses Svelte 5 runes for reactive state management.
 */

import { writable, derived, type Readable } from 'svelte/store';
import type { Presentation, Slide, Theme } from '$lib/types';

// ============================================================================
// Writable Stores
// ============================================================================

/**
 * The full presentation data from the backend.
 */
export const presentation = writable<Presentation | null>(null);

/**
 * Current slide index (0-based).
 */
export const currentSlideIndex = writable<number>(0);

/**
 * Current fragment index within the current slide.
 * -1 means no fragments are visible (before first fragment).
 */
export const currentFragmentIndex = writable<number>(-1);

/**
 * Whether the scroll animation has been triggered for the current slide.
 * Used for slides with scroll: true directive.
 * false = at top of slide, true = scrolled to bottom.
 */
export const scrollRevealed = writable<boolean>(false);

/**
 * Counter that increments each time scroll is triggered by user action.
 * This allows the component to distinguish between store updates from
 * user navigation vs initialization/resets.
 */
export const scrollTriggerCount = writable<number>(0);

/**
 * Theme override from WebSocket.
 * When set, this overrides the theme from presentation config.
 * This is temporary and doesn't modify the markdown file.
 */
export const themeOverride = writable<Theme | null>(null);

// ============================================================================
// Derived Stores
// ============================================================================

/**
 * The current slide object, or null if no presentation is loaded.
 */
export const currentSlide: Readable<Slide | null> = derived(
	[presentation, currentSlideIndex],
	([$presentation, $currentSlideIndex]) => {
		if (!$presentation || !$presentation.slides.length) {
			return null;
		}
		return $presentation.slides[$currentSlideIndex] ?? null;
	}
);

/**
 * Total number of slides in the presentation.
 */
export const totalSlides: Readable<number> = derived(presentation, ($presentation) => {
	return $presentation?.slides.length ?? 0;
});

/**
 * Total number of fragments in the current slide.
 * Only counts fragments if there are 2+ (meaning actual pause markers exist).
 */
export const totalFragments: Readable<number> = derived(currentSlide, ($currentSlide) => {
	const count = $currentSlide?.fragments?.length ?? 0;
	return count > 1 ? count : 0;
});

/**
 * Whether the current slide has scroll reveal enabled.
 */
export const hasScrollReveal: Readable<boolean> = derived(currentSlide, ($currentSlide) => {
	return $currentSlide?.scroll === true;
});

// ============================================================================
// Navigation Actions
// ============================================================================

/**
 * Navigate to the next slide.
 * Order of operations:
 * 1. Scroll first - if scroll enabled and not yet revealed, trigger scroll animation
 * 2. Then fragments - reveal fragments one by one
 * 3. Then next slide - advance to next slide
 * Returns true if navigation occurred.
 */
export function nextSlide(): boolean {
	let navigated = false;

	// Get current slide data once (not reactively) to avoid re-running logic
	// when slide index changes
	let slideData: Slide | null = null;
	currentSlide.subscribe(($currentSlide) => {
		slideData = $currentSlide;
	})();

	if (!slideData) return false;

	// Check if scroll needs to be triggered first
	const hasScroll = slideData.scroll === true;

	if (hasScroll) {
		let isScrollRevealed = false;
		scrollRevealed.subscribe((v) => (isScrollRevealed = v))();

		if (!isScrollRevealed) {
			// Trigger scroll animation
			scrollRevealed.set(true);
			// Increment trigger count to signal this is a user action
			scrollTriggerCount.update((n) => n + 1);
			return true;
		}
	}

	// Only treat slides with 2+ fragments as having fragments to reveal
	// (1 fragment means no pause markers - just show slide directly)
	const fragmentCount = slideData.fragments?.length ?? 0;
	const hasRealFragments = fragmentCount > 1;

	currentFragmentIndex.update(($fragmentIndex) => {
		// If there are more fragments to reveal, show next fragment
		if (hasRealFragments && $fragmentIndex < fragmentCount - 1) {
			navigated = true;
			return $fragmentIndex + 1;
		}
		return $fragmentIndex;
	});

	// If no fragment was revealed, move to next slide
	if (!navigated) {
		presentation.subscribe(($presentation) => {
			currentSlideIndex.update(($slideIndex) => {
				const total = $presentation?.slides.length ?? 0;
				if ($slideIndex < total - 1) {
					navigated = true;
					// Reset fragment index and scroll state for new slide
					currentFragmentIndex.set(-1);
					scrollRevealed.set(false);
					updateURLHash($slideIndex + 1);
					return $slideIndex + 1;
				}
				return $slideIndex;
			});
		})();
	}

	return navigated;
}

/**
 * Navigate to the previous slide.
 * Order of operations (reverse of nextSlide):
 * 1. Fragments first - hide fragments in reverse order
 * 2. Then scroll - animate back to top of slide
 * 3. Then prev slide - go to previous slide (scrolled to bottom, all fragments visible)
 * Returns true if navigation occurred.
 */
export function prevSlide(): boolean {
	let navigated = false;

	// Get current slide data once (not reactively)
	let slideData: Slide | null = null;
	currentSlide.subscribe(($currentSlide) => {
		slideData = $currentSlide;
	})();

	if (!slideData) return false;

	// Only treat slides with 2+ fragments as having fragments
	const fragmentCount = slideData.fragments?.length ?? 0;
	const hasRealFragments = fragmentCount > 1;

	currentFragmentIndex.update(($fragmentIndex) => {
		// If there are visible fragments, hide the last one
		if (hasRealFragments && $fragmentIndex >= 0) {
			navigated = true;
			return $fragmentIndex - 1;
		}
		return $fragmentIndex;
	});

	// If no fragment was hidden, check if we need to scroll back to top
	if (!navigated) {
		const hasScroll = slideData.scroll === true;
		if (hasScroll) {
			let isScrollRevealed = false;
			scrollRevealed.subscribe((v) => (isScrollRevealed = v))();

			if (isScrollRevealed) {
				// Scroll back to top
				scrollRevealed.set(false);
				return true;
			}
		}
	}

	// If no fragment was hidden and no scroll to reset, move to previous slide
	if (!navigated) {
		presentation.subscribe(($presentation) => {
			currentSlideIndex.update(($slideIndex) => {
				if ($slideIndex > 0) {
					navigated = true;
					const newSlideIndex = $slideIndex - 1;
					const newSlide = $presentation?.slides[newSlideIndex];
					const newFragmentCount = newSlide?.fragments?.length ?? 0;
					const newHasRealFragments = newFragmentCount > 1;
					const newHasScroll = newSlide?.scroll === true;
					// Show all fragments on the previous slide (or -1 if no real fragments)
					currentFragmentIndex.set(newHasRealFragments ? newFragmentCount - 1 : -1);
					// Set scroll to revealed state (scrolled to bottom) on previous slide
					scrollRevealed.set(newHasScroll);
					updateURLHash(newSlideIndex);
					return newSlideIndex;
				}
				return $slideIndex;
			});
		})();
	}

	return navigated;
}

/**
 * Navigate directly to a specific slide.
 * Resets fragment index to -1 (all fragments hidden) and scroll to top.
 */
export function goToSlide(index: number): boolean {
	let navigated = false;

	presentation.subscribe(($presentation) => {
		const total = $presentation?.slides.length ?? 0;
		if (index >= 0 && index < total) {
			currentSlideIndex.set(index);
			currentFragmentIndex.set(-1);
			scrollRevealed.set(false);
			updateURLHash(index);
			navigated = true;
		}
	})();

	return navigated;
}

/**
 * Navigate to the next fragment without changing slides.
 * Returns true if a fragment was revealed.
 */
export function nextFragment(): boolean {
	let revealed = false;

	currentSlide.subscribe(($currentSlide) => {
		const fragmentCount = $currentSlide?.fragments?.length ?? 0;
		// Only treat slides with 2+ fragments as having fragments
		if (fragmentCount <= 1) return;

		currentFragmentIndex.update(($fragmentIndex) => {
			if ($fragmentIndex < fragmentCount - 1) {
				revealed = true;
				return $fragmentIndex + 1;
			}
			return $fragmentIndex;
		});
	})();

	return revealed;
}

/**
 * Navigate to the previous fragment without changing slides.
 * Returns true if a fragment was hidden.
 */
export function prevFragment(): boolean {
	let hidden = false;

	currentSlide.subscribe(($currentSlide) => {
		const fragmentCount = $currentSlide?.fragments?.length ?? 0;
		// Only treat slides with 2+ fragments as having fragments
		if (fragmentCount <= 1) return;

		currentFragmentIndex.update(($fragmentIndex) => {
			if ($fragmentIndex >= 0) {
				hidden = true;
				return $fragmentIndex - 1;
			}
			return $fragmentIndex;
		});
	})();

	return hidden;
}

// ============================================================================
// URL Hash Management
// ============================================================================

/**
 * Update the URL hash to reflect the current slide.
 * Uses #1, #2, etc. (1-based for user-friendly URLs).
 */
function updateURLHash(slideIndex: number): void {
	if (typeof window !== 'undefined') {
		const hash = `#${slideIndex + 1}`;
		// Use replaceState to avoid polluting browser history
		window.history.replaceState(null, '', hash);
	}
}

/**
 * Parse the URL hash to get the slide index.
 * Returns 0 if hash is invalid or not present.
 */
function parseURLHash(): number {
	if (typeof window === 'undefined') {
		return 0;
	}

	const hash = window.location.hash;
	if (!hash || hash === '#') {
		return 0;
	}

	// Parse #1, #2, etc. (1-based) to 0-based index
	const slideNumber = parseInt(hash.slice(1), 10);
	if (isNaN(slideNumber) || slideNumber < 1) {
		return 0;
	}

	return slideNumber - 1;
}

/**
 * Initialize the store from the URL hash.
 * Call this after loading the presentation data.
 */
export function initializeFromURL(): void {
	const slideIndex = parseURLHash();

	presentation.subscribe(($presentation) => {
		const total = $presentation?.slides.length ?? 0;
		if (total > 0) {
			// Clamp to valid range
			const validIndex = Math.min(slideIndex, total - 1);
			currentSlideIndex.set(validIndex);
			currentFragmentIndex.set(-1);
			scrollRevealed.set(false);
		}
	})();
}

/**
 * Set up a listener for hashchange events.
 * Returns an unsubscribe function.
 */
export function setupHashChangeListener(): () => void {
	if (typeof window === 'undefined') {
		return () => {};
	}

	const handleHashChange = (): void => {
		const slideIndex = parseURLHash();
		presentation.subscribe(($presentation) => {
			const total = $presentation?.slides.length ?? 0;
			if (slideIndex >= 0 && slideIndex < total) {
				currentSlideIndex.set(slideIndex);
				currentFragmentIndex.set(-1);
				scrollRevealed.set(false);
			}
		})();
	};

	window.addEventListener('hashchange', handleHashChange);

	return () => {
		window.removeEventListener('hashchange', handleHashChange);
	};
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Reset the presentation state.
 */
export function resetPresentation(): void {
	presentation.set(null);
	currentSlideIndex.set(0);
	currentFragmentIndex.set(-1);
	scrollRevealed.set(false);
}

/**
 * Load a presentation and initialize from URL hash.
 */
export function loadPresentation(data: Presentation): void {
	presentation.set(data);
	initializeFromURL();
}

/**
 * Set the theme override from WebSocket message.
 * This temporarily overrides the theme without modifying the markdown file.
 */
export function setThemeOverride(theme: Theme): void {
	themeOverride.set(theme);
}

/**
 * Clear the theme override, reverting to the presentation config theme.
 */
export function clearThemeOverride(): void {
	themeOverride.set(null);
}
