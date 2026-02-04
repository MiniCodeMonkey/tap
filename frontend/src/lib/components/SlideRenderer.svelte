<script lang="ts">
	import type { Slide, BackgroundConfig, Transition, FragmentGroup, Theme, MapConfig } from '$lib/types';
	import { fade, fly, scale } from 'svelte/transition';
	import { untrack } from 'svelte';
	import { renderMermaidBlocksInElement } from '$lib/utils/mermaid';
	import { highlightCodeBlocksInElement } from '$lib/utils/highlighting';
	import { parseMapConfig } from '$lib/utils/map';
	import {
		scrollRevealed as scrollRevealedStore,
		currentSlideHasMap,
		mapAnimationTriggered
	} from '$lib/stores/presentation';
	import MapSlide from './MapSlide.svelte';

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
		/** Theme to use for mermaid diagrams */
		theme?: Theme;
		/** Whether in print/PDF mode */
		isPrintMode?: boolean;
	}

	let {
		slide,
		visibleFragments = -1,
		active = true,
		direction = 'forward',
		transitionDuration = 400,
		theme = 'paper',
		isPrintMode = false
	}: Props = $props();

	// ============================================================================
	// Computed Values
	// ============================================================================

	/**
	 * Get CSS classes for the slide based on layout.
	 */
	let layoutClass = $derived(`layout-${slide.layout}`);

	/**
	 * Check if the layout should be full-bleed (no padding).
	 */
	let isFullBleed = $derived(
		slide.layout === 'split-media' || slide.layout === 'cover'
	);

	/**
	 * Get the transition to use for this slide.
	 * Falls back to 'fade' if not specified.
	 */
	let slideTransition = $derived<Transition>(slide.transition ?? 'fade');

	/**
	 * Check if the slide has block fragments (content in fragment array from pause markers).
	 * Block fragments have non-empty content that gets wrapped in divs.
	 */
	let hasBlockFragments = $derived(
		slide.fragments !== undefined &&
			slide.fragments.length > 1 &&
			slide.fragments.some((f) => f.content && f.content.trim() !== '')
	);

	/**
	 * Check if the slide has inline fragments (fragment classes in HTML from fragments: true).
	 * Inline fragments have empty content but fragments array is populated for counting.
	 */
	let hasInlineFragments = $derived(
		slide.fragments !== undefined &&
			slide.fragments.length > 0 &&
			slide.fragments.every((f) => !f.content || f.content.trim() === '')
	);

	/**
	 * Check if the slide has scroll reveal enabled.
	 */
	let hasScrollReveal = $derived(slide.scroll === true);

	/**
	 * Get the scroll animation duration in milliseconds.
	 * Default is 2000ms for a readable scroll speed.
	 */
	let scrollSpeed = $derived(slide.scrollSpeed || 2000);

	/**
	 * Track the scroll distance (how far content extends beyond viewport).
	 */
	let scrollDistance = $state(0);

	/**
	 * Whether content actually needs scrolling (extends beyond viewport).
	 */
	let needsScroll = $derived(scrollDistance > 0);

	/**
	 * Track if scroll measurement is ready.
	 * Prevents animation before we know the scroll distance.
	 */
	let scrollMeasured = $state(false);

	/**
	 * Get fragments with visibility state (for block fragments).
	 */
	let fragmentsWithVisibility = $derived.by(() => {
		if (!slide.fragments) return [];
		return slide.fragments.map((fragment) => ({
			...fragment,
			visible: fragment.index <= visibleFragments
		}));
	});

	// ============================================================================
	// Map Slide State
	// ============================================================================

	/**
	 * Parsed map configuration from the slide content.
	 * Null if no map code block is present.
	 */
	let mapConfig = $state<MapConfig | null>(null);

	/**
	 * Reference to the MapSlide component for programmatic control.
	 */
	let mapSlideRef: MapSlide | undefined = $state();

	/**
	 * Check if the slide has a map (for fragment-like behavior).
	 */
	let hasMap = $derived(mapConfig !== null);

	/**
	 * Subscribe to map animation state from store.
	 */
	let mapAnimationTriggeredFromStore = $state(false);

	$effect(() => {
		const unsubscribe = mapAnimationTriggered.subscribe((value) => {
			mapAnimationTriggeredFromStore = value;
		});
		return unsubscribe;
	});

	/**
	 * Detect map code blocks in the HTML and extract config.
	 * Returns the HTML with map code blocks removed.
	 */
	function extractMapConfig(html: string): { html: string; config: MapConfig | null } {
		// Check for map code block pattern
		const mapBlockRegex = /<pre><code class="language-map">([\s\S]*?)<\/code><\/pre>/;
		const match = html.match(mapBlockRegex);

		if (!match) {
			return { html, config: null };
		}

		// Decode HTML entities in the code content
		const matchContent = match[1];
		if (!matchContent) {
			return { html, config: null };
		}

		const codeContent = matchContent
			.replace(/&lt;/g, '<')
			.replace(/&gt;/g, '>')
			.replace(/&amp;/g, '&')
			.replace(/&quot;/g, '"')
			.replace(/&#39;/g, "'");

		const config = parseMapConfig(codeContent);

		if (!config) {
			// Leave the code block for error display
			return { html, config: null };
		}

		// Remove the map code block from HTML (map will render separately)
		const cleanedHtml = html.replace(mapBlockRegex, '');

		return { html: cleanedHtml, config };
	}

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
	 * For block fragments, wraps content in divs.
	 * For inline fragments, returns HTML as-is (visibility controlled via DOM).
	 * Also extracts map configuration from map code blocks.
	 */
	let processedHtml = $derived.by(() => {
		let html = slide.html;

		// Extract map config from HTML
		const mapResult = extractMapConfig(html);
		// Update mapConfig state (use queueMicrotask to avoid state change during derivation)
		queueMicrotask(() => {
			if (mapResult.config !== mapConfig) {
				mapConfig = mapResult.config;
				// Update the store to tell navigation about the map
				currentSlideHasMap.set(mapResult.config !== null);
			}
		});
		html = mapResult.html;

		if (hasBlockFragments) {
			return processHtmlWithFragments(html, fragmentsWithVisibility);
		}
		return html;
	});

	// ============================================================================
	// Mermaid Diagram Rendering
	// ============================================================================

	/**
	 * Reference to the slide content element for DOM manipulation.
	 */
	let slideContentElement: HTMLElement | undefined = $state();


	/**
	 * Render mermaid diagrams and highlight code blocks when the slide content is mounted or changes.
	 * This runs after the HTML is inserted into the DOM via {@html}.
	 * Also re-renders when theme changes to apply theme-specific styling.
	 */
	$effect(() => {
		if (slideContentElement && active) {
			// Re-render when processedHtml or theme changes
			// eslint-disable-next-line @typescript-eslint/no-unused-expressions
			processedHtml;
			// eslint-disable-next-line @typescript-eslint/no-unused-expressions
			theme;

			// Use a microtask to ensure DOM has been updated, then process async
			queueMicrotask(async () => {
				try {
					await renderMermaidBlocksInElement(slideContentElement!, theme);
					// Pass the theme to highlighting for theme-appropriate Shiki colors
					await highlightCodeBlocksInElement(slideContentElement!, theme);
				} catch (err) {
					console.error('Error processing slide content:', err);
				}
			});
		}
	});

	/**
	 * Update inline fragment visibility when visibleFragments changes.
	 * This handles fragments where classes are directly on elements (e.g., <li class="fragment">)
	 * rather than wrapped in div containers.
	 */
	$effect(() => {
		if (slideContentElement && hasInlineFragments) {
			// Track visibleFragments to trigger updates
			const currentVisible = visibleFragments;

			// Use microtask to ensure DOM is ready
			queueMicrotask(() => {
				const fragments = slideContentElement!.querySelectorAll('[data-fragment-index]');
				fragments.forEach((el) => {
					const index = parseInt(el.getAttribute('data-fragment-index') || '0', 10);
					if (index <= currentVisible) {
						el.classList.remove('fragment-hidden');
						el.classList.add('fragment-visible');
					} else {
						el.classList.remove('fragment-visible');
						el.classList.add('fragment-hidden');
					}
				});
			});
		}
	});

	// ============================================================================
	// Map Animation Control
	// ============================================================================

	/**
	 * Track previous map animation state to detect changes.
	 */
	let prevMapAnimationTriggered = $state(false);

	/**
	 * React to map animation state changes from the store.
	 * When store says animation is triggered, tell the MapSlide component.
	 * When store says to reset, tell MapSlide to go back to start.
	 */
	$effect(() => {
		if (hasMap && active && mapSlideRef) {
			const currentTriggered = mapAnimationTriggeredFromStore;

			if (currentTriggered && !prevMapAnimationTriggered) {
				// Animation was just triggered - the MapSlide will handle this via its prop
			} else if (!currentTriggered && prevMapAnimationTriggered) {
				// Animation was reset - tell map to go back to start
				mapSlideRef.resetToStart();
			}

			prevMapAnimationTriggered = currentTriggered;
		}
	});

	/**
	 * Reset map state when slide becomes inactive.
	 */
	$effect(() => {
		if (!active && hasMap) {
			currentSlideHasMap.set(false);
		}
	});

	// ============================================================================
	// Scroll Reveal Implementation
	// ============================================================================

	/**
	 * Measure content height and calculate scroll distance.
	 * Uses ResizeObserver to handle window resize.
	 */
	$effect(() => {
		if (!slideContentElement || !hasScrollReveal || !active) {
			scrollDistance = 0;
			scrollMeasured = false;
			return;
		}

		// Function to measure and update scroll distance
		const measureScrollDistance = () => {
			if (!slideContentElement) return;
			const scrollHeight = slideContentElement.scrollHeight;
			const clientHeight = slideContentElement.clientHeight;
			scrollDistance = Math.max(0, scrollHeight - clientHeight);
			scrollMeasured = true;
		};

		// Initial measurement after DOM updates
		queueMicrotask(measureScrollDistance);

		// Set up ResizeObserver for dynamic recalculation
		const resizeObserver = new ResizeObserver(measureScrollDistance);
		resizeObserver.observe(slideContentElement);

		return () => {
			resizeObserver.disconnect();
		};
	});

	/**
	 * Subscribe to scrollRevealed store directly for more reliable updates.
	 */
	let scrollRevealedFromStore = $state(false);

	$effect(() => {
		const unsubscribe = scrollRevealedStore.subscribe((value) => {
			scrollRevealedFromStore = value;
		});
		return unsubscribe;
	});

	/**
	 * Track the last applied scroll position (non-reactive to avoid effect loops).
	 */
	let lastAppliedScrollY: number | null = null;

	/**
	 * Apply scroll transform based on scrollRevealed from store.
	 * Animates smoothly when scroll position changes.
	 */
	$effect(() => {
		if (!slideContentElement || !hasScrollReveal) {
			return;
		}

		// Wait for measurements to be ready before applying any transform
		if (!scrollMeasured) {
			return;
		}

		// Use store value to determine target state
		const isRevealed = scrollRevealedFromStore;

		// Calculate target position
		let targetY = 0;

		// Only scroll down if:
		// 1. scrollRevealed is true
		// 2. Content actually needs scrolling
		if (isRevealed && needsScroll) {
			targetY = scrollDistance;
		}

		// Check if this is the first application or a position change
		const isFirstApplication = lastAppliedScrollY === null;
		const isPositionChange = !isFirstApplication && targetY !== lastAppliedScrollY;

		// Check for reduced motion preference
		const reducedMotion = prefersReducedMotion();

		// Animate only when position actually changes (not on initial load)
		const shouldAnimate = isPositionChange && !reducedMotion;

		if (shouldAnimate) {
			// Set transition first, force reflow, then apply transform
			// This ensures the browser sees the transition before the transform change
			slideContentElement.style.transition = `transform ${scrollSpeed}ms ease-in-out`;
			// Force reflow by reading offsetHeight
			void slideContentElement.offsetHeight;
		} else {
			slideContentElement.style.transition = 'none';
		}

		slideContentElement.style.transform = `translateY(-${targetY}px)`;

		// Update last applied position (direct assignment, not reactive)
		lastAppliedScrollY = targetY;
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
		class="slide-renderer {layoutClass} absolute inset-0 overflow-hidden {hasBlockFragments || hasInlineFragments ? 'has-fragments' : ''} {isFullBleed ? '' : 'p-slide'} {hasScrollReveal ? 'scroll-enabled' : ''} {hasMap ? 'has-map' : ''}"
		style={backgroundStyles}
		in:getTransition
		out:getTransition
	>
		<!-- Map slide (rendered as overlay when map config exists) -->
		{#if mapConfig}
			<MapSlide
				bind:this={mapSlideRef}
				config={mapConfig}
				{active}
				animationTriggered={mapAnimationTriggeredFromStore}
				{isPrintMode}
			/>
		{/if}

		<!-- Regular slide content -->
		<div
			class="slide-content w-full {hasScrollReveal ? 'scroll-content' : 'h-full'} {hasMap ? 'map-content-overlay' : ''}"
			bind:this={slideContentElement}
			style={hasScrollReveal ? `--scroll-speed: ${scrollSpeed}ms` : ''}
		>
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
	 * Centers diagrams and scales them for slide presentation.
	 */
	:global(.mermaid-diagram) {
		display: flex;
		justify-content: center;
		align-items: center;
		width: 100%;
		min-height: 200px;
		margin: 2rem 0;
		overflow: visible;
	}

	:global(.mermaid-diagram svg) {
		max-width: 100%;
		height: auto;
		overflow: visible;
		/* Scale diagrams slightly for better presentation visibility */
		transform: scale(1.1);
		transform-origin: center center;
	}

	/* Ensure mermaid foreignObject content is visible */
	:global(.mermaid-diagram foreignObject) {
		overflow: visible;
	}

	/* Mermaid text styling for better readability */
	:global(.mermaid-diagram .nodeLabel) {
		font-size: 1.1em;
	}

	:global(.mermaid-diagram .edgeLabel) {
		font-size: 1em;
	}

	/*
	 * Mermaid error styles.
	 * Shows a visible error with the original code for debugging.
	 * Uses theme-aware error colors via CSS custom properties.
	 */
	:global(.mermaid-error) {
		padding: 1.5rem 2rem;
		border-radius: 0.75rem;
		background-color: var(--color-error-bg, rgba(220, 38, 38, 0.1));
		border: 2px solid var(--color-error, #dc2626);
		margin: 2rem 0;
	}

	:global(.mermaid-error-message) {
		color: var(--color-error, #dc2626);
		font-weight: 600;
		font-size: 1.25rem;
		margin-bottom: 1rem;
		font-family: var(--font-sans, inherit);
	}

	:global(.mermaid-error-code) {
		font-size: 1rem;
		opacity: 0.9;
		background-color: var(--color-surface, rgba(0, 0, 0, 0.05));
		border-radius: 0.5rem;
		padding: 1.25rem;
		border: 1px solid var(--color-border, rgba(0, 0, 0, 0.1));
	}

	:global(.mermaid-error-code code) {
		white-space: pre-wrap;
		word-break: break-word;
		font-family: var(--font-mono, monospace);
		color: var(--color-text, inherit);
		line-height: 1.6;
	}

	/*
	 * Full-bleed layouts need zero padding to allow content to edge.
	 * This overrides the p-slide Tailwind class.
	 */
	:global(.slide-renderer.layout-split-media),
	:global(.slide-renderer.layout-cover) {
		padding: 0 !important;
	}

	/*
	 * Scroll reveal styles for long content slides.
	 * Uses CSS transform for smooth, performant scrolling animation.
	 */
	:global(.slide-renderer.scroll-enabled) {
		/* Clip content at container bounds */
		overflow: hidden;
	}

	:global(.slide-renderer.scroll-enabled .scroll-content) {
		/* Allow content to extend beyond viewport */
		min-height: 100%;
		/* Transform is applied via JavaScript for smooth animation */
		will-change: transform;
	}

	/* Print mode: show all content without scroll truncation */
	@media print {
		:global(.slide-renderer.scroll-enabled) {
			overflow: visible;
		}

		:global(.slide-renderer.scroll-enabled .scroll-content) {
			transform: none !important;
			transition: none !important;
		}
	}

	/*
	 * Map slide styles.
	 * Maps render as full-slide backgrounds with optional content overlay.
	 */
	:global(.slide-renderer.has-map) {
		/* Ensure map fills the slide */
		padding: 0 !important;
	}

	:global(.slide-renderer.has-map .map-content-overlay) {
		/* Content overlays on top of map */
		position: relative;
		z-index: 10;
		pointer-events: none;
	}

	:global(.slide-renderer.has-map .map-content-overlay:empty) {
		/* Hide empty content overlay */
		display: none;
	}

	:global(.slide-renderer.has-map .map-content-overlay > *) {
		/* Re-enable pointer events on actual content */
		pointer-events: auto;
	}
</style>
