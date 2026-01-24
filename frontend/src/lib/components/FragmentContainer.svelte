<script lang="ts">
	/**
	 * FragmentContainer - Container for incremental content reveals.
	 *
	 * This component manages the display and animation of fragment groups
	 * within a slide, supporting incremental reveals controlled by keyboard
	 * navigation or direct fragment index manipulation.
	 *
	 * Fragment groups are content sections separated by <!-- pause --> markers
	 * in the markdown source.
	 */

	import type { FragmentGroup } from '$lib/types';
	import { cubicOut } from 'svelte/easing';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/**
		 * Array of fragment groups to display.
		 * Each fragment has an index and HTML content.
		 */
		fragments: FragmentGroup[];

		/**
		 * Current fragment index (0-based).
		 * -1 means no fragments are visible (initial state).
		 * 0 means first fragment is visible, etc.
		 */
		currentIndex: number;

		/**
		 * Animation type for fragment reveals.
		 * @default 'fade'
		 */
		animation?: 'fade' | 'slide-up' | 'slide-left' | 'scale' | 'none';

		/**
		 * Duration of fragment animation in milliseconds.
		 * @default 400
		 */
		duration?: number;

		/**
		 * Delay between fragment start and animation in milliseconds.
		 * @default 0
		 */
		delay?: number;

		/**
		 * Whether to show all fragments at once (no incremental reveal).
		 * Useful for print mode or when fragments are disabled.
		 * @default false
		 */
		showAll?: boolean;
	}

	let {
		fragments,
		currentIndex,
		animation = 'fade',
		duration = 400,
		delay = 0,
		showAll = false
	}: Props = $props();

	// ============================================================================
	// Computed Values
	// ============================================================================

	/**
	 * Check if reduced motion is preferred.
	 */
	function prefersReducedMotion(): boolean {
		if (typeof window === 'undefined') return false;
		return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
	}

	/**
	 * Get fragments with their visibility state.
	 */
	let fragmentsWithState = $derived(
		fragments.map((fragment) => ({
			...fragment,
			visible: showAll || fragment.index <= currentIndex
		}))
	);

	/**
	 * Get the effective animation duration (0 for reduced motion).
	 */
	let effectiveDuration = $derived(prefersReducedMotion() ? 0 : duration);

	// ============================================================================
	// Animation Functions
	// ============================================================================

	/**
	 * Get the transition parameters for a fragment reveal.
	 */
	function getTransitionIn(
		node: Element,
		params: { index: number }
	): {
		delay: number;
		duration: number;
		css?: (t: number, u: number) => string;
		easing?: (t: number) => number;
	} {
		const baseDelay = delay + params.index * 50; // Stagger by 50ms per fragment

		if (prefersReducedMotion() || animation === 'none') {
			return { delay: 0, duration: 0 };
		}

		switch (animation) {
			case 'slide-up':
				return {
					delay: baseDelay,
					duration: effectiveDuration,
					easing: cubicOut,
					css: (t) => `
						opacity: ${t};
						transform: translateY(${(1 - t) * 30}px);
					`
				};

			case 'slide-left':
				return {
					delay: baseDelay,
					duration: effectiveDuration,
					easing: cubicOut,
					css: (t) => `
						opacity: ${t};
						transform: translateX(${(1 - t) * 30}px);
					`
				};

			case 'scale':
				return {
					delay: baseDelay,
					duration: effectiveDuration,
					easing: cubicOut,
					css: (t) => `
						opacity: ${t};
						transform: scale(${0.95 + t * 0.05});
					`
				};

			case 'fade':
			default:
				return {
					delay: baseDelay,
					duration: effectiveDuration,
					easing: cubicOut,
					css: (t) => `opacity: ${t};`
				};
		}
	}

	/**
	 * Get the transition parameters for a fragment hiding.
	 */
	function getTransitionOut(
		_node: Element,
		_params: { index: number }
	): {
		delay: number;
		duration: number;
		css?: (t: number, u: number) => string;
		easing?: (t: number) => number;
	} {
		if (prefersReducedMotion() || animation === 'none') {
			return { delay: 0, duration: 0 };
		}

		// Faster exit animation, no stagger
		const exitDuration = Math.floor(effectiveDuration * 0.5);

		return {
			delay: 0,
			duration: exitDuration,
			easing: cubicOut,
			css: (t) => `opacity: ${t};`
		};
	}
</script>

<div class="fragment-container" class:show-all={showAll}>
	{#each fragmentsWithState as fragment (fragment.index)}
		{#if fragment.visible}
			<div
				class="fragment-item"
				class:fragment-visible={fragment.visible}
				data-fragment-index={fragment.index}
				in:getTransitionIn={{ index: fragment.index }}
				out:getTransitionOut={{ index: fragment.index }}
			>
				{@html fragment.content}
			</div>
		{/if}
	{/each}
</div>

<style>
	/* Show-all mode removes transitions and shows everything */
	.show-all .fragment-item {
		opacity: 1 !important;
		transform: none !important;
	}
</style>
