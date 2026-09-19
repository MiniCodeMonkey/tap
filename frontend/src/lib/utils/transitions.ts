/**
 * Slide transition helpers for Tap presentations.
 * Provides the reduced-motion and print-mode aware defaults, and the Motion
 * variants each transition type animates through.
 *
 * Supports 5 transition types:
 * - none: instant transition (no animation)
 * - fade: opacity crossfade (default)
 * - slide: horizontal slide in/out
 * - push: horizontal slide with a soft opacity overlap
 * - zoom: scale in/out
 */

import type { Transition } from '$lib/types';

// ============================================================================
// Types
// ============================================================================

/**
 * Direction of slide transition.
 */
export type TransitionDirection = 'forward' | 'backward';

/**
 * Configuration for transition defaults.
 */
export interface TransitionDefaults {
	/** Default transition type */
	defaultTransition: Transition;
	/** Default duration in milliseconds */
	defaultDuration: number;
}

/**
 * A Motion animation target, keyed by the CSS/transform properties it sets.
 */
export type TransitionTarget = Record<string, number | string>;

/**
 * A Motion easing value: Motion's named `'linear'`, or a cubic-bezier control
 * point array like Motion's own `Easing` type accepts.
 */
export type TransitionEasing = 'linear' | readonly [number, number, number, number];

/**
 * The three Motion variants a slide transition animates between, plus the
 * easing curve that transition type animates with.
 */
export interface TransitionVariants {
	initial: TransitionTarget;
	animate: TransitionTarget;
	exit: TransitionTarget;
	ease: TransitionEasing;
}

/**
 * Cubic-bezier control points for a fast-out, slow-in ease: motion starts
 * quickly and settles gently into its resting state. Used as the `ease`
 * curve for the slide, push, and zoom transition types.
 */
const CUBIC_OUT: TransitionEasing = [0.33, 1, 0.68, 1];

// ============================================================================
// Constants
// ============================================================================

/**
 * Default transition configuration.
 */
export const TRANSITION_DEFAULTS: TransitionDefaults = {
	defaultTransition: 'fade',
	defaultDuration: 400
};

/**
 * All available transition types.
 */
export const TRANSITION_TYPES: readonly Transition[] = [
	'none',
	'fade',
	'slide',
	'push',
	'zoom'
] as const;

// ============================================================================
// Reduced Motion Support
// ============================================================================

/**
 * Check if the user prefers reduced motion.
 * Returns true if the user has enabled reduced motion in their OS settings.
 */
export function prefersReducedMotion(): boolean {
	if (typeof window === 'undefined') {
		return false;
	}
	return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/**
 * Subscribe to reduced motion preference changes.
 * Returns an unsubscribe function.
 */
export function onReducedMotionChange(
	callback: (prefersReduced: boolean) => void
): () => void {
	if (typeof window === 'undefined') {
		return () => {};
	}

	const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
	const handler = (event: MediaQueryListEvent) => callback(event.matches);

	mediaQuery.addEventListener('change', handler);

	return () => {
		mediaQuery.removeEventListener('change', handler);
	};
}

/**
 * Check if the page is in print/PDF mode.
 * Print mode is indicated by ?print=true in the URL.
 */
export function isPrintMode(): boolean {
	if (typeof window === 'undefined') {
		return false;
	}
	return new URLSearchParams(window.location.search).get('print') === 'true';
}

/**
 * Get the effective duration, respecting reduced motion preferences and print mode.
 * Returns 0 if user prefers reduced motion or if in print mode (for PDF export).
 */
export function getEffectiveDuration(duration: number): number {
	if (prefersReducedMotion() || isPrintMode()) {
		return 0;
	}
	return duration;
}

// ============================================================================
// Transition Resolution
// ============================================================================

/**
 * Validate that a string is a valid transition type.
 */
export function isValidTransition(value: string): value is Transition {
	return TRANSITION_TYPES.includes(value as Transition);
}

/**
 * Get transition type from a value, with fallback to default.
 */
export function resolveTransition(
	slideTransition: Transition | undefined,
	globalTransition: Transition | undefined
): Transition {
	// Per-slide transition overrides global
	if (slideTransition && isValidTransition(slideTransition)) {
		return slideTransition;
	}

	// Fall back to global transition
	if (globalTransition && isValidTransition(globalTransition)) {
		return globalTransition;
	}

	// Default transition
	return TRANSITION_DEFAULTS.defaultTransition;
}

/**
 * Get a human-readable description of a transition type.
 */
export function getTransitionDescription(type: Transition): string {
	switch (type) {
		case 'none':
			return 'No transition (instant)';
		case 'fade':
			return 'Crossfade (opacity)';
		case 'slide':
			return 'Slide horizontally';
		case 'push':
			return 'Push with overlap';
		case 'zoom':
			return 'Zoom in/out';
		default:
			return 'Unknown transition';
	}
}

// ============================================================================
// Motion Variants
// ============================================================================

/**
 * Get the Motion variants for a slide transition type and direction.
 * `initial` is where the entering slide starts, `animate` is its resting
 * state, and `exit` is where the leaving slide animates to. Directional
 * transitions (slide, push, zoom) mirror their offsets for `backward`.
 *
 * `fade` eases linearly, a plain constant-speed cross-fade; `slide`,
 * `push`, and `zoom` ease with CUBIC_OUT, the fast-out, slow-in curve that
 * suits a directional motion settling into place.
 */
export function getTransitionVariants(
	type: Transition,
	direction: TransitionDirection
): TransitionVariants {
	switch (type) {
		case 'none':
			return { initial: {}, animate: {}, exit: {}, ease: 'linear' };

		case 'slide': {
			const x = direction === 'forward' ? 100 : -100;
			return {
				initial: { x },
				animate: { x: 0 },
				exit: { x },
				ease: CUBIC_OUT
			};
		}

		case 'push': {
			const x = direction === 'forward' ? 50 : -50;
			return {
				initial: { x, opacity: 0.5 },
				animate: { x: 0, opacity: 1 },
				exit: { x, opacity: 0.5 },
				ease: CUBIC_OUT
			};
		}

		case 'zoom': {
			const scale = direction === 'forward' ? 0.8 : 1.2;
			return {
				initial: { scale },
				animate: { scale: 1 },
				exit: { scale },
				ease: CUBIC_OUT
			};
		}

		case 'fade':
		default:
			return {
				initial: { opacity: 0 },
				animate: { opacity: 1 },
				exit: { opacity: 0 },
				ease: 'linear'
			};
	}
}
