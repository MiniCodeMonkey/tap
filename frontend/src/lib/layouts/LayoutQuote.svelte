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

<!--
	LayoutQuote uses CSS from layouts.css for impactful quote presentation.
	The blockquote is centered and styled with an accent border.
-->
<div class="layout-quote">
	{@render children()}
</div>
