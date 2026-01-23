<script lang="ts">
	/**
	 * LayoutCodeFocus - Full-width code block layout.
	 *
	 * This layout is used for:
	 * - Slides that focus on a single code snippet
	 * - Live code demonstrations
	 * - Code walkthroughs with optional title/description
	 *
	 * Expected HTML structure:
	 * - h2 or h3: Optional title above the code (optional)
	 * - pre/code: The main code block (required)
	 * - p: Optional description or explanation (optional)
	 *
	 * The code block takes prominence, filling most of the available space.
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

<div class="layout-code-focus">
	<div class="code-focus-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-code-focus {
		/* Fill parent container */
		width: 100%;
		height: 100%;

		/* Padding for content spacing */
		padding: var(--slide-padding, 3rem);
		box-sizing: border-box;

		/* Flex for content positioning */
		display: flex;
		flex-direction: column;
		justify-content: center;
	}

	.code-focus-content {
		/* Full width content area */
		width: 100%;
		max-width: 100%;

		/* Flex for vertical content arrangement */
		display: flex;
		flex-direction: column;
		gap: var(--content-gap, 1.5rem);
	}

	/* Title styling - smaller than default to leave room for code */
	.code-focus-content :global(h1) {
		font-size: var(--h1-font-size, 3rem);
		font-weight: 700;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.code-focus-content :global(h2) {
		font-size: var(--h2-font-size, 2.5rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.2;
		color: var(--heading-color, inherit);
	}

	.code-focus-content :global(h3) {
		font-size: var(--h3-font-size, 2rem);
		font-weight: 600;
		margin: 0;
		line-height: 1.3;
		color: var(--heading-color, inherit);
	}

	/* Paragraph styling - for descriptions */
	.code-focus-content :global(p) {
		font-size: var(--body-font-size, 1.5rem);
		margin: 0;
		line-height: 1.6;
		color: var(--muted-color, #666);
	}

	/* Main code block - the focus of this layout */
	.code-focus-content :global(pre) {
		/* Larger code for readability */
		font-size: var(--code-font-size, 1.75rem);
		margin: 0;
		padding: 2rem;
		background: var(--code-bg, #1e1e1e);
		color: var(--code-color, #d4d4d4);
		border-radius: var(--code-border-radius, 0.75rem);
		overflow-x: auto;

		/* Allow code to take available space */
		flex-grow: 1;
		max-height: 70vh;
		overflow-y: auto;

		/* Line height for readability */
		line-height: 1.5;
	}

	.code-focus-content :global(code) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	/* Inline code in paragraphs */
	.code-focus-content :global(p code) {
		font-size: 0.9em;
		padding: 0.2em 0.4em;
		background: var(--inline-code-bg, rgba(0, 0, 0, 0.1));
		border-radius: 0.25em;
	}

	/* Code title/filename styling */
	.code-focus-content :global(pre + p),
	.code-focus-content :global(p:has(+ pre)) {
		font-size: var(--code-caption-font-size, 1.25rem);
		color: var(--muted-color, #666);
	}

	/* Theme-aware styling */
	:global(.theme-minimal) .code-focus-content :global(h1),
	:global(.theme-minimal) .code-focus-content :global(h2),
	:global(.theme-minimal) .code-focus-content :global(h3) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
	}

	:global(.theme-minimal) .code-focus-content :global(pre) {
		box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
	}

	:global(.theme-terminal) .code-focus-content {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
	}

	:global(.theme-terminal) .code-focus-content :global(h1),
	:global(.theme-terminal) .code-focus-content :global(h2),
	:global(.theme-terminal) .code-focus-content :global(h3) {
		text-shadow: 0 0 10px var(--accent-color, #00ff00);
	}

	:global(.theme-terminal) .code-focus-content :global(pre) {
		background: var(--code-bg, #000);
		border: 1px solid var(--accent-color, #00ff00);
		box-shadow: 0 0 20px rgba(0, 255, 0, 0.2);
	}

	:global(.theme-gradient) .code-focus-content :global(h1),
	:global(.theme-gradient) .code-focus-content :global(h2) {
		background: var(--heading-gradient, linear-gradient(135deg, #7c3aed, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
	}

	:global(.theme-gradient) .code-focus-content :global(pre) {
		background: var(--code-bg, rgba(0, 0, 0, 0.8));
		backdrop-filter: blur(10px);
		border: 1px solid rgba(255, 255, 255, 0.1);
	}

	:global(.theme-brutalist) .code-focus-content :global(h1),
	:global(.theme-brutalist) .code-focus-content :global(h2),
	:global(.theme-brutalist) .code-focus-content :global(h3) {
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	:global(.theme-brutalist) .code-focus-content :global(pre) {
		border-radius: 0;
		border: 4px solid var(--accent-color, #000);
	}

	:global(.theme-keynote) .code-focus-content :global(h1),
	:global(.theme-keynote) .code-focus-content :global(h2),
	:global(.theme-keynote) .code-focus-content :global(h3) {
		font-weight: 500;
		text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	:global(.theme-keynote) .code-focus-content :global(pre) {
		box-shadow: 0 10px 40px rgba(0, 0, 0, 0.15);
	}
</style>
