<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { Slide, Theme } from '$lib/types';
	import { currentSlideIndex, goToSlide } from '$lib/stores/presentation';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** Array of slides to display as thumbnails */
		slides: Slide[];
		/** Current theme name */
		theme?: Theme;
		/** Whether the overview is currently visible */
		isOpen?: boolean;
		/** Callback when a slide is selected */
		onSelect?: (index: number) => void;
		/** Callback when the overview should be closed */
		onClose?: () => void;
	}

	let {
		slides,
		theme = 'paper' as Theme,
		isOpen = false,
		onSelect,
		onClose
	}: Props = $props();

	// ============================================================================
	// Background Helpers
	// ============================================================================

	function getBackgroundStyle(slide: Slide): string {
		if (!slide.background) return '';
		const bg = slide.background;
		switch (bg.type) {
			case 'color':
				return `background-color: ${bg.value};`;
			case 'image':
				return `background-image: url('${bg.value}'); background-size: cover; background-position: center;`;
			case 'gradient':
				return `background: ${bg.value};`;
			default:
				return '';
		}
	}

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
	const GRID_COLUMNS = 5;

	/** Base slide dimensions for scaling */
	const SLIDE_WIDTH = 1920;

	/** Track thumbnail scale factor */
	let thumbnailScale = $state(0.15);
	let gridRef: HTMLDivElement | undefined = $state();

	function updateThumbnailScale(): void {
		if (!gridRef) return;
		const firstThumb = gridRef.querySelector('.thumbnail') as HTMLElement | null;
		if (firstThumb) {
			thumbnailScale = firstThumb.clientWidth / SLIDE_WIDTH;
		}
	}

	let resizeObserver: ResizeObserver | undefined;

	$effect(() => {
		if (isOpen && gridRef) {
			// Wait for layout to settle before measuring
			requestAnimationFrame(() => updateThumbnailScale());
			resizeObserver = new ResizeObserver(() => updateThumbnailScale());
			resizeObserver.observe(gridRef);
		}
		return () => {
			resizeObserver?.disconnect();
		};
	});

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
		class="slide-overview"
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
			<div
				class="thumbnail-grid"
				style:--grid-columns={GRID_COLUMNS}
				role="listbox"
				aria-label="Select a slide"
				bind:this={gridRef}
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
						<div class="thumbnail-aspect theme-{theme}">
							<div
								class="thumbnail-content slide-renderer layout-{slide.layout}"
								style="transform: scale({thumbnailScale}); {getBackgroundStyle(slide)}"
							>
								{@html slide.html}
							</div>
						</div>
						<div class="thumbnail-number">
							{index + 1}
						</div>
					</button>
				{/each}
			</div>
		</div>
	</div>
{/if}
