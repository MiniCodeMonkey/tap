<script lang="ts">
	/**
	 * LayoutCover - Full-bleed background image layout.
	 *
	 * This layout is used for:
	 * - Dramatic opening slides
	 * - Section transitions with visual impact
	 * - Image-focused slides with overlay text
	 * - Chapter breaks in longer presentations
	 *
	 * Expected HTML structure:
	 * - h1: Main title overlay (optional)
	 * - p: Subtitle or description (optional)
	 * - Background image set via slide directives
	 *
	 * Example markdown:
	 * ```
	 * <!--
	 * layout: cover
	 * background: image.jpg
	 * -->
	 *
	 * # The Future of Technology
	 *
	 * Where we go from here
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

<div class="layout-cover">
	<div class="cover-overlay"></div>
	<div class="cover-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-cover {
		/* Fill parent container */
		width: 100%;
		height: 100%;

		/* Position for overlay */
		position: relative;

		/* Center content vertically and horizontally */
		display: flex;
		justify-content: center;
		align-items: center;
	}

	/* Dark overlay for text readability */
	.cover-overlay {
		position: absolute;
		inset: 0;
		background: var(--cover-overlay, linear-gradient(to bottom, rgba(0, 0, 0, 0.3), rgba(0, 0, 0, 0.6)));
		pointer-events: none;
	}

	.cover-content {
		/* Position above overlay */
		position: relative;
		z-index: 1;

		/* Center text within content area */
		text-align: center;

		/* Constrain width for readability */
		max-width: 85%;

		/* Flex for vertical content arrangement */
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--cover-gap, 1rem);

		/* Padding for content spacing */
		padding: var(--slide-padding, 4rem);
	}

	/* Title styling - large, bold, white for contrast */
	.cover-content :global(h1) {
		font-size: var(--cover-title-font-size, 6rem);
		font-weight: 700;
		margin: 0;
		line-height: 1.1;
		color: var(--cover-text-color, #fff);
		text-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
	}

	/* Subtitle styling */
	.cover-content :global(p) {
		font-size: var(--cover-subtitle-font-size, 2.5rem);
		color: var(--cover-text-color, rgba(255, 255, 255, 0.9));
		margin: 0;
		line-height: 1.4;
		text-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
	}

	/* H2 for secondary headings */
	.cover-content :global(h2) {
		font-size: var(--cover-h2-font-size, 3.5rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.2;
		color: var(--cover-text-color, #fff);
		text-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
	}

	/* Theme-aware styling */
	:global(.theme-minimal) .cover-content :global(h1),
	:global(.theme-minimal) .cover-content :global(h2) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
	}

	:global(.theme-minimal) .cover-overlay {
		background: linear-gradient(to bottom, rgba(0, 0, 0, 0.2), rgba(0, 0, 0, 0.5));
	}

	:global(.theme-terminal) .cover-content :global(h1),
	:global(.theme-terminal) .cover-content :global(h2) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		color: var(--accent-color, #00ff00);
		text-shadow: 0 0 20px var(--accent-color, #00ff00), 0 0 40px var(--accent-color, #00ff00);
	}

	:global(.theme-terminal) .cover-content :global(p) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		color: var(--cover-text-color, rgba(0, 255, 0, 0.8));
	}

	:global(.theme-terminal) .cover-overlay {
		background: linear-gradient(to bottom, rgba(0, 0, 0, 0.5), rgba(0, 0, 0, 0.8));
	}

	:global(.theme-gradient) .cover-content :global(h1) {
		background: var(--cover-gradient, linear-gradient(135deg, #fff, rgba(255, 255, 255, 0.8)));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
		text-shadow: none;
	}

	:global(.theme-gradient) .cover-overlay {
		background: var(--cover-overlay-gradient, linear-gradient(135deg, rgba(124, 58, 237, 0.6), rgba(6, 182, 212, 0.6)));
	}

	:global(.theme-brutalist) .cover-content :global(h1),
	:global(.theme-brutalist) .cover-content :global(h2) {
		text-transform: uppercase;
		letter-spacing: 0.1em;
		text-shadow: none;
		background: var(--cover-text-bg, rgba(255, 255, 255, 0.9));
		color: var(--accent-color, #000);
		padding: 0.25em 0.5em;
	}

	:global(.theme-brutalist) .cover-overlay {
		background: none;
	}

	:global(.theme-keynote) .cover-content :global(h1),
	:global(.theme-keynote) .cover-content :global(h2) {
		font-weight: 500;
		text-shadow: 0 4px 16px rgba(0, 0, 0, 0.5);
	}

	:global(.theme-keynote) .cover-overlay {
		background: linear-gradient(to bottom, rgba(0, 0, 0, 0.1), rgba(0, 0, 0, 0.5));
	}
</style>
