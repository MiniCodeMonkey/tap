<script lang="ts">
	/**
	 * LayoutTwoColumn - Side-by-side two-column layout.
	 *
	 * This layout is used for:
	 * - Comparing two concepts side by side
	 * - Image and text combinations
	 * - Before/after comparisons
	 * - Any slide where content should be split into two columns
	 *
	 * Expected HTML structure:
	 * - Content separated by ||| delimiter (parsed by transformer)
	 * - First portion goes in left column
	 * - Second portion goes in right column
	 *
	 * Example markdown:
	 * ```
	 * ## Left Column Title
	 * - Point 1
	 * - Point 2
	 *
	 * |||
	 *
	 * ## Right Column Title
	 * - Point A
	 * - Point B
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

<div class="layout-two-column">
	<div class="two-column-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-two-column {
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

	.two-column-content {
		/* Full size content area */
		width: 100%;
		height: 100%;

		/* Two-column grid layout */
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: var(--column-gap, 4rem);
		align-items: start;
	}

	/* Column dividers - target the direct children representing columns */
	.two-column-content :global(.column) {
		display: flex;
		flex-direction: column;
		gap: var(--content-gap, 1.5rem);
	}

	/* Heading styles within columns */
	.two-column-content :global(h1) {
		font-size: var(--h1-font-size, 3.5rem);
		font-weight: 700;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.two-column-content :global(h2) {
		font-size: var(--h2-font-size, 2.5rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.two-column-content :global(h3) {
		font-size: var(--h3-font-size, 2rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.3;
		color: var(--heading-color, inherit);
	}

	/* Paragraph styles */
	.two-column-content :global(p) {
		font-size: var(--body-font-size, 1.75rem);
		margin: 0;
		line-height: 1.6;
		color: var(--text-color, inherit);
	}

	/* List styles */
	.two-column-content :global(ul),
	.two-column-content :global(ol) {
		font-size: var(--body-font-size, 1.75rem);
		margin: 0;
		padding-left: 1.5em;
		line-height: 1.6;
	}

	.two-column-content :global(li) {
		margin-bottom: 0.5em;
	}

	.two-column-content :global(li:last-child) {
		margin-bottom: 0;
	}

	/* Image styles */
	.two-column-content :global(img) {
		max-width: 100%;
		height: auto;
		border-radius: var(--image-border-radius, 0.5rem);
	}

	/* Code styles */
	.two-column-content :global(pre) {
		font-size: var(--code-font-size, 1.25rem);
		margin: 0;
		padding: 1.25em;
		background: var(--code-bg, #1e1e1e);
		color: var(--code-color, #d4d4d4);
		border-radius: var(--code-border-radius, 0.5rem);
		overflow-x: auto;
	}

	.two-column-content :global(code) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	.two-column-content :global(p code),
	.two-column-content :global(li code) {
		font-size: 0.9em;
		padding: 0.2em 0.4em;
		background: var(--inline-code-bg, rgba(0, 0, 0, 0.1));
		border-radius: 0.25em;
	}

	/* Blockquote styles */
	.two-column-content :global(blockquote) {
		font-size: var(--body-font-size, 1.75rem);
		margin: 0;
		padding-left: 1.25em;
		border-left: 3px solid var(--accent-color, #7c3aed);
		color: var(--muted-color, #666);
		font-style: italic;
	}

	/* Table styles */
	.two-column-content :global(table) {
		width: 100%;
		border-collapse: collapse;
		font-size: var(--table-font-size, 1.25rem);
	}

	.two-column-content :global(th),
	.two-column-content :global(td) {
		padding: 0.5em 0.75em;
		text-align: left;
		border-bottom: 1px solid var(--border-color, rgba(0, 0, 0, 0.1));
	}

	.two-column-content :global(th) {
		font-weight: 600;
		background: var(--table-header-bg, rgba(0, 0, 0, 0.05));
	}

	/* Theme-aware styling */
	:global(.theme-minimal) .two-column-content :global(h1),
	:global(.theme-minimal) .two-column-content :global(h2),
	:global(.theme-minimal) .two-column-content :global(h3) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
	}

	:global(.theme-terminal) .two-column-content {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	:global(.theme-terminal) .two-column-content :global(h1),
	:global(.theme-terminal) .two-column-content :global(h2),
	:global(.theme-terminal) .two-column-content :global(h3) {
		text-shadow: 0 0 10px var(--accent-color, #00ff00);
	}

	:global(.theme-terminal) .two-column-content :global(ul) {
		list-style: none;
		padding-left: 1em;
	}

	:global(.theme-terminal) .two-column-content :global(li)::before {
		content: '> ';
		color: var(--accent-color, #00ff00);
	}

	:global(.theme-gradient) .two-column-content :global(h1),
	:global(.theme-gradient) .two-column-content :global(h2) {
		background: var(--heading-gradient, linear-gradient(135deg, #7c3aed, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
	}

	:global(.theme-brutalist) .two-column-content :global(h1),
	:global(.theme-brutalist) .two-column-content :global(h2),
	:global(.theme-brutalist) .two-column-content :global(h3) {
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	:global(.theme-brutalist) .two-column-content {
		/* Add a vertical divider between columns in brutalist theme */
		column-gap: var(--column-gap, 4rem);
		position: relative;
	}

	:global(.theme-brutalist) .two-column-content::after {
		content: '';
		position: absolute;
		top: 0;
		bottom: 0;
		left: 50%;
		width: 4px;
		background: var(--accent-color, #000);
		transform: translateX(-50%);
	}

	:global(.theme-keynote) .two-column-content :global(h1),
	:global(.theme-keynote) .two-column-content :global(h2),
	:global(.theme-keynote) .two-column-content :global(h3) {
		font-weight: 500;
		text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}
</style>
