<script lang="ts">
	/**
	 * LayoutBlank - Empty canvas layout.
	 *
	 * This layout is used for:
	 * - Custom designs that don't fit standard layouts
	 * - Embedded iframes or custom HTML
	 * - Full creative freedom slides
	 * - Animations or interactive content
	 *
	 * Expected HTML structure:
	 * - Any content - rendered as-is without additional styling
	 * - Full slide area available for custom positioning
	 *
	 * Example markdown:
	 * ```
	 * <!--
	 * layout: blank
	 * -->
	 *
	 * <div style="position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%);">
	 *   Custom positioned content
	 * </div>
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

<div class="layout-blank">
	<div class="blank-content">
		{@render children()}
	</div>
</div>

<style>
	/* Legacy CSS removed - will be replaced with Tailwind in US-107D */
</style>
