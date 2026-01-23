<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { Presentation, Slide } from '$lib/types';
	import {
		presentation,
		currentSlide,
		currentSlideIndex,
		currentFragmentIndex,
		loadPresentation,
		setupHashChangeListener
	} from '$lib/stores/presentation';
	import {
		connectWebSocket,
		disconnectWebSocket,
		detectStaticMode
	} from '$lib/stores/websocket';
	import { setupKeyboardNavigation } from '$lib/utils/keyboard';
	import SlideContainer from '$lib/components/SlideContainer.svelte';
	import SlideRenderer from '$lib/components/SlideRenderer.svelte';
	import ProgressBar from '$lib/components/ProgressBar.svelte';
	import SlideOverview from '$lib/components/SlideOverview.svelte';
	import ConnectionIndicator from '$lib/components/ConnectionIndicator.svelte';

	// ============================================================================
	// State
	// ============================================================================

	let isLoading = $state(true);
	let loadError = $state<string | null>(null);
	let isOverviewOpen = $state(false);
	let direction = $state<'forward' | 'backward'>('forward');

	// Store values
	let presentationData = $state<Presentation | null>(null);
	let currentSlideData = $state<Slide | null>(null);
	let slideIndex = $state(0);
	let fragmentIndex = $state(-1);
	let slides = $state<Slide[]>([]);

	// ============================================================================
	// Derived Values
	// ============================================================================

	let theme = $derived(presentationData?.config?.theme ?? 'minimal');
	let aspectRatio = $derived(presentationData?.config?.aspectRatio ?? '16:9');
	let showProgressBar = $derived(presentationData?.config?.showProgressBar !== false);

	// ============================================================================
	// Store Subscriptions
	// ============================================================================

	let unsubscribers: (() => void)[] = [];

	// ============================================================================
	// Fetch Presentation
	// ============================================================================

	async function fetchPresentation(): Promise<void> {
		try {
			// First check for embedded data (static build)
			const embeddedScript = document.getElementById('presentation-data');
			if (embeddedScript) {
				try {
					const data = JSON.parse(embeddedScript.textContent || '{}') as Presentation;
					if (data.slides && data.slides.length > 0) {
						loadPresentation(data);
						isLoading = false;
						return;
					}
				} catch {
					// Fall through to API fetch
				}
			}

			// Fetch from API
			const response = await fetch('/api/presentation');
			if (!response.ok) {
				throw new Error(`Failed to load presentation: ${response.statusText}`);
			}
			const data = (await response.json()) as Presentation;
			loadPresentation(data);
			isLoading = false;
		} catch (error) {
			loadError = error instanceof Error ? error.message : 'Failed to load presentation';
			isLoading = false;
		}
	}

	// ============================================================================
	// Overview Handlers
	// ============================================================================

	function toggleOverview(): void {
		isOverviewOpen = !isOverviewOpen;
	}

	function closeOverview(): void {
		isOverviewOpen = false;
	}

	function isOverviewOpenFn(): boolean {
		return isOverviewOpen;
	}

	// ============================================================================
	// Lifecycle
	// ============================================================================

	onMount(async () => {
		// Set up store subscriptions
		unsubscribers.push(
			presentation.subscribe((value) => {
				presentationData = value;
				slides = value?.slides ?? [];
			})
		);

		unsubscribers.push(
			currentSlide.subscribe((value) => {
				currentSlideData = value;
			})
		);

		unsubscribers.push(
			currentSlideIndex.subscribe((value) => {
				// Track direction for transitions
				if (value > slideIndex) {
					direction = 'forward';
				} else if (value < slideIndex) {
					direction = 'backward';
				}
				slideIndex = value;
			})
		);

		unsubscribers.push(
			currentFragmentIndex.subscribe((value) => {
				fragmentIndex = value;
			})
		);

		// Set up hash change listener
		const hashCleanup = setupHashChangeListener();
		unsubscribers.push(hashCleanup);

		// Set up keyboard navigation
		const keyboardCleanup = setupKeyboardNavigation({
			onToggleOverview: toggleOverview,
			isOverviewOpen: isOverviewOpenFn
		});
		unsubscribers.push(keyboardCleanup);

		// Detect static mode
		await detectStaticMode();

		// Connect WebSocket (will handle reconnection automatically)
		connectWebSocket();

		// Fetch presentation data
		await fetchPresentation();
	});

	onDestroy(() => {
		// Clean up all subscriptions
		unsubscribers.forEach((unsub) => unsub());

		// Disconnect WebSocket
		disconnectWebSocket();
	});
</script>

<svelte:head>
	<title>{presentationData?.config?.title ?? 'Tap Presentation'}</title>
</svelte:head>

<main class="app">
	{#if isLoading}
		<div class="loading-container">
			<div class="loading-spinner"></div>
			<p>Loading presentation...</p>
		</div>
	{:else if loadError}
		<div class="error-container">
			<h1>Error</h1>
			<p>{loadError}</p>
			<button onclick={() => window.location.reload()}>Reload</button>
		</div>
	{:else if presentationData && currentSlideData}
		<!-- Main slide view -->
		<SlideContainer {aspectRatio} {theme}>
			{#key slideIndex}
				<SlideRenderer
					slide={currentSlideData}
					visibleFragments={fragmentIndex}
					active={true}
					{direction}
					transitionDuration={400}
				/>
			{/key}
		</SlideContainer>

		<!-- Progress bar -->
		<ProgressBar {theme} show={showProgressBar} />

		<!-- Connection indicator -->
		<ConnectionIndicator {theme} />

		<!-- Slide overview modal -->
		<SlideOverview
			{slides}
			isOpen={isOverviewOpen}
			onClose={closeOverview}
			{theme}
		/>
	{:else}
		<div class="empty-container">
			<h1>No Presentation</h1>
			<p>No presentation data available.</p>
		</div>
	{/if}
</main>

<style>
	.app {
		width: 100vw;
		height: 100vh;
		overflow: hidden;
		background-color: #000;
	}

	/* Loading state */
	.loading-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100vh;
		gap: 1rem;
		color: #fff;
		font-family: system-ui, -apple-system, sans-serif;
	}

	.loading-spinner {
		width: 40px;
		height: 40px;
		border: 3px solid rgba(255, 255, 255, 0.2);
		border-top-color: #7c3aed;
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	/* Error state */
	.error-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100vh;
		gap: 1rem;
		color: #fff;
		font-family: system-ui, -apple-system, sans-serif;
		text-align: center;
		padding: 2rem;
	}

	.error-container h1 {
		margin: 0;
		font-size: 2rem;
		color: #ef4444;
	}

	.error-container p {
		margin: 0;
		color: rgba(255, 255, 255, 0.7);
		max-width: 400px;
	}

	.error-container button {
		margin-top: 1rem;
		padding: 0.75rem 1.5rem;
		background-color: #7c3aed;
		color: #fff;
		border: none;
		border-radius: 6px;
		font-size: 1rem;
		cursor: pointer;
		transition: background-color 0.2s ease;
	}

	.error-container button:hover {
		background-color: #6d28d9;
	}

	.error-container button:focus {
		outline: 2px solid #7c3aed;
		outline-offset: 2px;
	}

	/* Empty state */
	.empty-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100vh;
		gap: 1rem;
		color: #fff;
		font-family: system-ui, -apple-system, sans-serif;
		text-align: center;
	}

	.empty-container h1 {
		margin: 0;
		font-size: 2rem;
	}

	.empty-container p {
		margin: 0;
		color: rgba(255, 255, 255, 0.7);
	}
</style>
