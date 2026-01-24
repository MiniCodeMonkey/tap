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

<div class="layout-section">
	<div class="section-content">
		{@render children()}
	</div>
</div>

<style>
	/* Legacy CSS removed - will be replaced with Tailwind in US-107A */
</style>
