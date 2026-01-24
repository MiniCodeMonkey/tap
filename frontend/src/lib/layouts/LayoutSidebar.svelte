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
	LayoutSidebar uses CSS from layouts.css for grid-based layout.
	Uses grid-template-columns: 2fr 1fr for main content (2/3) and sidebar (1/3).
	Sidebar has a left border for visual separation.
-->
<div class="layout-sidebar">
	<div class="sidebar-content">
		{@render children()}
	</div>
</div>
