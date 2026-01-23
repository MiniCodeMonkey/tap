<script lang="ts">
	/**
	 * LayoutQuote - Styled blockquote layout for impactful quotes.
	 *
	 * This layout is used for:
	 * - Inspirational or thought-provoking quotes
	 * - Key takeaways or memorable statements
	 * - Customer testimonials
	 * - Any slide where a quote is the primary focus
	 *
	 * Expected HTML structure:
	 * - blockquote: The main quote text (required)
	 * - p: Attribution/source (optional, typically inside or after blockquote)
	 *
	 * Example markdown:
	 * ```
	 * > "The best way to predict the future is to invent it."
	 * >
	 * > — Alan Kay
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

<div class="layout-quote">
	<div class="quote-content">
		{@render children()}
	</div>
</div>

<style>
	.layout-quote {
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

	.quote-content {
		/* Constrain width for readability */
		max-width: 85%;

		/* Flex for vertical content arrangement */
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--content-gap, 2rem);

		/* Center text */
		text-align: center;
	}

	/* Main blockquote styling */
	.quote-content :global(blockquote) {
		/* Large, impactful text */
		font-size: var(--quote-font-size, 3.5rem);
		font-weight: 400;
		font-style: italic;
		margin: 0;
		padding: 0;
		line-height: 1.4;
		color: var(--quote-color, inherit);

		/* Remove default blockquote styling */
		border: none;

		/* Visual distinction */
		position: relative;
	}

	/* Decorative quote marks */
	.quote-content :global(blockquote)::before {
		content: '"';
		font-size: 6rem;
		font-family: Georgia, serif;
		font-style: normal;
		color: var(--accent-color, #7c3aed);
		opacity: 0.3;
		position: absolute;
		top: -2rem;
		left: -1rem;
		line-height: 1;
	}

	/* Quote text paragraphs */
	.quote-content :global(blockquote p) {
		font-size: inherit;
		font-weight: inherit;
		font-style: inherit;
		margin: 0;
		line-height: inherit;
		color: inherit;
	}

	/* Attribution styling - appears smaller and muted */
	.quote-content :global(blockquote p:last-child:not(:first-child)),
	.quote-content :global(p) {
		font-size: var(--attribution-font-size, 1.75rem);
		font-style: normal;
		color: var(--muted-color, #666);
		margin-top: 0.5em;
	}

	/* Em dash attribution pattern */
	.quote-content :global(blockquote p:last-child:not(:first-child))::before {
		content: '';
	}

	/* Standalone attribution outside blockquote */
	.quote-content > :global(p) {
		font-size: var(--attribution-font-size, 1.75rem);
		font-style: normal;
		color: var(--muted-color, #666);
	}

	/* Theme-aware styling */
	:global(.theme-minimal) .quote-content :global(blockquote) {
		font-family: var(--font-family, 'Helvetica Neue', Helvetica, Arial, sans-serif);
	}

	:global(.theme-minimal) .quote-content :global(blockquote)::before {
		color: var(--accent-color, #7c3aed);
	}

	:global(.theme-terminal) .quote-content :global(blockquote) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		font-style: normal;
		border-left: 4px solid var(--accent-color, #00ff00);
		padding-left: 2rem;
		text-align: left;
	}

	:global(.theme-terminal) .quote-content :global(blockquote)::before {
		content: '>';
		font-size: 3rem;
		color: var(--accent-color, #00ff00);
		text-shadow: 0 0 10px var(--accent-color, #00ff00);
		position: static;
		margin-right: 0.5em;
	}

	:global(.theme-gradient) .quote-content :global(blockquote) {
		background: var(--quote-gradient, linear-gradient(135deg, #7c3aed, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
	}

	:global(.theme-gradient) .quote-content :global(blockquote)::before {
		background: var(--quote-gradient, linear-gradient(135deg, #7c3aed, #06b6d4));
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
		opacity: 0.5;
	}

	:global(.theme-brutalist) .quote-content :global(blockquote) {
		font-style: normal;
		text-transform: uppercase;
		letter-spacing: 0.02em;
		border: 4px solid var(--accent-color, #000);
		padding: 2rem;
	}

	:global(.theme-brutalist) .quote-content :global(blockquote)::before {
		display: none;
	}

	:global(.theme-keynote) .quote-content :global(blockquote) {
		font-weight: 300;
		text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	:global(.theme-keynote) .quote-content :global(blockquote)::before {
		color: var(--accent-color, #007aff);
		opacity: 0.2;
	}
</style>
