<script lang="ts">
	/**
	 * LayoutCover - Full-bleed background image layout.
	 *
	 * This layout is used for:
	 * - Dramatic opening slides
	 * - Section transitions with visual impact
	 * - Image-focused slides with overlay text
	 * - Chapter breaks in longer presentations
	 *
	 * Expected HTML structure:
	 * - h1: Main title overlay (optional)
	 * - p: Subtitle or description (optional)
	 * - Background image set via slide directives
	 *
	 * Example markdown:
	 * ```
	 * <!--
	 * layout: cover
	 * background: image.jpg
	 * -->
	 *
	 * # The Future of Technology
	 *
	 * Where we go from here
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

<div class="layout-cover">
	<div class="cover-overlay"></div>
	<div class="cover-content">
		{@render children()}
	</div>
</div>

<style>
	/* Legacy CSS removed - will be replaced with Tailwind in US-107D */
</style>
