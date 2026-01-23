<script lang="ts">
	/**
	 * LayoutBlank - Empty canvas layout.
	 *
	 * This layout is used for:
	 * - Custom designs that don't fit standard layouts
	 * - Embedded iframes or custom HTML
	 * - Full creative freedom slides
	 * - Animations or interactive content
	 *
	 * Expected HTML structure:
	 * - Any content - rendered as-is without additional styling
	 * - Full slide area available for custom positioning
	 *
	 * Example markdown:
	 * ```
	 * <!--
	 * layout: blank
	 * -->
	 *
	 * <div style="position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%);">
	 *   Custom positioned content
	 * </div>
	 * ```
	 */

	import type { Snippet } from 'svelte';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** Slot content containing the slide HTML */
		children: Snippet;
	}

	let { children }: Props = $props();
</script>

<div class="layout-blank">
	<div class="blank-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-blank {
		/* Fill parent container */
		width: 100%;
		height: 100%;

		/* Position relative for absolute positioning of children */
		position: relative;

		/* No padding - full canvas */
		padding: 0;
		margin: 0;
	}

	.blank-content {
		/* Fill parent completely */
		width: 100%;
		height: 100%;

		/* Position relative for absolute children */
		position: relative;

		/* Minimal base styles - let content define everything */
		display: block;
	}

	/* Minimal heading styles - only basic resets */
	.blank-content :global(h1),
	.blank-content :global(h2),
	.blank-content :global(h3),
	.blank-content :global(h4),
	.blank-content :global(h5),
	.blank-content :global(h6) {
		margin: 0;
	}

	/* Minimal paragraph styles */
	.blank-content :global(p) {
		margin: 0;
	}

	/* Minimal list styles */
	.blank-content :global(ul),
	.blank-content :global(ol) {
		margin: 0;
	}

	/* Image resets */
	.blank-content :global(img) {
		max-width: 100%;
		height: auto;
	}

	/* Code block base styling */
	.blank-content :global(pre) {
		margin: 0;
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	.blank-content :global(code) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	/* Allow full-bleed elements */
	.blank-content :global(.full-bleed) {
		position: absolute;
		inset: 0;
	}

	/* Centering utility class */
	.blank-content :global(.centered) {
		position: absolute;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
	}

	/* Iframe support for embedded content */
	.blank-content :global(iframe) {
		border: none;
		max-width: 100%;
		max-height: 100%;
	}

	/* Fullscreen iframe */
	.blank-content :global(iframe.fullscreen) {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
	}

	/* Theme-aware styling - minimal overrides */
	:global(.theme-terminal) .blank-content {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	:global(.theme-terminal) .blank-content :global(h1),
	:global(.theme-terminal) .blank-content :global(h2),
	:global(.theme-terminal) .blank-content :global(h3) {
		text-shadow: 0 0 10px var(--accent-color, #00ff00);
	}

	:global(.theme-brutalist) .blank-content :global(h1),
	:global(.theme-brutalist) .blank-content :global(h2),
	:global(.theme-brutalist) .blank-content :global(h3) {
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
</style>
