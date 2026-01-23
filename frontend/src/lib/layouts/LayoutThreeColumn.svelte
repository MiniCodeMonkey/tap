<script lang="ts">
	/**
	 * LayoutThreeColumn - Three-column grid layout.
	 *
	 * This layout is used for:
	 * - Comparing three items or options
	 * - Feature comparisons
	 * - Process steps (1, 2, 3)
	 * - Any slide where content should be split into three columns
	 *
	 * Expected HTML structure:
	 * - Content separated by ||| delimiters (parsed by transformer)
	 * - First portion goes in left column
	 * - Second portion goes in middle column
	 * - Third portion goes in right column
	 *
	 * Example markdown:
	 * ```
	 * ## Step 1
	 * Research
	 *
	 * |||
	 *
	 * ## Step 2
	 * Design
	 *
	 * |||
	 *
	 * ## Step 3
	 * Build
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

<div class="layout-three-column">
	<div class="three-column-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-three-column {
		/* Fill parent container */
		width: 100%;
		height: 100%;

		/* Padding for content spacing */
		padding: var(--slide-padding, 4rem);
		box-sizing: border-box;

		/* Flex for content positioning */
		display: flex;
		flex-direction: column;
		justify-content: flex-start;
	}

	.three-column-content {
		/* Full size content area */
		width: 100%;
		height: 100%;

		/* Three-column grid layout */
		display: grid;
		grid-template-columns: 1fr 1fr 1fr;
		gap: var(--column-gap, 3rem);
		align-items: start;
	}

	/* Column containers */
	.three-column-content :global(.column) {
		display: flex;
		flex-direction: column;
		gap: var(--content-gap, 1.25rem);
	}

	/* Heading styles - smaller for three columns */
	.three-column-content :global(h1) {
		font-size: var(--h1-font-size, 2.5rem);
		font-weight: 700;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.three-column-content :global(h2) {
		font-size: var(--h2-font-size, 2rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.three-column-content :global(h3) {
		font-size: var(--h3-font-size, 1.5rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.3;
		color: var(--heading-color, inherit);
	}

	/* Paragraph styles */
	.three-column-content :global(p) {
		font-size: var(--body-font-size, 1.4rem);
		margin: 0;
		line-height: 1.5;
		color: var(--text-color, inherit);
	}

	/* List styles */
	.three-column-content :global(ul),
	.three-column-content :global(ol) {
		font-size: var(--body-font-size, 1.4rem);
		margin: 0;
		padding-left: 1.25em;
		line-height: 1.5;
	}

	.three-column-content :global(li) {
		margin-bottom: 0.4em;
	}

	.three-column-content :global(li:last-child) {
		margin-bottom: 0;
	}

	/* Image styles */
	.three-column-content :global(img) {
		max-width: 100%;
		height: auto;
		border-radius: var(--image-border-radius, 0.5rem);
	}

	/* Code styles */
	.three-column-content :global(pre) {
		font-size: var(--code-font-size, 1rem);
		margin: 0;
		padding: 1em;
		background: var(--code-bg, #1e1e1e);
		color: var(--code-color, #d4d4d4);
		border-radius: var(--code-border-radius, 0.5rem);
		overflow-x: auto;
	}

	.three-column-content :global(code) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	.three-column-content :global(p code),
	.three-column-content :global(li code) {
		font-size: 0.9em;
		padding: 0.2em 0.4em;
		background: var(--inline-code-bg, rgba(0, 0, 0, 0.1));
		border-radius: 0.25em;
	}

	/* Blockquote styles */
	.three-column-content :global(blockquote) {
		font-size: var(--body-font-size, 1.4rem);
		margin: 0;
		padding-left: 1em;
		border-left: 3px solid var(--accent-color, #7c3aed);
		color: var(--muted-color, #666);
		font-style: italic;
	}

	/* Theme-aware styling */
	:global(.theme-minimal) .three-column-content :global(h1),
	:global(.theme-minimal) .three-column-content :global(h2),
	:global(.theme-minimal) .three-column-content :global(h3) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
	}

	:global(.theme-terminal) .three-column-content {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	:global(.theme-terminal) .three-column-content :global(h1),
	:global(.theme-terminal) .three-column-content :global(h2),
	:global(.theme-terminal) .three-column-content :global(h3) {
		text-shadow: 0 0 10px var(--accent-color, #00ff00);
	}

	:global(.theme-terminal) .three-column-content :global(ul) {
		list-style: none;
		padding-left: 1em;
	}

	:global(.theme-terminal) .three-column-content :global(li)::before {
		content: '> ';
		color: var(--accent-color, #00ff00);
	}

	:global(.theme-gradient) .three-column-content :global(h1),
	:global(.theme-gradient) .three-column-content :global(h2) {
		background: var(--heading-gradient, linear-gradient(135deg, #7c3aed, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
	}

	:global(.theme-brutalist) .three-column-content :global(h1),
	:global(.theme-brutalist) .three-column-content :global(h2),
	:global(.theme-brutalist) .three-column-content :global(h3) {
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	/* Add vertical dividers between columns in brutalist theme */
	:global(.theme-brutalist) .three-column-content {
		position: relative;
	}

	:global(.theme-brutalist) .three-column-content :global(.column:not(:last-child))::after {
		content: '';
		position: absolute;
		top: 0;
		bottom: 0;
		right: calc(-1 * var(--column-gap, 3rem) / 2);
		width: 4px;
		background: var(--accent-color, #000);
	}

	:global(.theme-keynote) .three-column-content :global(h1),
	:global(.theme-keynote) .three-column-content :global(h2),
	:global(.theme-keynote) .three-column-content :global(h3) {
		font-weight: 500;
		text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}
</style>
