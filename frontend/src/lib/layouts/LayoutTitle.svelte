<script lang="ts">
	/**
	 * LayoutTitle - Centered title slide with optional subtitle.
	 *
	 * This layout is used for:
	 * - Title slides at the beginning of presentations
	 * - Section dividers with prominent headings
	 * - Any slide where you want a centered, bold title
	 *
	 * Expected HTML structure:
	 * - h1: Main title (required)
	 * - p: Subtitle (optional)
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

<div class="layout-title">
	<div class="title-content">
		{@render children()}
	</div>
</div>

<style>
	/* Legacy CSS removed - will be replaced with Tailwind in US-107A */
</style>
