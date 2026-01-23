<script lang="ts">
	/**
	 * LayoutBigStat - Large number emphasis layout for statistics.
	 *
	 * This layout is used for:
	 * - Highlighting key metrics or numbers
	 * - KPI displays
	 * - Impact statistics
	 * - Any slide where a number is the primary focus
	 *
	 * Expected HTML structure:
	 * - h1/strong: The big number/stat (required)
	 * - p: Description or context (optional)
	 *
	 * Example markdown:
	 * ```
	 * # 99.9%
	 *
	 * Uptime over the last 12 months
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

<div class="layout-big-stat">
	<div class="stat-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-big-stat {
		/* Fill parent container */
		width: 100%;
		height: 100%;

		/* Center content vertically and horizontally */
		display: flex;
		justify-content: center;
		align-items: center;

		/* Padding for content spacing */
		padding: var(--slide-padding, 4rem);
		box-sizing: border-box;
	}

	.stat-content {
		/* Center text within content area */
		text-align: center;

		/* Constrain width for readability */
		max-width: 90%;

		/* Flex for vertical content arrangement */
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--stat-gap, 1.5rem);
	}

	/* Big stat number styling - extra large and bold */
	.stat-content :global(h1),
	.stat-content :global(strong) {
		font-size: var(--stat-font-size, 10rem);
		font-weight: 800;
		margin: 0;
		line-height: 1;
		color: var(--stat-color, var(--accent-color, #7c3aed));
		letter-spacing: -0.02em;
	}

	/* Description text - contextual information below the stat */
	.stat-content :global(p) {
		font-size: var(--stat-description-font-size, 2.5rem);
		color: var(--muted-color, #666);
		margin: 0;
		line-height: 1.4;
		max-width: 70%;
	}

	/* Secondary stats or additional context */
	.stat-content :global(h2) {
		font-size: var(--stat-secondary-font-size, 4rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	/* Theme-aware styling */
	:global(.theme-minimal) .stat-content :global(h1),
	:global(.theme-minimal) .stat-content :global(strong) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
		color: var(--accent-color, #7c3aed);
	}

	:global(.theme-terminal) .stat-content :global(h1),
	:global(.theme-terminal) .stat-content :global(strong) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		color: var(--accent-color, #00ff00);
		text-shadow: 0 0 20px var(--accent-color, #00ff00), 0 0 40px var(--accent-color, #00ff00);
	}

	:global(.theme-terminal) .stat-content :global(p) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		color: var(--muted-color, #888);
	}

	:global(.theme-gradient) .stat-content :global(h1),
	:global(.theme-gradient) .stat-content :global(strong) {
		background: var(--stat-gradient, linear-gradient(135deg, #7c3aed, #ec4899, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
	}

	:global(.theme-brutalist) .stat-content :global(h1),
	:global(.theme-brutalist) .stat-content :global(strong) {
		font-weight: 900;
		color: var(--accent-color, #000);
		text-transform: uppercase;
	}

	:global(.theme-brutalist) .stat-content :global(p) {
		text-transform: uppercase;
		letter-spacing: 0.1em;
		font-weight: 600;
	}

	:global(.theme-keynote) .stat-content :global(h1),
	:global(.theme-keynote) .stat-content :global(strong) {
		font-weight: 700;
		color: var(--accent-color, #007aff);
		text-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
	}
</style>
