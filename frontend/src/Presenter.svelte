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
		<div class="slide-counter">
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
			class="timer"
			onclick={resetTimer}
			title="Click to reset timer"
			aria-label="Elapsed time: {formattedTime}. Click to reset."
		>
			{formattedTime}
		</button>

		<div class="connection-status" class:connected={isConnected}>
			{isConnected ? 'Connected' : 'Disconnected'}
		</div>
	</header>

	<!-- Main content area -->
	<main class="presenter-main">
		<!-- Current slide (compact) -->
		<div class="current-slide-panel">
			<h2 class="panel-title">Current Slide</h2>
			<div class="slide-preview current">
				{#if presentationData && slide}
					<SlideContainer {aspectRatio} {theme}>
						<SlideRenderer
							{slide}
							visibleFragments={fragmentIndex}
							active={true}
						/>
					</SlideContainer>
				{:else}
					<div class="loading-placeholder">Loading...</div>
				{/if}
			</div>
		</div>

		<!-- Next slide preview -->
		<div class="next-slide-panel">
			<h2 class="panel-title">Next Slide</h2>
			<div class="slide-preview next">
				{#if presentationData && nextSlideData}
					<SlideContainer {aspectRatio} {theme}>
						<SlideRenderer
							slide={nextSlideData}
							visibleFragments={-1}
							active={true}
						/>
					</SlideContainer>
				{:else if presentationData}
					<div class="end-placeholder">End of Presentation</div>
				{:else}
					<div class="loading-placeholder">Loading...</div>
				{/if}
			</div>
		</div>

		<!-- Speaker notes -->
		<div class="notes-panel" class:has-notes={hasNotes}>
			<h2 class="panel-title">Speaker Notes</h2>
			<div class="notes-content">
				{#if hasNotes}
					{@html speakerNotes}
				{:else}
					<p class="no-notes">No speaker notes for this slide.</p>
				{/if}
			</div>
		</div>
	</main>

	<!-- Touch-friendly navigation controls -->
	<footer class="presenter-controls">
		<button
			class="control-button prev"
			onclick={handlePrevSlide}
			disabled={slideIndex === 0 && fragmentIndex < 0}
			aria-label="Previous slide"
		>
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<polyline points="15 18 9 12 15 6"></polyline>
			</svg>
			<span>Previous</span>
		</button>

		<div class="control-info">
			<span class="keyboard-hint">Use arrow keys or space to navigate</span>
			<span class="keyboard-hint">Press R to reset timer</span>
		</div>

		<button
			class="control-button next"
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

<style>
	.presenter-view {
		display: grid;
		grid-template-rows: auto 1fr auto;
		height: 100vh;
		width: 100vw;
		background-color: #1a1a2e;
		color: #eaeaea;
		font-family: system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
		overflow: hidden;
	}

	/* ========================================================================
	 * Header
	 * ======================================================================== */

	.presenter-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem 2rem;
		background-color: #16213e;
		border-bottom: 1px solid #0f3460;
	}

	.slide-counter {
		font-size: 1.5rem;
		font-weight: 600;
	}

	.slide-counter .current {
		color: #e94560;
		font-size: 2rem;
	}

	.slide-counter .separator {
		color: #666;
		margin: 0 0.25rem;
	}

	.slide-counter .total {
		color: #888;
	}

	.slide-counter .fragment-counter {
		font-size: 1rem;
		color: #666;
		margin-left: 0.5rem;
	}

	.timer {
		font-size: 2.5rem;
		font-weight: 700;
		font-family: 'SF Mono', Monaco, Consolas, monospace;
		background: transparent;
		border: none;
		color: #4ecca3;
		cursor: pointer;
		padding: 0.5rem 1rem;
		border-radius: 8px;
		transition: background-color 0.2s, transform 0.1s;
	}

	.timer:hover {
		background-color: rgba(78, 204, 163, 0.1);
	}

	.timer:active {
		transform: scale(0.98);
	}

	.connection-status {
		padding: 0.5rem 1rem;
		border-radius: 9999px;
		font-size: 0.875rem;
		font-weight: 500;
		background-color: rgba(255, 82, 82, 0.2);
		color: #ff5252;
	}

	.connection-status.connected {
		background-color: rgba(78, 204, 163, 0.2);
		color: #4ecca3;
	}

	/* ========================================================================
	 * Main Content
	 * ======================================================================== */

	.presenter-main {
		display: grid;
		grid-template-columns: 2fr 1fr;
		grid-template-rows: 1fr 1fr;
		gap: 1rem;
		padding: 1rem;
		overflow: hidden;
	}

	.panel-title {
		font-size: 0.875rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: #888;
		margin: 0 0 0.75rem 0;
	}

	/* Current slide panel */
	.current-slide-panel {
		grid-row: 1 / 3;
		display: flex;
		flex-direction: column;
	}

	/* Next slide panel */
	.next-slide-panel {
		display: flex;
		flex-direction: column;
	}

	/* Notes panel */
	.notes-panel {
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.slide-preview {
		flex: 1;
		min-height: 0;
		background-color: #0f0f23;
		border-radius: 12px;
		overflow: hidden;
		border: 2px solid #0f3460;
		transition: border-color 0.2s;
	}

	.slide-preview.current {
		border-color: #e94560;
	}

	.slide-preview.next {
		opacity: 0.85;
	}

	.loading-placeholder,
	.end-placeholder {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: #666;
		font-size: 1.25rem;
	}

	.end-placeholder {
		background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
		color: #4ecca3;
		font-weight: 500;
	}

	.notes-content {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
		padding: 1rem;
		background-color: #0f0f23;
		border-radius: 12px;
		border: 2px solid #0f3460;
		font-size: 1.25rem;
		line-height: 1.6;
	}

	.notes-panel.has-notes .notes-content {
		border-color: #0f3460;
	}

	.notes-content :global(p) {
		margin: 0 0 1em;
	}

	.notes-content :global(ul),
	.notes-content :global(ol) {
		margin: 0 0 1em;
		padding-left: 1.5em;
	}

	.notes-content :global(li) {
		margin-bottom: 0.5em;
	}

	.notes-content :global(strong) {
		color: #e94560;
	}

	.notes-content :global(em) {
		color: #4ecca3;
	}

	.no-notes {
		color: #666;
		font-style: italic;
	}

	/* ========================================================================
	 * Controls Footer
	 * ======================================================================== */

	.presenter-controls {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem 2rem;
		background-color: #16213e;
		border-top: 1px solid #0f3460;
	}

	.control-button {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 1rem 2rem;
		font-size: 1.125rem;
		font-weight: 600;
		background-color: #0f3460;
		border: none;
		border-radius: 12px;
		color: #eaeaea;
		cursor: pointer;
		transition: background-color 0.2s, transform 0.1s, opacity 0.2s;
		min-width: 160px;
		justify-content: center;
	}

	.control-button svg {
		width: 24px;
		height: 24px;
	}

	.control-button:hover:not(:disabled) {
		background-color: #1a4a7a;
	}

	.control-button:active:not(:disabled) {
		transform: scale(0.98);
	}

	.control-button:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.control-button.next {
		background-color: #e94560;
	}

	.control-button.next:hover:not(:disabled) {
		background-color: #ff6b8a;
	}

	.control-info {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.25rem;
	}

	.keyboard-hint {
		font-size: 0.75rem;
		color: #666;
	}

	/* ========================================================================
	 * Touch-friendly sizing (for tablets)
	 * ======================================================================== */

	@media (max-width: 1024px) {
		.presenter-main {
			grid-template-columns: 1fr 1fr;
			grid-template-rows: 1fr auto auto;
		}

		.current-slide-panel {
			grid-row: 1;
			grid-column: 1 / 3;
		}

		.next-slide-panel {
			grid-row: 2;
			grid-column: 1;
		}

		.notes-panel {
			grid-row: 2;
			grid-column: 2;
		}

		.control-button {
			padding: 1.25rem 2rem;
			font-size: 1.25rem;
			min-width: 180px;
		}

		.control-button svg {
			width: 28px;
			height: 28px;
		}
	}

	@media (max-width: 768px) {
		.presenter-header {
			padding: 0.75rem 1rem;
		}

		.slide-counter {
			font-size: 1.25rem;
		}

		.slide-counter .current {
			font-size: 1.5rem;
		}

		.timer {
			font-size: 1.75rem;
		}

		.presenter-main {
			grid-template-columns: 1fr;
			grid-template-rows: 1fr auto auto;
			padding: 0.75rem;
			gap: 0.75rem;
		}

		.current-slide-panel {
			grid-column: 1;
		}

		.next-slide-panel,
		.notes-panel {
			grid-column: 1;
		}

		.notes-content {
			font-size: 1rem;
			max-height: 150px;
		}

		.presenter-controls {
			padding: 0.75rem 1rem;
		}

		.control-button {
			padding: 1rem 1.5rem;
			font-size: 1rem;
			min-width: 140px;
		}

		.control-info {
			display: none;
		}
	}

	/* ========================================================================
	 * Reduced Motion
	 * ======================================================================== */

	@media (prefers-reduced-motion: reduce) {
		.timer,
		.control-button,
		.slide-preview {
			transition: none;
		}
	}
</style>
