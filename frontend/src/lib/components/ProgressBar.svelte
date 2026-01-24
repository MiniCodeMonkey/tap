<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { currentSlideIndex, totalSlides } from '$lib/stores/presentation';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** Theme name for styling */
		theme?: string;
		/** Whether to show the progress bar (can be disabled via config) */
		show?: boolean;
	}

	let { theme = 'minimal', show = true }: Props = $props();

	// ============================================================================
	// State
	// ============================================================================

	let currentIndex = $state(0);
	let total = $state(0);

	// ============================================================================
	// Store Subscriptions
	// ============================================================================

	let unsubscribers: (() => void)[] = [];

	onMount(() => {
		unsubscribers.push(
			currentSlideIndex.subscribe((value) => {
				currentIndex = value;
			})
		);

		unsubscribers.push(
			totalSlides.subscribe((value) => {
				total = value;
			})
		);
	});

	onDestroy(() => {
		unsubscribers.forEach((unsub) => unsub());
	});

	// ============================================================================
	// Computed Values
	// ============================================================================

	/**
	 * Calculate progress percentage based on current slide.
	 * Uses (currentIndex + 1) / total so that first slide shows some progress
	 * and last slide shows 100%.
	 */
	let progressPercent = $derived(() => {
		if (total <= 0) return 0;
		// On first slide (index 0), show 1/total progress
		// On last slide (index total-1), show 100%
		return ((currentIndex + 1) / total) * 100;
	});
</script>

{#if show && total > 0}
	<div
		class="progress-bar-container theme-{theme}"
		role="progressbar"
		aria-valuenow={currentIndex + 1}
		aria-valuemin={1}
		aria-valuemax={total}
		aria-label="Presentation progress: slide {currentIndex + 1} of {total}"
	>
		<div class="progress-bar-fill" style:width="{progressPercent()}%"></div>
	</div>
{/if}

<style>
	/* Legacy CSS removed - will be replaced with Tailwind in US-108 */
	/* Animation-related styles preserved below */

	.progress-bar-container {
		transition: opacity 0.3s ease;
	}

	.progress-bar-fill {
		transition: width 0.3s ease-out;
	}

	/* Reduced motion support */
	@media (prefers-reduced-motion: reduce) {
		.progress-bar-container,
		.progress-bar-fill {
			transition: none;
		}
	}
</style>
