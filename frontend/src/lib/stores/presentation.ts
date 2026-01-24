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

// ============================================================================
// Navigation Actions
// ============================================================================

/**
 * Navigate to the next slide.
 * If there are unrevealed fragments (2+ fragments means actual pause markers),
 * reveals the next fragment instead.
 * Returns true if navigation occurred.
 */
export function nextSlide(): boolean {
	let navigated = false;

	currentSlide.subscribe(($currentSlide) => {
		// Only treat slides with 2+ fragments as having fragments to reveal
		// (1 fragment means no pause markers - just show slide directly)
		const fragmentCount = $currentSlide?.fragments?.length ?? 0;
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
						// Reset fragment index for new slide
						currentFragmentIndex.set(-1);
						updateURLHash($slideIndex + 1);
						return $slideIndex + 1;
					}
					return $slideIndex;
				});
			})();
		}
	})();

	return navigated;
}

/**
 * Navigate to the previous slide.
 * If there are visible fragments (on slides with 2+ fragments), hides the last visible fragment instead.
 * Returns true if navigation occurred.
 */
export function prevSlide(): boolean {
	let navigated = false;

	currentSlide.subscribe(($currentSlide) => {
		// Only treat slides with 2+ fragments as having fragments
		const fragmentCount = $currentSlide?.fragments?.length ?? 0;
		const hasRealFragments = fragmentCount > 1;

		currentFragmentIndex.update(($fragmentIndex) => {
			// If there are visible fragments, hide the last one
			if (hasRealFragments && $fragmentIndex >= 0) {
				navigated = true;
				return $fragmentIndex - 1;
			}
			return $fragmentIndex;
		});
	})();

	// If no fragment was hidden, move to previous slide
	if (!navigated) {
		presentation.subscribe(($presentation) => {
			currentSlideIndex.update(($slideIndex) => {
				if ($slideIndex > 0) {
					navigated = true;
					const newSlideIndex = $slideIndex - 1;
					const newSlide = $presentation?.slides[newSlideIndex];
					const fragmentCount = newSlide?.fragments?.length ?? 0;
					const hasRealFragments = fragmentCount > 1;
					// Show all fragments on the previous slide (or -1 if no real fragments)
					currentFragmentIndex.set(hasRealFragments ? fragmentCount - 1 : -1);
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
 * Resets fragment index to -1 (all fragments hidden).
 */
export function goToSlide(index: number): boolean {
	let navigated = false;

	presentation.subscribe(($presentation) => {
		const total = $presentation?.slides.length ?? 0;
		if (index >= 0 && index < total) {
			currentSlideIndex.set(index);
			currentFragmentIndex.set(-1);
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
