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
	LayoutDefault uses CSS from layouts.css for standard content presentation.
	Content flows vertically with proper spacing between elements.
-->
<div class="layout-default">
	<div class="default-content">
		{@render children()}
	</div>
</div>
