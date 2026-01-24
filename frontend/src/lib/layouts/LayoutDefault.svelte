<script lang="ts">
	/**
	 * LayoutDefault - Standard content layout for general-purpose slides.
	 *
	 * This layout is used for:
	 * - Regular content slides with headings and body text
	 * - Bullet point lists
	 * - Mixed content with images and text
	 * - Any slide that doesn't fit a more specific layout
	 *
	 * Expected HTML structure:
	 * - h1, h2, h3: Headings (optional)
	 * - p: Paragraphs
	 * - ul, ol: Lists
	 * - img: Images
	 * - pre, code: Code blocks
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

<div class="layout-default">
	<div class="default-content">
		{@render children()}
	</div>
</div>

<style>
	/* Legacy CSS removed - will be replaced with Tailwind in US-107A */
</style>
