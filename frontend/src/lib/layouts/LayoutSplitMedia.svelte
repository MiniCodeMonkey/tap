<script lang="ts">
	/**
	 * LayoutSplitMedia - Image + text side-by-side layout.
	 *
	 * This layout is used for:
	 * - Showcasing an image alongside explanatory text
	 * - Product demonstrations with descriptions
	 * - Screenshots with annotations
	 * - Any slide where media and text should be equally prominent
	 *
	 * Expected HTML structure:
	 * - Content separated by ||| delimiter (parsed by transformer)
	 * - First portion: image or media content (left side)
	 * - Second portion: text content (right side)
	 * OR
	 * - First portion: text content (left side)
	 * - Second portion: image or media content (right side)
	 *
	 * Example markdown:
	 * ```
	 * ![Product screenshot](screenshot.png)
	 *
	 * |||
	 *
	 * ## New Dashboard
	 *
	 * - Real-time analytics
	 * - Customizable widgets
	 * - Dark mode support
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

<div class="layout-split-media">
	<div class="split-media-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-split-media {
		/* Fill parent container */
		width: 100%;
		height: 100%;

		/* Padding for content spacing */
		padding: var(--slide-padding, 4rem);
		box-sizing: border-box;

		/* Flex for content positioning */
		display: flex;
		flex-direction: column;
		justify-content: center;
	}

	.split-media-content {
		/* Full size content area */
		width: 100%;
		height: 100%;

		/* Two-column grid layout - equal columns */
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: var(--column-gap, 4rem);
		align-items: center;
	}

	/* Column containers */
	.split-media-content :global(.column) {
		display: flex;
		flex-direction: column;
		gap: var(--content-gap, 1.5rem);
		height: 100%;
		justify-content: center;
	}

	/* Image styling - fill the column appropriately */
	.split-media-content :global(img) {
		max-width: 100%;
		max-height: 100%;
		height: auto;
		object-fit: contain;
		border-radius: var(--image-border-radius, 0.75rem);
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
	}

	/* Heading styles */
	.split-media-content :global(h1) {
		font-size: var(--h1-font-size, 3.5rem);
		font-weight: 700;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.split-media-content :global(h2) {
		font-size: var(--h2-font-size, 2.75rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.split-media-content :global(h3) {
		font-size: var(--h3-font-size, 2rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.3;
		color: var(--heading-color, inherit);
	}

	/* Paragraph styles */
	.split-media-content :global(p) {
		font-size: var(--body-font-size, 1.75rem);
		margin: 0;
		line-height: 1.6;
		color: var(--text-color, inherit);
	}

	/* List styles */
	.split-media-content :global(ul),
	.split-media-content :global(ol) {
		font-size: var(--body-font-size, 1.75rem);
		margin: 0;
		padding-left: 1.5em;
		line-height: 1.6;
	}

	.split-media-content :global(li) {
		margin-bottom: 0.5em;
	}

	.split-media-content :global(li:last-child) {
		margin-bottom: 0;
	}

	/* Code styles */
	.split-media-content :global(pre) {
		font-size: var(--code-font-size, 1.25rem);
		margin: 0;
		padding: 1.5em;
		background: var(--code-bg, #1e1e1e);
		color: var(--code-color, #d4d4d4);
		border-radius: var(--code-border-radius, 0.5rem);
		overflow-x: auto;
	}

	.split-media-content :global(code) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	.split-media-content :global(p code),
	.split-media-content :global(li code) {
		font-size: 0.9em;
		padding: 0.2em 0.4em;
		background: var(--inline-code-bg, rgba(0, 0, 0, 0.1));
		border-radius: 0.25em;
	}

	/* Blockquote styles */
	.split-media-content :global(blockquote) {
		font-size: var(--body-font-size, 1.75rem);
		margin: 0;
		padding-left: 1em;
		border-left: 3px solid var(--accent-color, #7c3aed);
		color: var(--muted-color, #666);
		font-style: italic;
	}

	/* Video styling */
	.split-media-content :global(video) {
		max-width: 100%;
		max-height: 100%;
		border-radius: var(--image-border-radius, 0.75rem);
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
	}

	/* Theme-aware styling */
	:global(.theme-minimal) .split-media-content :global(h1),
	:global(.theme-minimal) .split-media-content :global(h2),
	:global(.theme-minimal) .split-media-content :global(h3) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
	}

	:global(.theme-minimal) .split-media-content :global(img) {
		box-shadow: 0 10px 40px rgba(0, 0, 0, 0.08);
	}

	:global(.theme-terminal) .split-media-content {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	:global(.theme-terminal) .split-media-content :global(h1),
	:global(.theme-terminal) .split-media-content :global(h2),
	:global(.theme-terminal) .split-media-content :global(h3) {
		text-shadow: 0 0 10px var(--accent-color, #00ff00);
	}

	:global(.theme-terminal) .split-media-content :global(img) {
		border: 2px solid var(--accent-color, #00ff00);
		box-shadow: 0 0 20px rgba(0, 255, 0, 0.2);
		border-radius: 0;
	}

	:global(.theme-terminal) .split-media-content :global(ul) {
		list-style: none;
		padding-left: 1em;
	}

	:global(.theme-terminal) .split-media-content :global(li)::before {
		content: '> ';
		color: var(--accent-color, #00ff00);
	}

	:global(.theme-gradient) .split-media-content :global(h1),
	:global(.theme-gradient) .split-media-content :global(h2) {
		background: var(--heading-gradient, linear-gradient(135deg, #7c3aed, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
	}

	:global(.theme-gradient) .split-media-content :global(img) {
		border-radius: 1rem;
		box-shadow: 0 20px 60px rgba(124, 58, 237, 0.2);
	}

	:global(.theme-brutalist) .split-media-content :global(h1),
	:global(.theme-brutalist) .split-media-content :global(h2),
	:global(.theme-brutalist) .split-media-content :global(h3) {
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	:global(.theme-brutalist) .split-media-content :global(img) {
		border-radius: 0;
		border: 6px solid var(--accent-color, #000);
		box-shadow: 12px 12px 0 0 var(--accent-color, #000);
	}

	:global(.theme-keynote) .split-media-content :global(h1),
	:global(.theme-keynote) .split-media-content :global(h2),
	:global(.theme-keynote) .split-media-content :global(h3) {
		font-weight: 500;
		text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	:global(.theme-keynote) .split-media-content :global(img) {
		box-shadow: 0 20px 60px rgba(0, 0, 0, 0.12);
		border-radius: 1rem;
	}
</style>
