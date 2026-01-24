<script lang="ts">
	/**
	 * LayoutCodeFocus - Full-width code block layout.
	 *
	 * This layout is used for:
	 * - Slides that focus on a single code snippet
	 * - Live code demonstrations
	 * - Code walkthroughs with optional title/description
	 *
	 * Expected HTML structure:
	 * - h2 or h3: Optional title above the code (optional)
	 * - pre/code: The main code block (required)
	 * - p: Optional description or explanation (optional)
	 *
	 * The code block takes prominence, filling most of the available space.
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
	LayoutCodeFocus uses CSS from layouts.css for code-centric presentation.
	The code block takes prominence, filling most of the available space.
-->
<div class="layout-code-focus">
	{@render children()}
</div>
