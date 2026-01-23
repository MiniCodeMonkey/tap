<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { Slide } from '$lib/types';
	import { currentSlideIndex, goToSlide } from '$lib/stores/presentation';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** Array of slides to display as thumbnails */
		slides: Slide[];
		/** Whether the overview is currently visible */
		isOpen?: boolean;
		/** Callback when a slide is selected */
		onSelect?: (index: number) => void;
		/** Callback when the overview should be closed */
		onClose?: () => void;
		/** Theme name for styling */
		theme?: string;
	}

	let {
		slides,
		isOpen = false,
		onSelect,
		onClose,
		theme = 'minimal'
	}: Props = $props();

	// ============================================================================
	// State
	// ============================================================================

	/** Currently focused slide index in the grid (for keyboard navigation) */
	let focusedIndex = $state(0);

	/** Reference to the container for focus management */
	let containerRef: HTMLDivElement | undefined = $state();

	// ============================================================================
	// Store Subscriptions
	// ============================================================================

	let unsubscribe: (() => void) | undefined;
	let currentIndex = $state(0);

	// Sync focused index with current slide when opening
	$effect(() => {
		if (isOpen) {
			focusedIndex = currentIndex;
		}
	});

	onMount(() => {
		unsubscribe = currentSlideIndex.subscribe((value) => {
			currentIndex = value;
		});
	});

	onDestroy(() => {
		if (unsubscribe) {
			unsubscribe();
		}
	});

	// ============================================================================
	// Grid Configuration
	// ============================================================================

	/** Number of columns in the grid */
	const GRID_COLUMNS = 4;

	// ============================================================================
	// Computed Values
	// ============================================================================


	// ============================================================================
	// Navigation Functions
	// ============================================================================

	/**
	 * Move focus to the next slide thumbnail.
	 */
	function focusNext(): void {
		if (focusedIndex < slides.length - 1) {
			focusedIndex += 1;
		}
	}

	/**
	 * Move focus to the previous slide thumbnail.
	 */
	function focusPrev(): void {
		if (focusedIndex > 0) {
			focusedIndex -= 1;
		}
	}

	/**
	 * Move focus down one row.
	 */
	function focusDown(): void {
		const nextIndex = focusedIndex + GRID_COLUMNS;
		if (nextIndex < slides.length) {
			focusedIndex = nextIndex;
		}
	}

	/**
	 * Move focus up one row.
	 */
	function focusUp(): void {
		const prevIndex = focusedIndex - GRID_COLUMNS;
		if (prevIndex >= 0) {
			focusedIndex = prevIndex;
		}
	}

	/**
	 * Select the currently focused slide and close overview.
	 */
	function selectFocused(): void {
		selectSlide(focusedIndex);
	}

	/**
	 * Select a specific slide and close the overview.
	 */
	function selectSlide(index: number): void {
		goToSlide(index);
		if (onSelect) {
			onSelect(index);
		}
		if (onClose) {
			onClose();
		}
	}

	// ============================================================================
	// Keyboard Handler
	// ============================================================================

	function handleKeyDown(event: KeyboardEvent): void {
		if (!isOpen) return;

		switch (event.key) {
			case 'ArrowRight':
				event.preventDefault();
				focusNext();
				break;
			case 'ArrowLeft':
				event.preventDefault();
				focusPrev();
				break;
			case 'ArrowDown':
				event.preventDefault();
				focusDown();
				break;
			case 'ArrowUp':
				event.preventDefault();
				focusUp();
				break;
			case 'Enter':
			case ' ':
				event.preventDefault();
				selectFocused();
				break;
			case 'Escape':
				event.preventDefault();
				if (onClose) {
					onClose();
				}
				break;
			case 'Home':
				event.preventDefault();
				focusedIndex = 0;
				break;
			case 'End':
				event.preventDefault();
				focusedIndex = slides.length - 1;
				break;
		}
	}

	// ============================================================================
	// Lifecycle & Effects
	// ============================================================================

	// Focus the container when opened
	$effect(() => {
		if (isOpen && containerRef) {
			containerRef.focus();
		}
	});

	// Set up keyboard listener when open
	onMount(() => {
		// The component handles its own keyboard events via the div's onkeydown
	});
</script>

{#if isOpen}
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<div
		class="slide-overview theme-{theme}"
		role="dialog"
		aria-label="Slide overview"
		aria-modal="true"
		tabindex="0"
		bind:this={containerRef}
		onkeydown={handleKeyDown}
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div class="overview-backdrop" role="presentation" onclick={() => onClose?.()}></div>

		<div class="overview-content">
			<div class="overview-header">
				<h2>Slide Overview</h2>
				<button class="close-button" onclick={() => onClose?.()} aria-label="Close overview">
					<span aria-hidden="true">&times;</span>
				</button>
			</div>

			<div
				class="thumbnail-grid"
				style:--grid-columns={GRID_COLUMNS}
				role="listbox"
				aria-label="Select a slide"
			>
				{#each slides as slide, index}
					<button
						class="thumbnail"
						class:current={index === currentIndex}
						class:focused={index === focusedIndex}
						onclick={() => selectSlide(index)}
						role="option"
						aria-selected={index === currentIndex}
						aria-label="Slide {index + 1}"
					>
						<div class="thumbnail-content layout-{slide.layout}">
							{@html slide.html}
						</div>
						<div class="thumbnail-number">
							{index + 1}
						</div>
					</button>
				{/each}
			</div>

			<div class="overview-footer">
				<span class="keyboard-hint">
					Use arrow keys to navigate, Enter to select, Escape to close
				</span>
			</div>
		</div>
	</div>
{/if}

<style>
	.slide-overview {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		z-index: 10000;
		display: flex;
		align-items: center;
		justify-content: center;
		outline: none;
	}

	.overview-backdrop {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background-color: rgba(0, 0, 0, 0.85);
		backdrop-filter: blur(4px);
	}

	.overview-content {
		position: relative;
		z-index: 1;
		width: 90%;
		max-width: 1400px;
		max-height: 90vh;
		display: flex;
		flex-direction: column;
		background-color: var(--overview-bg, #1a1a1a);
		border-radius: 12px;
		box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
		overflow: hidden;
	}

	.overview-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem 1.5rem;
		border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
	}

	.overview-header h2 {
		margin: 0;
		font-size: 1.25rem;
		font-weight: 600;
		color: var(--header-text, #fff);
	}

	.close-button {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 2rem;
		height: 2rem;
		padding: 0;
		background: transparent;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		color: var(--header-text, #fff);
		font-size: 1.5rem;
		line-height: 1;
		transition: background-color 0.2s ease;
	}

	.close-button:hover {
		background-color: rgba(255, 255, 255, 0.1);
	}

	.close-button:focus {
		outline: 2px solid var(--focus-ring, #7c3aed);
		outline-offset: 2px;
	}

	.thumbnail-grid {
		display: grid;
		grid-template-columns: repeat(var(--grid-columns, 4), 1fr);
		gap: 1rem;
		padding: 1.5rem;
		overflow-y: auto;
		flex: 1;
	}

	.thumbnail {
		position: relative;
		aspect-ratio: 16 / 9;
		border: 2px solid transparent;
		border-radius: 8px;
		overflow: hidden;
		cursor: pointer;
		background-color: var(--thumbnail-bg, #fff);
		transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
		padding: 0;
	}

	.thumbnail:hover {
		transform: scale(1.02);
		border-color: var(--hover-border, rgba(124, 58, 237, 0.5));
	}

	.thumbnail:focus {
		outline: none;
	}

	.thumbnail.focused {
		border-color: var(--focus-border, #7c3aed);
		box-shadow: 0 0 0 3px var(--focus-ring, rgba(124, 58, 237, 0.3));
	}

	.thumbnail.current {
		border-color: var(--current-border, #10b981);
	}

	.thumbnail.current.focused {
		border-color: var(--current-focus-border, #10b981);
		box-shadow: 0 0 0 3px var(--current-focus-ring, rgba(16, 185, 129, 0.3));
	}

	.thumbnail-content {
		width: 100%;
		height: 100%;
		transform: scale(0.15);
		transform-origin: top left;
		width: calc(100% / 0.15);
		height: calc(100% / 0.15);
		pointer-events: none;
		overflow: hidden;
		padding: 80px;
		box-sizing: border-box;
		font-size: 32px;
	}

	.thumbnail-number {
		position: absolute;
		bottom: 0.5rem;
		right: 0.5rem;
		background-color: rgba(0, 0, 0, 0.7);
		color: #fff;
		font-size: 0.75rem;
		font-weight: 600;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		min-width: 1.5rem;
		text-align: center;
	}

	.current .thumbnail-number {
		background-color: var(--current-badge, #10b981);
	}

	.overview-footer {
		padding: 0.75rem 1.5rem;
		border-top: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
		text-align: center;
	}

	.keyboard-hint {
		font-size: 0.875rem;
		color: var(--muted-text, rgba(255, 255, 255, 0.5));
	}

	/* Theme-specific overrides */
	:global(.theme-minimal) .overview-content {
		--overview-bg: #1a1a1a;
		--header-text: #fff;
		--border-color: rgba(255, 255, 255, 0.1);
		--thumbnail-bg: #fff;
		--focus-border: #7c3aed;
		--focus-ring: rgba(124, 58, 237, 0.3);
		--current-border: #10b981;
		--current-badge: #10b981;
		--muted-text: rgba(255, 255, 255, 0.5);
	}

	:global(.theme-terminal) .overview-content {
		--overview-bg: #0a0a0a;
		--header-text: #00ff00;
		--border-color: rgba(0, 255, 0, 0.2);
		--thumbnail-bg: #0d1117;
		--focus-border: #00ff00;
		--focus-ring: rgba(0, 255, 0, 0.3);
		--current-border: #00ff00;
		--current-badge: #00ff00;
		--muted-text: rgba(0, 255, 0, 0.5);
	}

	:global(.theme-gradient) .overview-content {
		--overview-bg: rgba(15, 15, 35, 0.95);
		--header-text: #fff;
		--border-color: rgba(255, 255, 255, 0.1);
		--thumbnail-bg: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
		--focus-border: #fbbf24;
		--focus-ring: rgba(251, 191, 36, 0.3);
		--current-border: #10b981;
		--current-badge: #10b981;
		--muted-text: rgba(255, 255, 255, 0.5);
	}

	:global(.theme-brutalist) .overview-content {
		--overview-bg: #000;
		--header-text: #fff;
		--border-color: #fff;
		--thumbnail-bg: #fff;
		--focus-border: #ff0000;
		--focus-ring: rgba(255, 0, 0, 0.3);
		--current-border: #ff0000;
		--current-badge: #ff0000;
		--muted-text: rgba(255, 255, 255, 0.5);
	}

	:global(.theme-keynote) .overview-content {
		--overview-bg: #1a1a1a;
		--header-text: #fff;
		--border-color: rgba(255, 255, 255, 0.1);
		--thumbnail-bg: #fff;
		--focus-border: #007aff;
		--focus-ring: rgba(0, 122, 255, 0.3);
		--current-border: #34c759;
		--current-badge: #34c759;
		--muted-text: rgba(255, 255, 255, 0.5);
	}

	/* Reduced motion support */
	@media (prefers-reduced-motion: reduce) {
		.thumbnail {
			transition: none;
		}
	}

	/* Responsive adjustments */
	@media (max-width: 1024px) {
		.thumbnail-grid {
			--grid-columns: 3;
		}
	}

	@media (max-width: 768px) {
		.thumbnail-grid {
			--grid-columns: 2;
		}

		.keyboard-hint {
			display: none;
		}
	}

	@media (max-width: 480px) {
		.thumbnail-grid {
			--grid-columns: 1;
		}

		.overview-content {
			width: 95%;
			max-height: 95vh;
		}
	}
</style>
