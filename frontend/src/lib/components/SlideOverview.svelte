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
	}

	let {
		slides,
		isOpen = false,
		onSelect,
		onClose
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
