<script lang="ts">
	/**
	 * LayoutBigStat - Large number emphasis layout for statistics.
	 *
	 * This layout is used for:
	 * - Highlighting key metrics or numbers
	 * - KPI displays
	 * - Impact statistics
	 * - Any slide where a number is the primary focus
	 *
	 * Expected HTML structure:
	 * - h1/strong: The big number/stat (required)
	 * - p: Description or context (optional)
	 *
	 * Example markdown:
	 * ```
	 * # 99.9%
	 *
	 * Uptime over the last 12 months
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

<div class="layout-big-stat">
	<div class="stat-content">
		{@render children()}
	</div>
</div>

<style>
	/* Legacy CSS removed - will be replaced with Tailwind in US-107C */
</style>
