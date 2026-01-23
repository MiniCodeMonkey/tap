<script lang="ts">
	/**
	 * LayoutSidebar - Main content with sidebar layout.
	 *
	 * This layout is used for:
	 * - Content with supporting information in a sidebar
	 * - Main topic with related links, notes, or context
	 * - Slides where secondary information complements the main content
	 *
	 * Expected HTML structure:
	 * - Content separated by ||| delimiter (parsed by transformer)
	 * - First portion goes in main content area (larger, left side)
	 * - Second portion goes in sidebar (smaller, right side)
	 *
	 * Example markdown:
	 * ```
	 * ## Main Topic
	 *
	 * This is the main content that takes up most of the slide.
	 * It can contain multiple paragraphs, lists, and other content.
	 *
	 * |||
	 *
	 * ### Related
	 * - Link 1
	 * - Link 2
	 * - Note
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

<div class="layout-sidebar">
	<div class="sidebar-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-sidebar {
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

	.sidebar-content {
		/* Full size content area */
		width: 100%;
		height: 100%;

		/* Sidebar grid layout - main content 2/3, sidebar 1/3 */
		display: grid;
		grid-template-columns: 2fr 1fr;
		gap: var(--column-gap, 3rem);
		align-items: start;
	}

	/* Main content area (first column) */
	.sidebar-content :global(.column:first-child),
	.sidebar-content :global(> *:first-child) {
		display: flex;
		flex-direction: column;
		gap: var(--content-gap, 1.5rem);
	}

	/* Sidebar area (second column) */
	.sidebar-content :global(.column:last-child),
	.sidebar-content :global(> *:last-child) {
		display: flex;
		flex-direction: column;
		gap: var(--content-gap, 1rem);
		padding: 1.5rem;
		background: var(--sidebar-bg, rgba(0, 0, 0, 0.03));
		border-radius: var(--sidebar-border-radius, 0.75rem);
		border-left: 3px solid var(--accent-color, #7c3aed);
	}

	/* Main content heading styles */
	.sidebar-content :global(h1) {
		font-size: var(--h1-font-size, 3.5rem);
		font-weight: 700;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.sidebar-content :global(h2) {
		font-size: var(--h2-font-size, 2.5rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	/* Sidebar heading - smaller */
	.sidebar-content :global(.column:last-child h3),
	.sidebar-content :global(> *:last-child h3) {
		font-size: var(--sidebar-h3-font-size, 1.5rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.3;
		color: var(--heading-color, inherit);
	}

	.sidebar-content :global(h3) {
		font-size: var(--h3-font-size, 2rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.3;
		color: var(--heading-color, inherit);
	}

	/* Paragraph styles - main content */
	.sidebar-content :global(p) {
		font-size: var(--body-font-size, 1.75rem);
		margin: 0;
		line-height: 1.6;
		color: var(--text-color, inherit);
	}

	/* Sidebar paragraph - smaller */
	.sidebar-content :global(.column:last-child p),
	.sidebar-content :global(> *:last-child p) {
		font-size: var(--sidebar-body-font-size, 1.25rem);
		line-height: 1.5;
	}

	/* List styles - main content */
	.sidebar-content :global(ul),
	.sidebar-content :global(ol) {
		font-size: var(--body-font-size, 1.75rem);
		margin: 0;
		padding-left: 1.5em;
		line-height: 1.6;
	}

	.sidebar-content :global(li) {
		margin-bottom: 0.5em;
	}

	.sidebar-content :global(li:last-child) {
		margin-bottom: 0;
	}

	/* Sidebar list - smaller */
	.sidebar-content :global(.column:last-child ul),
	.sidebar-content :global(.column:last-child ol),
	.sidebar-content :global(> *:last-child ul),
	.sidebar-content :global(> *:last-child ol) {
		font-size: var(--sidebar-body-font-size, 1.25rem);
		padding-left: 1.25em;
		line-height: 1.5;
	}

	.sidebar-content :global(.column:last-child li),
	.sidebar-content :global(> *:last-child li) {
		margin-bottom: 0.4em;
	}

	/* Image styles */
	.sidebar-content :global(img) {
		max-width: 100%;
		height: auto;
		border-radius: var(--image-border-radius, 0.5rem);
	}

	/* Code styles */
	.sidebar-content :global(pre) {
		font-size: var(--code-font-size, 1.25rem);
		margin: 0;
		padding: 1.5em;
		background: var(--code-bg, #1e1e1e);
		color: var(--code-color, #d4d4d4);
		border-radius: var(--code-border-radius, 0.5rem);
		overflow-x: auto;
	}

	.sidebar-content :global(code) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	.sidebar-content :global(p code),
	.sidebar-content :global(li code) {
		font-size: 0.9em;
		padding: 0.2em 0.4em;
		background: var(--inline-code-bg, rgba(0, 0, 0, 0.1));
		border-radius: 0.25em;
	}

	/* Blockquote styles */
	.sidebar-content :global(blockquote) {
		font-size: var(--body-font-size, 1.75rem);
		margin: 0;
		padding-left: 1em;
		border-left: 3px solid var(--accent-color, #7c3aed);
		color: var(--muted-color, #666);
		font-style: italic;
	}

	/* Theme-aware styling */
	:global(.theme-minimal) .sidebar-content :global(h1),
	:global(.theme-minimal) .sidebar-content :global(h2),
	:global(.theme-minimal) .sidebar-content :global(h3) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
	}

	:global(.theme-minimal) .sidebar-content :global(.column:last-child),
	:global(.theme-minimal) .sidebar-content :global(> *:last-child) {
		background: var(--sidebar-bg, rgba(0, 0, 0, 0.02));
	}

	:global(.theme-terminal) .sidebar-content {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	:global(.theme-terminal) .sidebar-content :global(h1),
	:global(.theme-terminal) .sidebar-content :global(h2),
	:global(.theme-terminal) .sidebar-content :global(h3) {
		text-shadow: 0 0 10px var(--accent-color, #00ff00);
	}

	:global(.theme-terminal) .sidebar-content :global(.column:last-child),
	:global(.theme-terminal) .sidebar-content :global(> *:last-child) {
		background: var(--sidebar-bg, rgba(0, 255, 0, 0.05));
		border-left-color: var(--accent-color, #00ff00);
	}

	:global(.theme-terminal) .sidebar-content :global(ul) {
		list-style: none;
		padding-left: 1em;
	}

	:global(.theme-terminal) .sidebar-content :global(li)::before {
		content: '> ';
		color: var(--accent-color, #00ff00);
	}

	:global(.theme-gradient) .sidebar-content :global(h1),
	:global(.theme-gradient) .sidebar-content :global(h2) {
		background: var(--heading-gradient, linear-gradient(135deg, #7c3aed, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
	}

	:global(.theme-gradient) .sidebar-content :global(.column:last-child),
	:global(.theme-gradient) .sidebar-content :global(> *:last-child) {
		background: var(--sidebar-bg, rgba(124, 58, 237, 0.05));
		backdrop-filter: blur(5px);
		border-left-color: transparent;
		border-image: linear-gradient(135deg, #7c3aed, #06b6d4) 1;
	}

	:global(.theme-brutalist) .sidebar-content :global(h1),
	:global(.theme-brutalist) .sidebar-content :global(h2),
	:global(.theme-brutalist) .sidebar-content :global(h3) {
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	:global(.theme-brutalist) .sidebar-content :global(.column:last-child),
	:global(.theme-brutalist) .sidebar-content :global(> *:last-child) {
		background: var(--sidebar-bg, rgba(0, 0, 0, 0.05));
		border-radius: 0;
		border-left-width: 6px;
		border-left-color: var(--accent-color, #000);
	}

	:global(.theme-keynote) .sidebar-content :global(h1),
	:global(.theme-keynote) .sidebar-content :global(h2),
	:global(.theme-keynote) .sidebar-content :global(h3) {
		font-weight: 500;
		text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	:global(.theme-keynote) .sidebar-content :global(.column:last-child),
	:global(.theme-keynote) .sidebar-content :global(> *:last-child) {
		background: var(--sidebar-bg, rgba(0, 0, 0, 0.02));
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
		border-left-color: var(--accent-color, #007aff);
	}
</style>
