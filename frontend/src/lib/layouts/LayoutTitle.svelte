<script lang="ts">
	/**
	 * LayoutTitle - Centered title slide with optional subtitle.
	 *
	 * This layout is used for:
	 * - Title slides at the beginning of presentations
	 * - Section dividers with prominent headings
	 * - Any slide where you want a centered, bold title
	 *
	 * Expected HTML structure:
	 * - h1: Main title (required)
	 * - p: Subtitle (optional)
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

<div class="layout-title">
	<div class="title-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-title {
		/* Fill parent container */
		width: 100%;
		height: 100%;

		/* Center content vertically and horizontally */
		display: flex;
		justify-content: center;
		align-items: center;
	}

	.title-content {
		/* Center text within content area */
		text-align: center;

		/* Constrain width for readability */
		max-width: 90%;

		/* Flex for vertical content arrangement */
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--title-gap, 0.5em);
	}

	/* Title styling - large, bold heading */
	.title-content :global(h1) {
		font-size: var(--title-font-size, 6rem);
		font-weight: 700;
		margin: 0;
		line-height: 1.1;
		color: var(--title-color, inherit);
	}

	/* Subtitle styling - smaller, muted text */
	.title-content :global(p) {
		font-size: var(--subtitle-font-size, 2.5rem);
		color: var(--muted-color, #666);
		margin: 0;
		line-height: 1.4;
	}

	/* Support for author/date metadata below subtitle */
	.title-content :global(p:last-child:not(:first-child)) {
		font-size: var(--meta-font-size, 1.5rem);
		color: var(--muted-color, #666);
		margin-top: 1em;
	}

	/* Theme-aware styling via CSS custom properties */
	:global(.theme-minimal) .title-content :global(h1) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
	}

	:global(.theme-terminal) .title-content :global(h1) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		text-shadow: 0 0 10px var(--accent-color, #00ff00);
	}

	:global(.theme-gradient) .title-content :global(h1) {
		background: var(--title-gradient, linear-gradient(135deg, #7c3aed, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
	}

	:global(.theme-brutalist) .title-content :global(h1) {
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
</style>
