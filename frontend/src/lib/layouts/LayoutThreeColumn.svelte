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
	/* Legacy CSS removed - will be replaced with Tailwind in US-107B */
</style>
