<script lang="ts">
	import { onMount, onDestroy } from 'svelte';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** Aspect ratio in format "16:9", "4:3", or "16:10" */
		aspectRatio?: string;
		/** Theme name for CSS class */
		theme?: string;
		/** Whether fullscreen mode is active */
		fullscreen?: boolean;
		/** Content to render inside the slide */
		children?: import('svelte').Snippet;
	}

	let {
		aspectRatio = '16:9',
		theme = 'minimal',
		fullscreen = false,
		children
	}: Props = $props();

	// ============================================================================
	// State
	// ============================================================================

	let containerRef: HTMLDivElement | undefined = $state();
	let slideRef: HTMLDivElement | undefined = $state();
	let scale = $state(1);

	// ============================================================================
	// Computed Values
	// ============================================================================

	/**
	 * Parse aspect ratio string to numeric ratio.
	 */
	function parseAspectRatio(ratio: string): number {
		const [width, height] = ratio.split(':').map(Number);
		if (!width || !height || isNaN(width) || isNaN(height)) {
			// Default to 16:9
			return 16 / 9;
		}
		return width / height;
	}

	/**
	 * Get CSS aspect ratio value.
	 */
	function getCSSAspectRatio(ratio: string): string {
		const [width, height] = ratio.split(':').map(Number);
		if (!width || !height || isNaN(width) || isNaN(height)) {
			return '16 / 9';
		}
		return `${width} / ${height}`;
	}

	let numericRatio = $derived(parseAspectRatio(aspectRatio));
	let cssAspectRatio = $derived(getCSSAspectRatio(aspectRatio));

	// ============================================================================
	// Scaling Logic
	// ============================================================================

	/**
	 * Calculate the scale to fit the slide within the container.
	 */
	function calculateScale(): void {
		if (!containerRef || !slideRef) return;

		const containerRect = containerRef.getBoundingClientRect();
		const containerWidth = containerRect.width;
		const containerHeight = containerRect.height;

		if (containerWidth === 0 || containerHeight === 0) return;

		// Calculate the slide's natural dimensions based on aspect ratio
		// Use a base width of 1920px (Full HD) as the reference
		const baseWidth = 1920;
		const baseHeight = baseWidth / numericRatio;

		// Calculate scale to fit within container
		const scaleX = containerWidth / baseWidth;
		const scaleY = containerHeight / baseHeight;

		// Use the smaller scale to ensure the slide fits entirely
		scale = Math.min(scaleX, scaleY);
	}

	// ============================================================================
	// Lifecycle
	// ============================================================================

	let resizeObserver: ResizeObserver | undefined;

	onMount(() => {
		// Initial scale calculation
		calculateScale();

		// Set up ResizeObserver for container size changes
		if (containerRef) {
			resizeObserver = new ResizeObserver(() => {
				calculateScale();
			});
			resizeObserver.observe(containerRef);
		}

		// Handle window resize (for fullscreen changes)
		window.addEventListener('resize', calculateScale);
	});

	onDestroy(() => {
		if (resizeObserver) {
			resizeObserver.disconnect();
		}
		if (typeof window !== 'undefined') {
			window.removeEventListener('resize', calculateScale);
		}
	});

	// Recalculate scale when aspect ratio changes
	$effect(() => {
		// Access numericRatio to track as dependency
		const _ = numericRatio;
		void _;
		calculateScale();
	});

	// Recalculate scale when fullscreen changes
	$effect(() => {
		// Access fullscreen to track as dependency
		const isFullscreen = fullscreen;
		if (isFullscreen) {
			// Small delay to allow fullscreen transition
			setTimeout(calculateScale, 100);
		} else {
			calculateScale();
		}
	});
</script>

<div
	class="slide-container theme-{theme}"
	class:fullscreen
	bind:this={containerRef}
>
	<div
		class="slide"
		bind:this={slideRef}
		style:--aspect-ratio={cssAspectRatio}
		style:transform="scale({scale})"
	>
		{#if children}
			{@render children()}
		{/if}
	</div>
</div>

<style>
	/* Legacy CSS removed - will be replaced with Tailwind in US-105 */
	/* Animation-related styles preserved below */

	.slide-container {
		transition: background-color 0.3s ease;
	}

	.slide {
		transition: transform 0.2s ease-out;
	}

	/* Reduced motion support */
	@media (prefers-reduced-motion: reduce) {
		.slide-container {
			transition: none;
		}
		.slide {
			transition: none;
		}
	}
</style>
