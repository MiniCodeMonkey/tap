<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { currentSlideIndex, totalSlides } from '$lib/stores/presentation';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** Whether to show the progress bar (can be disabled via config) */
		show?: boolean;
	}

	let { show = true }: Props = $props();

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
		class="progress-bar-container"
		role="progressbar"
		aria-valuenow={currentIndex + 1}
		aria-valuemin={1}
		aria-valuemax={total}
		aria-label="Presentation progress: slide {currentIndex + 1} of {total}"
	>
		<div class="progress-bar-fill" style:width="{progressPercent()}%"></div>
	</div>
{/if}
