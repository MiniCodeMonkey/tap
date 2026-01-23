<script lang="ts">
	/**
	 * LayoutSection - Large section header for dividing presentation sections.
	 *
	 * This layout is used for:
	 * - Section dividers between major presentation topics
	 * - Chapter headers
	 * - Any slide with a single prominent H2 heading
	 *
	 * Expected HTML structure:
	 * - h2: Section title (required)
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

<div class="layout-section">
	<div class="section-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-section {
		/* Fill parent container */
		width: 100%;
		height: 100%;

		/* Center content vertically and horizontally */
		display: flex;
		justify-content: center;
		align-items: center;
	}

	.section-content {
		/* Center text within content area */
		text-align: center;

		/* Constrain width for readability */
		max-width: 90%;

		/* Flex for vertical content arrangement */
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--section-gap, 0.5em);
	}

	/* Section header styling - large, prominent heading */
	.section-content :global(h2) {
		font-size: var(--section-font-size, 4rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.2;
		color: var(--section-color, inherit);
	}

	/* Support for optional section subtitle/description */
	.section-content :global(p) {
		font-size: var(--section-subtitle-size, 2rem);
		color: var(--muted-color, #666);
		margin: 0;
		line-height: 1.4;
	}

	/* Theme-aware styling via CSS custom properties */
	:global(.theme-minimal) .section-content :global(h2) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
	}

	:global(.theme-terminal) .section-content :global(h2) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		text-shadow: 0 0 10px var(--accent-color, #00ff00);
	}

	:global(.theme-terminal) .section-content :global(h2)::before {
		content: '# ';
		opacity: 0.5;
	}

	:global(.theme-gradient) .section-content :global(h2) {
		background: var(--section-gradient, linear-gradient(135deg, #7c3aed, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
	}

	:global(.theme-brutalist) .section-content :global(h2) {
		text-transform: uppercase;
		letter-spacing: 0.05em;
		border-bottom: 4px solid var(--accent-color, #000);
		padding-bottom: 0.25em;
	}

	:global(.theme-keynote) .section-content :global(h2) {
		font-weight: 500;
		text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}
</style>
