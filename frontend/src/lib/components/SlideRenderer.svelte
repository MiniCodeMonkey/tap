<script lang="ts">
	import type { Slide, BackgroundConfig, Transition, FragmentGroup } from '$lib/types';
	import { fade, fly, scale } from 'svelte/transition';
	import { renderMermaidBlocksInElement } from '$lib/utils/mermaid';

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
	 * Check if the slide has multiple fragments (actual pause markers).
	 * A single fragment means no pause markers - just show the slide HTML directly.
	 */
	let hasFragments = $derived(
		slide.fragments !== undefined && slide.fragments.length > 1
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

	// ============================================================================
	// Mermaid Diagram Rendering
	// ============================================================================

	/**
	 * Reference to the slide content element for DOM manipulation.
	 */
	let slideContentElement: HTMLElement | undefined = $state();

	/**
	 * Render mermaid diagrams when the slide content is mounted or changes.
	 * This runs after the HTML is inserted into the DOM via {@html}.
	 */
	$effect(() => {
		if (slideContentElement && active) {
			// Re-render mermaid diagrams when processedHtml changes
			// eslint-disable-next-line @typescript-eslint/no-unused-expressions
			processedHtml;

			// Use a microtask to ensure DOM has been updated
			queueMicrotask(() => {
				renderMermaidBlocksInElement(slideContentElement!);
			});
		}
	});
</script>

<!--
	SlideRenderer uses Tailwind utilities for layout and structure.
	- Full width/height for slide area
	- Layout classes (layout-title, layout-default, etc.) are passed to content
	- Background support via inline styles (image, gradient, color)
	- Fragment visibility controlled via Tailwind-based CSS classes
-->
{#if active}
	<div
		class="slide-renderer {layoutClass} w-full h-full p-slide relative overflow-hidden {hasFragments ? 'has-fragments' : ''}"
		style={backgroundStyles}
		in:getTransition
		out:getTransition
	>
		<div class="slide-content w-full h-full" bind:this={slideContentElement}>
			{@html processedHtml}
		</div>
	</div>
{/if}

<style>
	/*
	 * Fragment animation styles - kept as custom CSS because they target
	 * dynamically generated HTML content via {@html} which cannot use
	 * Tailwind classes directly.
	 *
	 * These use Tailwind-like timing values:
	 * - duration-fragment (300ms from tailwind.config.js)
	 * - translateY(20px) for slide-up effect
	 */
	:global(.fragment) {
		transition: opacity 300ms ease-out, transform 300ms ease-out;
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

	/* Reduced motion: disable transform animations, instant opacity change */
	@media (prefers-reduced-motion: reduce) {
		:global(.fragment) {
			transition: none;
		}

		:global(.fragment-hidden) {
			transform: none;
		}
	}

	/*
	 * Mermaid diagram container styles.
	 * Centers diagrams and ensures they don't overflow the slide.
	 * Uses max-height to prevent vertical overflow while preserving aspect ratio.
	 * Small diagrams stay at natural size (no upscaling).
	 */
	:global(.mermaid-diagram) {
		display: flex;
		justify-content: center;
		align-items: center;
		width: 100%;
		/* Limit height to prevent overflow, leaving room for other content */
		max-height: 70vh;
		margin: 1rem 0;
		overflow: hidden;
	}

	:global(.mermaid-diagram svg) {
		/* Constrain to container bounds while preserving aspect ratio */
		max-width: 100%;
		max-height: 100%;
		/* Ensure SVG scales proportionally */
		width: auto;
		height: auto;
		/* Prevent upscaling beyond natural size */
		object-fit: contain;
	}

	/*
	 * Mermaid error styles.
	 * Shows a visible error with the original code for debugging.
	 * Uses theme-aware error colors via CSS custom properties.
	 */
	:global(.mermaid-error) {
		padding: 1.25rem 1.5rem;
		border-radius: 0.5rem;
		background-color: var(--color-error-bg, rgba(220, 38, 38, 0.1));
		border: 1px solid color-mix(in srgb, var(--color-error, #dc2626) 40%, transparent);
		margin: 1rem 0;
	}

	:global(.mermaid-error-message) {
		color: var(--color-error, #dc2626);
		font-weight: 600;
		margin-bottom: 0.75rem;
		font-family: var(--font-sans, inherit);
	}

	:global(.mermaid-error-code) {
		font-size: 0.875rem;
		opacity: 0.85;
		background-color: var(--color-surface, rgba(0, 0, 0, 0.05));
		border-radius: 0.375rem;
		padding: 1rem;
		border: 1px solid var(--color-border, rgba(0, 0, 0, 0.1));
	}

	:global(.mermaid-error-code code) {
		white-space: pre-wrap;
		word-break: break-word;
		font-family: var(--font-mono, monospace);
		color: var(--color-text, inherit);
	}
</style>
