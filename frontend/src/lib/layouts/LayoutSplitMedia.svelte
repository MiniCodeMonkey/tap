<script lang="ts">
	/**
	 * LayoutSplitMedia - Image + text side-by-side layout.
	 *
	 * This layout is used for:
	 * - Showcasing an image alongside explanatory text
	 * - Product demonstrations with descriptions
	 * - Screenshots with annotations
	 * - Any slide where media and text should be equally prominent
	 *
	 * Expected HTML structure:
	 * - Content separated by ||| delimiter (parsed by transformer)
	 * - First portion: image or media content (left side)
	 * - Second portion: text content (right side)
	 * OR
	 * - First portion: text content (left side)
	 * - Second portion: image or media content (right side)
	 *
	 * Example markdown:
	 * ```
	 * ![Product screenshot](screenshot.png)
	 *
	 * |||
	 *
	 * ## New Dashboard
	 *
	 * - Real-time analytics
	 * - Customizable widgets
	 * - Dark mode support
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

<div class="layout-split-media">
	<div class="split-media-content">
		{@render children()}
	</div>
</div>

<style>
	/* Legacy CSS removed - will be replaced with Tailwind in US-107D */
</style>
