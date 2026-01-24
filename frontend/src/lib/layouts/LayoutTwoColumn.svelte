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
	/* Legacy CSS removed - will be replaced with Tailwind in US-107B */
</style>
