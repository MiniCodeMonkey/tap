<script lang="ts">
	/**
	 * LayoutDefault - Standard content layout for general-purpose slides.
	 *
	 * This layout is used for:
	 * - Regular content slides with headings and body text
	 * - Bullet point lists
	 * - Mixed content with images and text
	 * - Any slide that doesn't fit a more specific layout
	 *
	 * Expected HTML structure:
	 * - h1, h2, h3: Headings (optional)
	 * - p: Paragraphs
	 * - ul, ol: Lists
	 * - img: Images
	 * - pre, code: Code blocks
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

<div class="layout-default">
	<div class="default-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-default {
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

	.default-content {
		/* Full width content area */
		width: 100%;
		max-width: 100%;

		/* Flex for vertical content arrangement */
		display: flex;
		flex-direction: column;
		gap: var(--content-gap, 1.5rem);
	}

	/* Heading styles */
	.default-content :global(h1) {
		font-size: var(--h1-font-size, 4rem);
		font-weight: 700;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.default-content :global(h2) {
		font-size: var(--h2-font-size, 3rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.default-content :global(h3) {
		font-size: var(--h3-font-size, 2.25rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.3;
		color: var(--heading-color, inherit);
	}

	/* Paragraph styles */
	.default-content :global(p) {
		font-size: var(--body-font-size, 2rem);
		margin: 0;
		line-height: 1.6;
		color: var(--text-color, inherit);
	}

	/* List styles */
	.default-content :global(ul),
	.default-content :global(ol) {
		font-size: var(--body-font-size, 2rem);
		margin: 0;
		padding-left: 2em;
		line-height: 1.6;
	}

	.default-content :global(li) {
		margin-bottom: 0.5em;
	}

	.default-content :global(li:last-child) {
		margin-bottom: 0;
	}

	/* Image styles */
	.default-content :global(img) {
		max-width: 100%;
		height: auto;
		border-radius: var(--image-border-radius, 0.5rem);
	}

	/* Code styles */
	.default-content :global(pre) {
		font-size: var(--code-font-size, 1.5rem);
		margin: 0;
		padding: 1.5em;
		background: var(--code-bg, #1e1e1e);
		color: var(--code-color, #d4d4d4);
		border-radius: var(--code-border-radius, 0.5rem);
		overflow-x: auto;
	}

	.default-content :global(code) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	.default-content :global(p code),
	.default-content :global(li code) {
		font-size: 0.9em;
		padding: 0.2em 0.4em;
		background: var(--inline-code-bg, rgba(0, 0, 0, 0.1));
		border-radius: 0.25em;
	}

	/* Blockquote styles */
	.default-content :global(blockquote) {
		font-size: var(--body-font-size, 2rem);
		margin: 0;
		padding-left: 1.5em;
		border-left: 4px solid var(--accent-color, #7c3aed);
		color: var(--muted-color, #666);
		font-style: italic;
	}

	/* Table styles */
	.default-content :global(table) {
		width: 100%;
		border-collapse: collapse;
		font-size: var(--table-font-size, 1.5rem);
	}

	.default-content :global(th),
	.default-content :global(td) {
		padding: 0.75em 1em;
		text-align: left;
		border-bottom: 1px solid var(--border-color, rgba(0, 0, 0, 0.1));
	}

	.default-content :global(th) {
		font-weight: 600;
		background: var(--table-header-bg, rgba(0, 0, 0, 0.05));
	}

	/* Theme-aware styling */
	:global(.theme-minimal) .default-content :global(h1),
	:global(.theme-minimal) .default-content :global(h2),
	:global(.theme-minimal) .default-content :global(h3) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
	}

	:global(.theme-terminal) .default-content {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	:global(.theme-terminal) .default-content :global(h1),
	:global(.theme-terminal) .default-content :global(h2),
	:global(.theme-terminal) .default-content :global(h3) {
		text-shadow: 0 0 10px var(--accent-color, #00ff00);
	}

	:global(.theme-terminal) .default-content :global(ul) {
		list-style: none;
		padding-left: 1em;
	}

	:global(.theme-terminal) .default-content :global(li)::before {
		content: '> ';
		color: var(--accent-color, #00ff00);
	}

	:global(.theme-gradient) .default-content :global(h1),
	:global(.theme-gradient) .default-content :global(h2) {
		background: var(--heading-gradient, linear-gradient(135deg, #7c3aed, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
	}

	:global(.theme-brutalist) .default-content :global(h1),
	:global(.theme-brutalist) .default-content :global(h2),
	:global(.theme-brutalist) .default-content :global(h3) {
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	:global(.theme-keynote) .default-content :global(h1),
	:global(.theme-keynote) .default-content :global(h2),
	:global(.theme-keynote) .default-content :global(h3) {
		font-weight: 500;
		text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}
</style>
