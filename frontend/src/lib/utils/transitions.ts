/**
 * Slide transitions for Tap presentations.
 * Provides reusable transition functions using Svelte's built-in transitions.
 *
 * Supports 5 transition types:
 * - none: Instant transition (no animation)
 * - fade: Opacity crossfade (default)
 * - slide: Horizontal slide in/out
 * - push: Horizontal slide with slight overlap
 * - zoom: Scale in/out
 */

import { fade, fly, scale, crossfade } from 'svelte/transition';
import type { TransitionConfig as SvelteTransitionConfig } from 'svelte/transition';
import type { Transition } from '$lib/types';

// ============================================================================
// Types
// ============================================================================

/**
 * Direction of slide transition.
 */
export type TransitionDirection = 'forward' | 'backward';

/**
 * Options for creating a slide transition.
 */
export interface TransitionOptions {
	/** Transition type to use */
	type?: Transition;
	/** Duration in milliseconds (default: 400) */
	duration?: number;
	/** Direction for directional transitions */
	direction?: TransitionDirection;
	/** Delay before transition starts in milliseconds */
	delay?: number;
}

/**
 * Configuration for transition defaults.
 */
export interface TransitionDefaults {
	/** Default transition type */
	defaultTransition: Transition;
	/** Default duration in milliseconds */
	defaultDuration: number;
}

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

// ============================================================================
// Transition Functions
// ============================================================================

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

/**
 * Create a "none" transition (instant, no animation).
 */
export function transitionNone(
	_node: Element,
	_options?: TransitionOptions
): SvelteTransitionConfig {
	return {
		duration: 0,
		css: () => ''
	};
}

/**
 * Create a "fade" transition (opacity crossfade).
 */
export function transitionFade(
	node: Element,
	options: TransitionOptions = {}
): SvelteTransitionConfig {
	const duration = getEffectiveDuration(
		options.duration ?? TRANSITION_DEFAULTS.defaultDuration
	);
	const delay = options.delay ?? 0;

	return fade(node, { duration, delay });
}

/**
 * Create a "slide" transition (horizontal slide in/out).
 */
export function transitionSlide(
	node: Element,
	options: TransitionOptions = {}
): SvelteTransitionConfig {
	const duration = getEffectiveDuration(
		options.duration ?? TRANSITION_DEFAULTS.defaultDuration
	);
	const delay = options.delay ?? 0;
	const direction = options.direction ?? 'forward';

	// Slide from right when going forward, from left when going backward
	const x = direction === 'forward' ? 100 : -100;

	return fly(node, { x, duration, delay });
}

/**
 * Create a "push" transition (horizontal slide with opacity).
 * Similar to slide but with a softer overlap effect.
 */
export function transitionPush(
	node: Element,
	options: TransitionOptions = {}
): SvelteTransitionConfig {
	const duration = getEffectiveDuration(
		options.duration ?? TRANSITION_DEFAULTS.defaultDuration
	);
	const delay = options.delay ?? 0;
	const direction = options.direction ?? 'forward';

	// Smaller distance than slide for push effect
	const x = direction === 'forward' ? 50 : -50;

	return fly(node, { x, duration, delay, opacity: 0.5 });
}

/**
 * Create a "zoom" transition (scale in/out).
 */
export function transitionZoom(
	node: Element,
	options: TransitionOptions = {}
): SvelteTransitionConfig {
	const duration = getEffectiveDuration(
		options.duration ?? TRANSITION_DEFAULTS.defaultDuration
	);
	const delay = options.delay ?? 0;
	const direction = options.direction ?? 'forward';

	// Scale down when entering forward, scale up when entering backward
	const start = direction === 'forward' ? 0.8 : 1.2;

	return scale(node, { start, duration, delay });
}

// ============================================================================
// Main Transition Factory
// ============================================================================

/**
 * Get the transition function for a given transition type.
 */
export function getTransitionFunction(
	type: Transition
): (node: Element, options?: TransitionOptions) => SvelteTransitionConfig {
	switch (type) {
		case 'none':
			return transitionNone;
		case 'slide':
			return transitionSlide;
		case 'push':
			return transitionPush;
		case 'zoom':
			return transitionZoom;
		case 'fade':
		default:
			return transitionFade;
	}
}

/**
 * Create a transition for a slide element.
 * This is the main function to use for slide transitions.
 *
 * @param node - The DOM element to transition
 * @param options - Transition options
 * @returns Svelte transition config
 *
 * @example
 * ```svelte
 * <script>
 *   import { createSlideTransition } from '$lib/utils/transitions';
 *
 *   let transitionType = 'fade';
 *   let direction = 'forward';
 * </script>
 *
 * <div
 *   in:createSlideTransition={{ type: transitionType, direction }}
 *   out:createSlideTransition={{ type: transitionType, direction }}
 * >
 *   Slide content
 * </div>
 * ```
 */
export function createSlideTransition(
	node: Element,
	options: TransitionOptions = {}
): SvelteTransitionConfig {
	const type = options.type ?? TRANSITION_DEFAULTS.defaultTransition;
	const transitionFn = getTransitionFunction(type);
	return transitionFn(node, options);
}

// ============================================================================
// Crossfade Transition
// ============================================================================

/**
 * Create a crossfade transition pair for smooth slide switching.
 * Returns [send, receive] functions for use with Svelte's crossfade.
 *
 * @example
 * ```svelte
 * <script>
 *   import { createCrossfade } from '$lib/utils/transitions';
 *
 *   const [send, receive] = createCrossfade({ duration: 400 });
 * </script>
 *
 * {#each slides as slide (slide.index)}
 *   <div in:receive={{ key: slide.index }} out:send={{ key: slide.index }}>
 *     {slide.content}
 *   </div>
 * {/each}
 * ```
 */
export function createCrossfade(
	options: { duration?: number; delay?: number } = {}
) {
	const duration = getEffectiveDuration(
		options.duration ?? TRANSITION_DEFAULTS.defaultDuration
	);
	const delay = options.delay ?? 0;

	return crossfade({
		duration,
		delay,
		fallback: (node) => fade(node, { duration })
	});
}

// ============================================================================
// Utility Functions
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
