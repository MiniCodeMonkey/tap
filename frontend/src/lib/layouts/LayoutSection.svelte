<script lang="ts">
	/**
	 * LayoutSection - Large section header for dividing presentation sections.
	 *
	 * This layout is used for:
	 * - Section dividers between major presentation topics
	 * - Chapter headers
	 * - Any slide with a single prominent H2 heading
	 *
	 * Expected HTML structure:
	 * - h2: Section title (required)
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
	LayoutSection uses CSS from layouts.css for centered section header presentation.
	Content is centered both horizontally and vertically using flexbox.
-->
<div class="layout-section">
	<div class="section-content">
		{@render children()}
	</div>
</div>
