/**
 * Animation presets for Tap presentations.
 *
 * Provides built-in animation presets that can be applied to slide content:
 * - typewriter: Character-by-character reveal
 * - count-up: Animate numbers from 0 to target value
 * - cascade: Staggered list item entrance
 * - spring: Spring physics motion
 *
 * Animations are triggered when slides enter and respect reduced motion preferences.
 */

import { cubicOut } from 'svelte/easing';

// ============================================================================
// Types
// ============================================================================

/**
 * Available animation preset names.
 */
export type AnimationPreset = 'typewriter' | 'count-up' | 'cascade' | 'spring' | 'none';

/**
 * Options for typewriter animation.
 */
export interface TypewriterOptions {
	/** Total duration of the animation in milliseconds. @default 1000 */
	duration?: number;
	/** Delay before animation starts in milliseconds. @default 0 */
	delay?: number;
	/** Text to animate (if not using element's textContent). */
	text?: string;
	/** Optional cursor character to show during typing. @default '' */
	cursor?: string;
}

/**
 * Options for count-up animation.
 */
export interface CountUpOptions {
	/** Target number to count up to. @default parsed from element */
	target?: number;
	/** Starting number. @default 0 */
	start?: number;
	/** Duration of the animation in milliseconds. @default 2000 */
	duration?: number;
	/** Delay before animation starts in milliseconds. @default 0 */
	delay?: number;
	/** Number of decimal places to show. @default 0 */
	decimals?: number;
	/** Prefix string (e.g., '$'). @default '' */
	prefix?: string;
	/** Suffix string (e.g., '%'). @default '' */
	suffix?: string;
	/** Separator for thousands (e.g., ','). @default '' */
	separator?: string;
}

/**
 * Options for cascade animation.
 */
export interface CascadeOptions {
	/** Duration for each item's animation in milliseconds. @default 400 */
	duration?: number;
	/** Delay between each item in milliseconds. @default 100 */
	stagger?: number;
	/** Initial delay before first item in milliseconds. @default 0 */
	delay?: number;
	/** Animation direction: 'up', 'down', 'left', 'right', 'fade'. @default 'up' */
	direction?: 'up' | 'down' | 'left' | 'right' | 'fade';
	/** Distance to travel in pixels. @default 30 */
	distance?: number;
}

/**
 * Options for spring animation.
 */
export interface SpringOptions {
	/** Stiffness of the spring (0-1). @default 0.15 */
	stiffness?: number;
	/** Damping ratio (0-1). @default 0.8 */
	damping?: number;
	/** Delay before animation starts in milliseconds. @default 0 */
	delay?: number;
	/** Transform to apply: 'scale', 'translateY', 'translateX', 'rotate'. @default 'scale' */
	transform?: 'scale' | 'translateY' | 'translateX' | 'rotate';
	/** Initial value for the transform. */
	from?: number;
	/** Final value for the transform. @default depends on transform type */
	to?: number;
}

/**
 * Result from animation setup functions.
 */
export interface AnimationController {
	/** Start or restart the animation. */
	start: () => void;
	/** Stop/cancel the animation. */
	stop: () => void;
	/** Clean up resources. */
	destroy: () => void;
	/** Whether the animation is currently running. */
	readonly running: boolean;
}

// ============================================================================
// Constants
// ============================================================================

/**
 * All available animation presets.
 */
export const ANIMATION_PRESETS: readonly AnimationPreset[] = [
	'typewriter',
	'count-up',
	'cascade',
	'spring',
	'none'
] as const;

/**
 * Default durations for each animation type.
 */
export const ANIMATION_DEFAULTS = {
	typewriter: { duration: 1000, delay: 0 },
	'count-up': { duration: 2000, delay: 0, decimals: 0 },
	cascade: { duration: 400, stagger: 100, delay: 0, distance: 30 },
	spring: { stiffness: 0.15, damping: 0.8, delay: 0 }
} as const;

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
export function onReducedMotionChange(callback: (prefersReduced: boolean) => void): () => void {
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
// Typewriter Animation
// ============================================================================

/**
 * Create a typewriter animation that reveals text character by character.
 *
 * @param element - The DOM element containing the text to animate
 * @param options - Animation options
 * @returns Animation controller
 *
 * @example
 * ```typescript
 * const controller = typewriter(element, { duration: 1500 });
 * controller.start();
 * ```
 */
export function typewriter(element: HTMLElement, options: TypewriterOptions = {}): AnimationController {
	const { duration = 1000, delay = 0, cursor = '' } = options;
	const text = options.text ?? element.textContent ?? '';

	let animationId: number | null = null;
	let timeoutId: ReturnType<typeof setTimeout> | null = null;
	let running = false;

	// Store original text for reset
	const originalText = element.textContent;

	function start() {
		stop();
		running = true;

		// Respect reduced motion
		if (prefersReducedMotion()) {
			element.textContent = text;
			running = false;
			return;
		}

		// Clear element initially
		element.textContent = cursor;

		timeoutId = setTimeout(() => {
			const startTime = performance.now();
			const charCount = text.length;

			function animate(currentTime: number) {
				if (!running) return;

				const elapsed = currentTime - startTime;
				const progress = Math.min(elapsed / duration, 1);
				const charIndex = Math.floor(progress * charCount);

				element.textContent = text.slice(0, charIndex) + (progress < 1 ? cursor : '');

				if (progress < 1) {
					animationId = requestAnimationFrame(animate);
				} else {
					running = false;
				}
			}

			animationId = requestAnimationFrame(animate);
		}, delay);
	}

	function stop() {
		running = false;
		if (animationId !== null) {
			cancelAnimationFrame(animationId);
			animationId = null;
		}
		if (timeoutId !== null) {
			clearTimeout(timeoutId);
			timeoutId = null;
		}
	}

	function destroy() {
		stop();
		if (originalText !== null) {
			element.textContent = originalText;
		}
	}

	return {
		start,
		stop,
		destroy,
		get running() {
			return running;
		}
	};
}

// ============================================================================
// Count-Up Animation
// ============================================================================

/**
 * Format a number with optional separators, prefix, suffix, and decimal places.
 */
function formatNumber(
	value: number,
	decimals: number,
	separator: string,
	prefix: string,
	suffix: string
): string {
	const fixed = value.toFixed(decimals);
	const [intPart, decPart] = fixed.split('.');

	// Add thousand separators if specified
	let formattedInt = intPart;
	if (separator && intPart) {
		formattedInt = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, separator);
	}

	const formatted = decPart ? `${formattedInt}.${decPart}` : formattedInt;
	return `${prefix}${formatted}${suffix}`;
}

/**
 * Parse a number from a string, handling common formats.
 */
function parseFormattedNumber(text: string): number {
	// Remove common formatting characters and parse
	const cleaned = text.replace(/[$%,\s]/g, '');
	const parsed = parseFloat(cleaned);
	return isNaN(parsed) ? 0 : parsed;
}

/**
 * Create a count-up animation that animates a number from start to target.
 *
 * @param element - The DOM element to display the number
 * @param options - Animation options
 * @returns Animation controller
 *
 * @example
 * ```typescript
 * const controller = countUp(element, { target: 1000, duration: 2000 });
 * controller.start();
 * ```
 */
export function countUp(element: HTMLElement, options: CountUpOptions = {}): AnimationController {
	const {
		start: startValue = 0,
		duration = 2000,
		delay = 0,
		decimals = 0,
		prefix = '',
		suffix = '',
		separator = ''
	} = options;

	// Parse target from element if not provided
	const target = options.target ?? parseFormattedNumber(element.textContent ?? '0');

	let animationId: number | null = null;
	let timeoutId: ReturnType<typeof setTimeout> | null = null;
	let running = false;

	// Store original content
	const originalText = element.textContent;

	function start() {
		stop();
		running = true;

		// Respect reduced motion
		if (prefersReducedMotion()) {
			element.textContent = formatNumber(target, decimals, separator, prefix, suffix);
			running = false;
			return;
		}

		// Show start value initially
		element.textContent = formatNumber(startValue, decimals, separator, prefix, suffix);

		timeoutId = setTimeout(() => {
			const startTime = performance.now();
			const range = target - startValue;

			function animate(currentTime: number) {
				if (!running) return;

				const elapsed = currentTime - startTime;
				const progress = Math.min(elapsed / duration, 1);

				// Use easing for smooth animation
				const easedProgress = cubicOut(progress);
				const currentValue = startValue + range * easedProgress;

				element.textContent = formatNumber(currentValue, decimals, separator, prefix, suffix);

				if (progress < 1) {
					animationId = requestAnimationFrame(animate);
				} else {
					running = false;
				}
			}

			animationId = requestAnimationFrame(animate);
		}, delay);
	}

	function stop() {
		running = false;
		if (animationId !== null) {
			cancelAnimationFrame(animationId);
			animationId = null;
		}
		if (timeoutId !== null) {
			clearTimeout(timeoutId);
			timeoutId = null;
		}
	}

	function destroy() {
		stop();
		if (originalText !== null) {
			element.textContent = originalText;
		}
	}

	return {
		start,
		stop,
		destroy,
		get running() {
			return running;
		}
	};
}

// ============================================================================
// Cascade Animation
// ============================================================================

/**
 * Get CSS transform string for cascade animation direction.
 */
function getCascadeTransform(direction: CascadeOptions['direction'], distance: number, progress: number): string {
	const offset = (1 - progress) * distance;

	switch (direction) {
		case 'up':
			return `translateY(${offset}px)`;
		case 'down':
			return `translateY(${-offset}px)`;
		case 'left':
			return `translateX(${offset}px)`;
		case 'right':
			return `translateX(${-offset}px)`;
		case 'fade':
		default:
			return 'none';
	}
}

/**
 * Create a cascade animation that animates child elements with staggered timing.
 *
 * @param container - The container element with children to animate
 * @param options - Animation options
 * @returns Animation controller
 *
 * @example
 * ```typescript
 * const controller = cascade(listElement, { stagger: 100, direction: 'up' });
 * controller.start();
 * ```
 */
export function cascade(container: HTMLElement, options: CascadeOptions = {}): AnimationController {
	const {
		duration = 400,
		stagger = 100,
		delay = 0,
		direction = 'up',
		distance = 30
	} = options;

	const children = Array.from(container.children) as HTMLElement[];
	const animationIds: number[] = [];
	let timeoutId: ReturnType<typeof setTimeout> | null = null;
	let running = false;

	// Store original styles
	const originalStyles = children.map((child) => ({
		opacity: child.style.opacity,
		transform: child.style.transform
	}));

	function start() {
		stop();
		running = true;

		// Respect reduced motion
		if (prefersReducedMotion()) {
			children.forEach((child) => {
				child.style.opacity = '1';
				child.style.transform = 'none';
			});
			running = false;
			return;
		}

		// Hide all children initially
		children.forEach((child) => {
			child.style.opacity = '0';
			child.style.transform = getCascadeTransform(direction, distance, 0);
		});

		timeoutId = setTimeout(() => {
			children.forEach((child, index) => {
				const itemDelay = index * stagger;
				const startTime = performance.now() + itemDelay;

				function animateItem(currentTime: number) {
					if (!running) return;

					const elapsed = currentTime - startTime;
					if (elapsed < 0) {
						// Not started yet
						animationIds.push(requestAnimationFrame(animateItem));
						return;
					}

					const progress = Math.min(elapsed / duration, 1);
					const easedProgress = cubicOut(progress);

					child.style.opacity = String(easedProgress);
					child.style.transform = getCascadeTransform(direction, distance, easedProgress);

					if (progress < 1) {
						animationIds.push(requestAnimationFrame(animateItem));
					}
				}

				animationIds.push(requestAnimationFrame(animateItem));
			});
		}, delay);
	}

	function stop() {
		running = false;
		animationIds.forEach((id) => cancelAnimationFrame(id));
		animationIds.length = 0;
		if (timeoutId !== null) {
			clearTimeout(timeoutId);
			timeoutId = null;
		}
	}

	function destroy() {
		stop();
		// Restore original styles
		children.forEach((child, index) => {
			const original = originalStyles[index];
			if (original) {
				child.style.opacity = original.opacity;
				child.style.transform = original.transform;
			}
		});
	}

	return {
		start,
		stop,
		destroy,
		get running() {
			return running;
		}
	};
}

// ============================================================================
// Spring Animation
// ============================================================================

/**
 * Simple spring physics simulation.
 * Based on a critically damped spring model.
 */
function springPhysics(
	current: number,
	target: number,
	velocity: number,
	stiffness: number,
	damping: number
): { position: number; velocity: number } {
	// Spring force
	const springForce = (target - current) * stiffness;
	// Damping force
	const dampingForce = -velocity * damping;
	// Total force = acceleration
	const acceleration = springForce + dampingForce;
	// Update velocity and position
	const newVelocity = velocity + acceleration;
	const newPosition = current + newVelocity;

	return {
		position: newPosition,
		velocity: newVelocity
	};
}

/**
 * Get the CSS transform string for spring animation.
 */
function getSpringTransform(transform: SpringOptions['transform'], value: number): string {
	switch (transform) {
		case 'scale':
			return `scale(${value})`;
		case 'translateY':
			return `translateY(${value}px)`;
		case 'translateX':
			return `translateX(${value}px)`;
		case 'rotate':
			return `rotate(${value}deg)`;
		default:
			return `scale(${value})`;
	}
}

/**
 * Get default from/to values for spring transforms.
 */
function getSpringDefaults(transform: SpringOptions['transform']): { from: number; to: number } {
	switch (transform) {
		case 'scale':
			return { from: 0.5, to: 1 };
		case 'translateY':
			return { from: 50, to: 0 };
		case 'translateX':
			return { from: 50, to: 0 };
		case 'rotate':
			return { from: -15, to: 0 };
		default:
			return { from: 0.5, to: 1 };
	}
}

/**
 * Create a spring animation with physics-based motion.
 *
 * @param element - The DOM element to animate
 * @param options - Animation options
 * @returns Animation controller
 *
 * @example
 * ```typescript
 * const controller = spring(element, { transform: 'scale', stiffness: 0.2 });
 * controller.start();
 * ```
 */
export function spring(element: HTMLElement, options: SpringOptions = {}): AnimationController {
	const { stiffness = 0.15, damping = 0.8, delay = 0, transform = 'scale' } = options;

	const defaults = getSpringDefaults(transform);
	const from = options.from ?? defaults.from;
	const to = options.to ?? defaults.to;

	let animationId: number | null = null;
	let timeoutId: ReturnType<typeof setTimeout> | null = null;
	let running = false;

	// Store original styles
	const originalTransform = element.style.transform;
	const originalOpacity = element.style.opacity;

	function start() {
		stop();
		running = true;

		// Respect reduced motion
		if (prefersReducedMotion()) {
			element.style.transform = getSpringTransform(transform, to);
			element.style.opacity = '1';
			running = false;
			return;
		}

		// Set initial state
		element.style.transform = getSpringTransform(transform, from);
		element.style.opacity = '0.5';

		timeoutId = setTimeout(() => {
			let position = from;
			let velocity = 0;

			// Threshold for considering animation complete
			const threshold = 0.001;

			function animate() {
				if (!running) return;

				const result = springPhysics(position, to, velocity, stiffness, damping);
				position = result.position;
				velocity = result.velocity;

				// Calculate opacity based on progress
				const progress = Math.min(Math.abs((position - from) / (to - from)), 1);
				const opacity = 0.5 + progress * 0.5;

				element.style.transform = getSpringTransform(transform, position);
				element.style.opacity = String(opacity);

				// Check if we're close enough to target and velocity is low
				const isSettled = Math.abs(position - to) < threshold && Math.abs(velocity) < threshold;

				if (!isSettled) {
					animationId = requestAnimationFrame(animate);
				} else {
					// Snap to final value
					element.style.transform = getSpringTransform(transform, to);
					element.style.opacity = '1';
					running = false;
				}
			}

			animationId = requestAnimationFrame(animate);
		}, delay);
	}

	function stop() {
		running = false;
		if (animationId !== null) {
			cancelAnimationFrame(animationId);
			animationId = null;
		}
		if (timeoutId !== null) {
			clearTimeout(timeoutId);
			timeoutId = null;
		}
	}

	function destroy() {
		stop();
		element.style.transform = originalTransform;
		element.style.opacity = originalOpacity;
	}

	return {
		start,
		stop,
		destroy,
		get running() {
			return running;
		}
	};
}

// ============================================================================
// Animation Factory
// ============================================================================

/**
 * Union type for all animation options.
 */
export type AnimationOptions = TypewriterOptions | CountUpOptions | CascadeOptions | SpringOptions;

/**
 * Create an animation controller based on preset name.
 *
 * @param preset - The animation preset name
 * @param element - The DOM element to animate
 * @param options - Animation options (type depends on preset)
 * @returns Animation controller or null if preset is 'none'
 *
 * @example
 * ```typescript
 * const controller = createAnimation('typewriter', element, { duration: 1500 });
 * if (controller) {
 *   controller.start();
 * }
 * ```
 */
export function createAnimation(
	preset: AnimationPreset,
	element: HTMLElement,
	options: AnimationOptions = {}
): AnimationController | null {
	switch (preset) {
		case 'typewriter':
			return typewriter(element, options as TypewriterOptions);
		case 'count-up':
			return countUp(element, options as CountUpOptions);
		case 'cascade':
			return cascade(element, options as CascadeOptions);
		case 'spring':
			return spring(element, options as SpringOptions);
		case 'none':
		default:
			return null;
	}
}

// ============================================================================
// Svelte Action Helpers
// ============================================================================

/**
 * Svelte action for typewriter animation.
 * Use with use:typewriterAction directive.
 *
 * @example
 * ```svelte
 * <h1 use:typewriterAction={{ duration: 1500 }}>Hello World</h1>
 * ```
 */
export function typewriterAction(
	element: HTMLElement,
	options: TypewriterOptions = {}
): { destroy: () => void } {
	const controller = typewriter(element, options);
	controller.start();

	return {
		destroy: controller.destroy
	};
}

/**
 * Svelte action for count-up animation.
 * Use with use:countUpAction directive.
 *
 * @example
 * ```svelte
 * <span use:countUpAction={{ target: 1000, duration: 2000 }}>0</span>
 * ```
 */
export function countUpAction(
	element: HTMLElement,
	options: CountUpOptions = {}
): { destroy: () => void } {
	const controller = countUp(element, options);
	controller.start();

	return {
		destroy: controller.destroy
	};
}

/**
 * Svelte action for cascade animation.
 * Use with use:cascadeAction directive on a container.
 *
 * @example
 * ```svelte
 * <ul use:cascadeAction={{ stagger: 100, direction: 'up' }}>
 *   <li>Item 1</li>
 *   <li>Item 2</li>
 * </ul>
 * ```
 */
export function cascadeAction(
	element: HTMLElement,
	options: CascadeOptions = {}
): { destroy: () => void } {
	const controller = cascade(element, options);
	controller.start();

	return {
		destroy: controller.destroy
	};
}

/**
 * Svelte action for spring animation.
 * Use with use:springAction directive.
 *
 * @example
 * ```svelte
 * <div use:springAction={{ transform: 'scale', stiffness: 0.2 }}>
 *   Spring content
 * </div>
 * ```
 */
export function springAction(
	element: HTMLElement,
	options: SpringOptions = {}
): { destroy: () => void } {
	const controller = spring(element, options);
	controller.start();

	return {
		destroy: controller.destroy
	};
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Check if a string is a valid animation preset.
 */
export function isValidAnimationPreset(value: string): value is AnimationPreset {
	return ANIMATION_PRESETS.includes(value as AnimationPreset);
}

/**
 * Get a human-readable description of an animation preset.
 */
export function getAnimationDescription(preset: AnimationPreset): string {
	switch (preset) {
		case 'typewriter':
			return 'Character-by-character text reveal';
		case 'count-up':
			return 'Animate numbers from 0 to target';
		case 'cascade':
			return 'Staggered list item entrance';
		case 'spring':
			return 'Spring physics motion';
		case 'none':
			return 'No animation';
		default:
			return 'Unknown animation';
	}
}
