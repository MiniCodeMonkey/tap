<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { Presentation, Slide } from '$lib/types';
	import SlideContainer from '$lib/components/SlideContainer.svelte';
	import SlideRenderer from '$lib/components/SlideRenderer.svelte';
	import {
		presentation,
		currentSlideIndex,
		currentFragmentIndex,
		currentSlide,
		totalSlides,
		totalFragments,
		nextSlide,
		prevSlide,
		goToSlide,
		loadPresentation
	} from '$lib/stores/presentation';
	import {
		connected,
		connectWebSocket,
		disconnectWebSocket,
		getWebSocketClient
	} from '$lib/stores/websocket';

	// ============================================================================
	// State
	// ============================================================================

	let elapsedSeconds = $state(0);
	let timerInterval: ReturnType<typeof setInterval> | null = null;
	let presentationData = $state<Presentation | null>(null);
	let slideIndex = $state(0);
	let fragmentIndex = $state(-1);
	let slideCount = $state(0);
	let fragmentCount = $state(0);
	let slide = $state<Slide | null>(null);
	let isConnected = $state(false);

	// ============================================================================
	// Derived State
	// ============================================================================

	let nextSlideData = $derived.by(() => {
		if (!presentationData || slideIndex >= presentationData.slides.length - 1) {
			return null;
		}
		return presentationData.slides[slideIndex + 1] ?? null;
	});

	let theme = $derived(presentationData?.config?.theme ?? 'minimal');
	let aspectRatio = $derived(presentationData?.config?.aspectRatio ?? '16:9');
	let speakerNotes = $derived(slide?.notes ?? '');
	let hasNotes = $derived(speakerNotes.length > 0);

	// ============================================================================
	// Timer Functions
	// ============================================================================

	function startTimer(): void {
		if (timerInterval) return;
		timerInterval = setInterval(() => {
			elapsedSeconds++;
		}, 1000);
	}

	function stopTimer(): void {
		if (timerInterval) {
			clearInterval(timerInterval);
			timerInterval = null;
		}
	}

	function resetTimer(): void {
		elapsedSeconds = 0;
	}

	function formatTime(seconds: number): string {
		const hours = Math.floor(seconds / 3600);
		const minutes = Math.floor((seconds % 3600) / 60);
		const secs = seconds % 60;

		if (hours > 0) {
			return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
		}
		return `${minutes}:${secs.toString().padStart(2, '0')}`;
	}

	let formattedTime = $derived(formatTime(elapsedSeconds));

	// ============================================================================
	// Navigation Functions
	// ============================================================================

	function handleNextSlide(): void {
		nextSlide();
		broadcastSlide();
	}

	function handlePrevSlide(): void {
		prevSlide();
		broadcastSlide();
	}

	function handleGoToSlide(index: number): void {
		goToSlide(index);
		broadcastSlide();
	}

	function broadcastSlide(): void {
		const client = getWebSocketClient();
		// Read the current slide index from the store
		let currentIndex = 0;
		const unsubscribe = currentSlideIndex.subscribe((value) => {
			currentIndex = value;
		});
		unsubscribe();

		client.send({
			type: 'slide',
			slideIndex: currentIndex
		});
	}

	// ============================================================================
	// Keyboard Navigation
	// ============================================================================

	function handleKeydown(event: KeyboardEvent): void {
		// Skip if focused on input element
		const target = event.target as HTMLElement;
		if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
			return;
		}

		switch (event.key) {
			case 'ArrowRight':
			case 'ArrowDown':
			case ' ':
			case 'Enter':
				event.preventDefault();
				handleNextSlide();
				break;
			case 'ArrowLeft':
			case 'ArrowUp':
			case 'Backspace':
				event.preventDefault();
				handlePrevSlide();
				break;
			case 'Home':
				event.preventDefault();
				handleGoToSlide(0);
				break;
			case 'End':
				event.preventDefault();
				if (presentationData) {
					handleGoToSlide(presentationData.slides.length - 1);
				}
				break;
			case 'r':
			case 'R':
				event.preventDefault();
				resetTimer();
				break;
		}
	}

	// ============================================================================
	// API Fetch
	// ============================================================================

	async function fetchPresentation(): Promise<void> {
		try {
			const response = await fetch('/api/presentation');
			if (!response.ok) {
				throw new Error(`HTTP error: ${response.status}`);
			}
			const data = (await response.json()) as Presentation;
			loadPresentation(data);
		} catch (error) {
			console.error('Failed to fetch presentation:', error);
		}
	}

	// ============================================================================
	// Store Subscriptions
	// ============================================================================

	let unsubscribers: (() => void)[] = [];

	function setupSubscriptions(): void {
		unsubscribers.push(
			presentation.subscribe((value) => {
				presentationData = value;
			})
		);

		unsubscribers.push(
			currentSlideIndex.subscribe((value) => {
				slideIndex = value;
			})
		);

		unsubscribers.push(
			currentFragmentIndex.subscribe((value) => {
				fragmentIndex = value;
			})
		);

		unsubscribers.push(
			totalSlides.subscribe((value) => {
				slideCount = value;
			})
		);

		unsubscribers.push(
			totalFragments.subscribe((value) => {
				fragmentCount = value;
			})
		);

		unsubscribers.push(
			currentSlide.subscribe((value) => {
				slide = value;
			})
		);

		unsubscribers.push(
			connected.subscribe((value) => {
				isConnected = value;
			})
		);
	}

	function cleanupSubscriptions(): void {
		unsubscribers.forEach((unsub) => unsub());
		unsubscribers = [];
	}

	// ============================================================================
	// Lifecycle
	// ============================================================================

	onMount(() => {
		setupSubscriptions();
		fetchPresentation();
		connectWebSocket();
		startTimer();

		window.addEventListener('keydown', handleKeydown);
	});

	onDestroy(() => {
		cleanupSubscriptions();
		stopTimer();
		disconnectWebSocket();

		if (typeof window !== 'undefined') {
			window.removeEventListener('keydown', handleKeydown);
		}
	});
</script>

<div class="presenter-view">
	<!-- Header with timer and slide counter -->
	<header class="presenter-header">
		<div class="presenter-slide-counter">
			<span class="current">{slideIndex + 1}</span>
			<span class="separator">/</span>
			<span class="total">{slideCount}</span>
			{#if fragmentCount > 0}
				<span class="fragment-counter">
					({fragmentIndex + 1}/{fragmentCount})
				</span>
			{/if}
		</div>

		<button
			class="presenter-timer"
			onclick={resetTimer}
			title="Click to reset timer"
			aria-label="Elapsed time: {formattedTime}. Click to reset."
		>
			{formattedTime}
		</button>

		<div class="presenter-connection-status" class:connected={isConnected}>
			{isConnected ? 'Connected' : 'Disconnected'}
		</div>
	</header>

	<!-- Main content area -->
	<main class="presenter-main">
		<!-- Current slide (compact) -->
		<div class="presenter-current-slide-panel">
			<h2 class="presenter-panel-title">Current Slide</h2>
			<div class="presenter-slide-preview current">
				{#if presentationData && slide}
					<SlideContainer {aspectRatio} {theme}>
						<SlideRenderer
							{slide}
							visibleFragments={fragmentIndex}
							active={true}
						/>
					</SlideContainer>
				{:else}
					<div class="presenter-loading-placeholder">Loading...</div>
				{/if}
			</div>
		</div>

		<!-- Next slide preview -->
		<div class="presenter-next-slide-panel">
			<h2 class="presenter-panel-title">Next Slide</h2>
			<div class="presenter-slide-preview next">
				{#if presentationData && nextSlideData}
					<SlideContainer {aspectRatio} {theme}>
						<SlideRenderer
							slide={nextSlideData}
							visibleFragments={-1}
							active={true}
						/>
					</SlideContainer>
				{:else if presentationData}
					<div class="presenter-end-placeholder">End of Presentation</div>
				{:else}
					<div class="presenter-loading-placeholder">Loading...</div>
				{/if}
			</div>
		</div>

		<!-- Speaker notes -->
		<div class="presenter-notes-panel" class:has-notes={hasNotes}>
			<h2 class="presenter-panel-title">Speaker Notes</h2>
			<div class="presenter-notes-content">
				{#if hasNotes}
					{@html speakerNotes}
				{:else}
					<p class="presenter-no-notes">No speaker notes for this slide.</p>
				{/if}
			</div>
		</div>
	</main>

	<!-- Touch-friendly navigation controls -->
	<footer class="presenter-controls">
		<button
			class="presenter-control-button prev"
			onclick={handlePrevSlide}
			disabled={slideIndex === 0 && fragmentIndex < 0}
			aria-label="Previous slide"
		>
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<polyline points="15 18 9 12 15 6"></polyline>
			</svg>
			<span>Previous</span>
		</button>

		<div class="presenter-control-info">
			<span class="presenter-keyboard-hint">Use arrow keys or space to navigate</span>
			<span class="presenter-keyboard-hint">Press R to reset timer</span>
		</div>

		<button
			class="presenter-control-button next"
			onclick={handleNextSlide}
			disabled={slideIndex === slideCount - 1 && fragmentIndex >= fragmentCount - 1}
			aria-label="Next slide"
		>
			<span>Next</span>
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<polyline points="9 18 15 12 9 6"></polyline>
			</svg>
		</button>
	</footer>
</div>

