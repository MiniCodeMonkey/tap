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
	/* Legacy CSS removed - will be replaced with Tailwind in US-107C */
</style>
