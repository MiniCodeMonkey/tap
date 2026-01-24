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
	 *
	 * Typography styles are in src/lib/styles/layouts.css.
	 * This component provides the structure, CSS handles the styling.
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
	LayoutTitle uses CSS from layouts.css for centered title presentation.
	Content is centered both horizontally and vertically using flexbox.
-->
<div class="layout-title">
	<div class="title-content">
		{@render children()}
	</div>
</div>
