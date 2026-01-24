<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { ThemeColors } from '$lib/types';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** Aspect ratio in format "16:9", "4:3", or "16:10" */
		aspectRatio?: string;
		/** Theme name for CSS class */
		theme?: string;
		/** Theme color overrides from frontmatter */
		themeColors?: ThemeColors;
		/** Whether fullscreen mode is active */
		fullscreen?: boolean;
		/** Content to render inside the slide */
		children?: import('svelte').Snippet;
	}

	let {
		aspectRatio = '16:9',
		theme = 'paper',
		themeColors,
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
	// Theme Color Overrides
	// ============================================================================

	/**
	 * Regex to validate CSS color values.
	 * Supports: hex (#RGB, #RRGGBB, #RGBA, #RRGGBBAA), rgb(), rgba(), hsl(), hsla(), oklch(), etc.
	 */
	const hexColorRegex = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/;
	const colorFunctionRegex = /^(rgb|rgba|hsl|hsla|oklch|oklab|lch|lab)\(/i;
	const namedColors = new Set([
		'black', 'white', 'red', 'green', 'blue', 'yellow', 'orange', 'purple',
		'pink', 'gray', 'grey', 'transparent', 'currentColor', 'inherit'
	]);

	/**
	 * Validate a CSS color value.
	 */
	function isValidColor(value: string): boolean {
		if (hexColorRegex.test(value)) return true;
		if (colorFunctionRegex.test(value)) return true;
		if (namedColors.has(value)) return true;
		return false;
	}

	/**
	 * Map themeColors keys to CSS custom property names.
	 */
	const colorKeyToProperty: Record<string, string> = {
		background: '--color-bg',
		text: '--color-text',
		muted: '--color-muted',
		accent: '--color-accent',
		codeBg: '--color-code-bg'
	};

	/**
	 * Generate inline style string for theme color overrides.
	 * Invalid colors are logged as warnings and skipped.
	 */
	let colorOverrideStyle = $derived.by(() => {
		if (!themeColors) return '';

		const styles: string[] = [];

		for (const [key, value] of Object.entries(themeColors)) {
			if (!value) continue;

			const cssProperty = colorKeyToProperty[key];
			if (!cssProperty) {
				console.warn(`[tap] Invalid themeColors key "${key}". Valid keys: ${Object.keys(colorKeyToProperty).join(', ')}`);
				continue;
			}

			if (!isValidColor(value)) {
				console.warn(`[tap] Invalid color value "${value}" for themeColors.${key}. Skipping.`);
				continue;
			}

			styles.push(`${cssProperty}: ${value}`);
		}

		return styles.join('; ');
	});

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

<!--
	SlideContainer uses Tailwind utilities for layout and theme CSS variables for colors.
	The slide is centered in the viewport using flex and scaled to fit within the container.
	Theme color overrides from frontmatter are applied as inline CSS custom properties.
-->
<div
	class="slide-container theme-{theme} w-full h-full flex items-center justify-center bg-theme-bg overflow-hidden transition-colors duration-slide ease-out {fullscreen ? 'fixed inset-0 z-50' : 'relative'}"
	style={colorOverrideStyle}
	bind:this={containerRef}
>
	<div
		class="slide w-[1920px] origin-center bg-theme-bg text-theme-text transition-transform duration-slide-fast ease-out motion-reduce:transition-none"
		bind:this={slideRef}
		style:aspect-ratio={cssAspectRatio}
		style:transform="scale({scale})"
	>
		{#if children}
			{@render children()}
		{/if}
	</div>
</div>
