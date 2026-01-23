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
	.progress-bar-container {
		/* Position at bottom of slide viewport */
		position: fixed;
		bottom: 0;
		left: 0;
		right: 0;
		z-index: 100;

		/* Subtle appearance */
		height: 3px;
		background-color: var(--progress-bg, rgba(0, 0, 0, 0.1));

		/* Smooth appearance */
		transition: opacity 0.3s ease;
	}

	.progress-bar-fill {
		height: 100%;
		background-color: var(--progress-fill, rgba(124, 58, 237, 0.6));

		/* Smooth width transitions */
		transition: width 0.3s ease-out;
	}

	/* Theme-specific overrides */
	:global(.theme-minimal) .progress-bar-container {
		--progress-bg: rgba(0, 0, 0, 0.05);
		--progress-fill: rgba(124, 58, 237, 0.5);
	}

	:global(.theme-terminal) .progress-bar-container {
		--progress-bg: rgba(0, 255, 0, 0.1);
		--progress-fill: rgba(0, 255, 0, 0.6);
	}

	:global(.theme-gradient) .progress-bar-container {
		--progress-bg: rgba(255, 255, 255, 0.1);
		--progress-fill: rgba(251, 191, 36, 0.7);
	}

	:global(.theme-brutalist) .progress-bar-container {
		--progress-bg: rgba(0, 0, 0, 0.1);
		--progress-fill: #ff0000;
		height: 4px;
	}

	:global(.theme-keynote) .progress-bar-container {
		--progress-bg: rgba(0, 0, 0, 0.05);
		--progress-fill: rgba(0, 122, 255, 0.6);
	}

	/* Reduced motion support */
	@media (prefers-reduced-motion: reduce) {
		.progress-bar-container,
		.progress-bar-fill {
			transition: none;
		}
	}
</style>
