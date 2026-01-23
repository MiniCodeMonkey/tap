<script lang="ts">
	import type { Slide, BackgroundConfig, Transition, FragmentGroup } from '$lib/types';
	import { fade, fly, scale } from 'svelte/transition';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** The slide data to render */
		slide: Slide;
		/** Number of visible fragments (-1 = none, 0 = first, etc.) */
		visibleFragments?: number;
		/** Whether the slide is currently active (for transitions) */
		active?: boolean;
		/** Direction of slide entry/exit for directional transitions */
		direction?: 'forward' | 'backward';
		/** Custom transition duration in milliseconds */
		transitionDuration?: number;
	}

	let {
		slide,
		visibleFragments = -1,
		active = true,
		direction = 'forward',
		transitionDuration = 400
	}: Props = $props();

	// ============================================================================
	// Computed Values
	// ============================================================================

	/**
	 * Get CSS classes for the slide based on layout.
	 */
	let layoutClass = $derived(`layout-${slide.layout}`);

	/**
	 * Get the transition to use for this slide.
	 * Falls back to 'fade' if not specified.
	 */
	let slideTransition = $derived<Transition>(slide.transition ?? 'fade');

	/**
	 * Check if the slide has fragments.
	 */
	let hasFragments = $derived(
		slide.fragments !== undefined && slide.fragments.length > 0
	);

	/**
	 * Get fragments with visibility state.
	 */
	let fragmentsWithVisibility = $derived.by(() => {
		if (!slide.fragments) return [];
		return slide.fragments.map((fragment) => ({
			...fragment,
			visible: fragment.index <= visibleFragments
		}));
	});

	// ============================================================================
	// Background Handling
	// ============================================================================

	/**
	 * Generate CSS for the background.
	 */
	function getBackgroundStyles(bg: BackgroundConfig | undefined): string {
		if (!bg) return '';

		switch (bg.type) {
			case 'image':
				return `background-image: url('${bg.value}'); background-size: cover; background-position: center;`;
			case 'gradient':
				return `background: ${bg.value};`;
			case 'color':
			default:
				return `background-color: ${bg.value};`;
		}
	}

	let backgroundStyles = $derived(getBackgroundStyles(slide.background));

	// ============================================================================
	// Transition Functions
	// ============================================================================

	/**
	 * Check if reduced motion is preferred.
	 */
	function prefersReducedMotion(): boolean {
		if (typeof window === 'undefined') return false;
		return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
	}

	/**
	 * Get the appropriate Svelte transition for the slide.
	 */
	function getTransition(node: Element) {
		if (prefersReducedMotion()) {
			return fade(node, { duration: 0 });
		}

		const duration = transitionDuration;

		switch (slideTransition) {
			case 'none':
				return fade(node, { duration: 0 });
			case 'slide': {
				const x = direction === 'forward' ? 100 : -100;
				return fly(node, { x, duration });
			}
			case 'push': {
				const x = direction === 'forward' ? 50 : -50;
				return fly(node, { x, duration, opacity: 0.5 });
			}
			case 'zoom':
				return scale(node, { start: direction === 'forward' ? 0.8 : 1.2, duration });
			case 'fade':
			default:
				return fade(node, { duration });
		}
	}

	// ============================================================================
	// Fragment HTML Processing
	// ============================================================================

	/**
	 * Process HTML content to wrap fragments with visibility classes.
	 * If the slide has fragments, split content at fragment markers and wrap each section.
	 */
	function processHtmlWithFragments(
		html: string,
		fragments: Array<FragmentGroup & { visible: boolean }>
	): string {
		if (fragments.length === 0) {
			return html;
		}

		// Build the HTML with fragment wrappers
		let result = '';
		for (const fragment of fragments) {
			const visibilityClass = fragment.visible ? 'fragment-visible' : 'fragment-hidden';
			result += `<div class="fragment ${visibilityClass}" data-fragment-index="${fragment.index}">${fragment.content}</div>`;
		}

		return result;
	}

	/**
	 * Get the final HTML content, either with or without fragment processing.
	 */
	let processedHtml = $derived.by(() => {
		if (hasFragments) {
			return processHtmlWithFragments(slide.html, fragmentsWithVisibility);
		}
		return slide.html;
	});
</script>

{#if active}
	<div
		class="slide-renderer {layoutClass}"
		class:has-fragments={hasFragments}
		style={backgroundStyles}
		in:getTransition
		out:getTransition
	>
		<div class="slide-content">
			{@html processedHtml}
		</div>
	</div>
{/if}

<style>
	.slide-renderer {
		/* Fill the parent container */
		width: 100%;
		height: 100%;

		/* Position for absolute children */
		position: relative;

		/* Padding for content */
		padding: var(--slide-padding, 80px);

		/* Box sizing */
		box-sizing: border-box;

		/* Background defaults (can be overridden by inline styles) */
		background-color: var(--slide-bg, #fff);
	}

	.slide-content {
		/* Fill available space */
		width: 100%;
		height: 100%;

		/* Enable content layout */
		display: flex;
		flex-direction: column;
	}

	/* ========================================================================
	 * Layout-specific styles
	 * ======================================================================== */

	/* Default layout - standard content flow */
	.layout-default .slide-content {
		justify-content: flex-start;
		gap: var(--content-gap, 24px);
	}

	/* Title layout - centered with emphasis */
	.layout-title .slide-content {
		justify-content: center;
		align-items: center;
		text-align: center;
	}

	.layout-title :global(h1) {
		font-size: var(--title-font-size, 6rem);
		font-weight: 700;
		margin: 0;
		line-height: 1.1;
	}

	.layout-title :global(p) {
		font-size: var(--subtitle-font-size, 2.5rem);
		color: var(--muted-color, #666);
		margin: 0;
		margin-top: 0.5em;
	}

	/* Section layout - large section header */
	.layout-section .slide-content {
		justify-content: center;
		align-items: center;
		text-align: center;
	}

	.layout-section :global(h2) {
		font-size: var(--section-font-size, 4rem);
		font-weight: 600;
		margin: 0;
	}

	/* Two-column layout */
	.layout-two-column .slide-content {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: var(--column-gap, 48px);
		align-items: start;
	}

	/* Three-column layout */
	.layout-three-column .slide-content {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: var(--column-gap, 48px);
		align-items: start;
	}

	/* Code-focus layout - full width code block */
	.layout-code-focus .slide-content {
		justify-content: center;
	}

	.layout-code-focus :global(pre) {
		max-height: 80%;
		overflow: auto;
		font-size: var(--code-font-size, 1.5rem);
	}

	/* Quote layout - centered blockquote */
	.layout-quote .slide-content {
		justify-content: center;
		align-items: center;
		text-align: center;
		padding: 0 10%;
	}

	.layout-quote :global(blockquote) {
		font-size: var(--quote-font-size, 3rem);
		font-style: italic;
		margin: 0;
		padding: 0;
		border: none;
		position: relative;
	}

	.layout-quote :global(blockquote)::before {
		content: '\201C';
		font-size: 6rem;
		position: absolute;
		top: -0.5em;
		left: -0.3em;
		color: var(--accent-color, #7c3aed);
		opacity: 0.3;
	}

	.layout-quote :global(blockquote p:last-child) {
		font-size: var(--quote-attribution-size, 1.5rem);
		font-style: normal;
		margin-top: 1em;
		color: var(--muted-color, #666);
	}

	/* Big-stat layout - large number emphasis */
	.layout-big-stat .slide-content {
		justify-content: center;
		align-items: center;
		text-align: center;
	}

	.layout-big-stat :global(.stat-number),
	.layout-big-stat :global(strong) {
		font-size: var(--stat-font-size, 12rem);
		font-weight: 700;
		line-height: 1;
		color: var(--accent-color, #7c3aed);
	}

	.layout-big-stat :global(p) {
		font-size: var(--stat-label-size, 2rem);
		color: var(--muted-color, #666);
		margin-top: 0.5em;
	}

	/* Cover layout - full-bleed background image */
	.layout-cover {
		padding: 0;
	}

	.layout-cover .slide-content {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		justify-content: flex-end;
		padding: var(--slide-padding, 80px);
		background: linear-gradient(
			to top,
			rgba(0, 0, 0, 0.7) 0%,
			rgba(0, 0, 0, 0.3) 50%,
			transparent 100%
		);
		color: #fff;
	}

	.layout-cover :global(h1),
	.layout-cover :global(h2) {
		color: #fff;
		text-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
	}

	/* Sidebar layout - main content with sidebar */
	.layout-sidebar .slide-content {
		display: grid;
		grid-template-columns: 2fr 1fr;
		gap: var(--column-gap, 48px);
		align-items: start;
	}

	/* Split-media layout - image + text side by side */
	.layout-split-media .slide-content {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0;
		padding: 0;
	}

	.layout-split-media :global(img) {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	/* Blank layout - empty canvas */
	.layout-blank {
		padding: 0;
	}

	.layout-blank .slide-content {
		/* Minimal styling, full control to content */
		display: block;
	}

	/* ========================================================================
	 * Fragment styles
	 * ======================================================================== */

	:global(.fragment) {
		transition: opacity var(--fragment-duration, 400ms) ease-out,
			transform var(--fragment-duration, 400ms) ease-out;
	}

	:global(.fragment-hidden) {
		opacity: 0;
		transform: translateY(20px);
		pointer-events: none;
	}

	:global(.fragment-visible) {
		opacity: 1;
		transform: translateY(0);
		pointer-events: auto;
	}

	/* Reduced motion support for fragments */
	@media (prefers-reduced-motion: reduce) {
		:global(.fragment) {
			transition: none;
		}

		:global(.fragment-hidden) {
			transform: none;
		}
	}

	/* ========================================================================
	 * Common element styles
	 * ======================================================================== */

	.slide-renderer :global(h1) {
		font-size: var(--h1-font-size, 4rem);
		font-weight: 700;
		margin: 0 0 0.5em;
		line-height: 1.2;
	}

	.slide-renderer :global(h2) {
		font-size: var(--h2-font-size, 3rem);
		font-weight: 600;
		margin: 0 0 0.5em;
		line-height: 1.2;
	}

	.slide-renderer :global(h3) {
		font-size: var(--h3-font-size, 2.25rem);
		font-weight: 600;
		margin: 0 0 0.5em;
		line-height: 1.3;
	}

	.slide-renderer :global(p) {
		font-size: var(--body-font-size, 2rem);
		margin: 0 0 0.75em;
		line-height: 1.6;
	}

	.slide-renderer :global(ul),
	.slide-renderer :global(ol) {
		font-size: var(--body-font-size, 2rem);
		margin: 0 0 0.75em;
		padding-left: 1.5em;
		line-height: 1.6;
	}

	.slide-renderer :global(li) {
		margin-bottom: 0.5em;
	}

	.slide-renderer :global(code) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		font-size: 0.9em;
		background-color: var(--code-bg, rgba(0, 0, 0, 0.05));
		padding: 0.1em 0.3em;
		border-radius: 4px;
	}

	.slide-renderer :global(pre) {
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		font-size: var(--code-font-size, 1.5rem);
		background-color: var(--pre-bg, #1e1e1e);
		color: var(--pre-color, #d4d4d4);
		padding: 1.5em;
		border-radius: 8px;
		overflow-x: auto;
		margin: 0 0 0.75em;
		line-height: 1.5;
	}

	.slide-renderer :global(pre code) {
		background: none;
		padding: 0;
		font-size: inherit;
	}

	.slide-renderer :global(img) {
		max-width: 100%;
		height: auto;
		border-radius: 4px;
	}

	.slide-renderer :global(a) {
		color: var(--link-color, #7c3aed);
		text-decoration: none;
	}

	.slide-renderer :global(a:hover) {
		text-decoration: underline;
	}

	.slide-renderer :global(table) {
		width: 100%;
		border-collapse: collapse;
		font-size: var(--table-font-size, 1.75rem);
		margin: 0 0 0.75em;
	}

	.slide-renderer :global(th),
	.slide-renderer :global(td) {
		padding: 0.75em 1em;
		text-align: left;
		border-bottom: 1px solid var(--border-color, rgba(0, 0, 0, 0.1));
	}

	.slide-renderer :global(th) {
		font-weight: 600;
		background-color: var(--table-header-bg, rgba(0, 0, 0, 0.02));
	}
</style>
