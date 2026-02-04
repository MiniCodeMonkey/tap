<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { Presentation, Slide, Theme } from '$lib/types';
	import {
		presentation,
		currentSlide,
		currentSlideIndex,
		currentFragmentIndex,
		loadPresentation,
		setupHashChangeListener,
		themeOverride
	} from '$lib/stores/presentation';
	import {
		connectWebSocket,
		disconnectWebSocket,
		detectStaticMode,
		getWebSocketClient
	} from '$lib/stores/websocket';
	import { setupKeyboardNavigation } from '$lib/utils/keyboard';
	import { createSlideTransition } from '$lib/utils/transitions';
	import { preloadPresentationImages } from '$lib/utils/preload';
	import type { Transition } from '$lib/types';
	import SlideContainer from '$lib/components/SlideContainer.svelte';
	import SlideRenderer from '$lib/components/SlideRenderer.svelte';
	import ProgressBar from '$lib/components/ProgressBar.svelte';
	import SlideOverview from '$lib/components/SlideOverview.svelte';
	import ConnectionIndicator from '$lib/components/ConnectionIndicator.svelte';

	// ============================================================================
	// State
	// ============================================================================

	let isLoading = $state(true);
	let isPreloading = $state(false);
	let preloadProgress = $state(0);
	let loadError = $state<string | null>(null);
	let isOverviewOpen = $state(false);
	let direction = $state<'forward' | 'backward'>('forward');

	// Store values
	let presentationData = $state<Presentation | null>(null);
	let currentSlideData = $state<Slide | null>(null);
	let slideIndex = $state(0);
	let fragmentIndex = $state(-1);
	let slides = $state<Slide[]>([]);
	let currentThemeOverride = $state<string | null>(null);

	// Print mode detection (for PDF export - shows all fragments)
	const isPrintMode = typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('print') === 'true';

	// ============================================================================
	// Derived Values
	// ============================================================================

	// Theme override from WebSocket takes precedence over presentation config
	let theme = $derived((currentThemeOverride ?? presentationData?.config?.theme ?? 'paper') as Theme);
	let aspectRatio = $derived(presentationData?.config?.aspectRatio ?? '16:9');
	let showProgressBar = $derived(presentationData?.config?.showProgressBar !== false);
	let themeColors = $derived(presentationData?.config?.themeColors);
	let customTheme = $derived(presentationData?.config?.customTheme);
	let transition = $derived((presentationData?.config?.transition ?? 'fade') as Transition);
	let transitionDuration = $derived(presentationData?.config?.transitionDuration ?? 400);

	// Track custom theme link element
	let customThemeLinkEl: HTMLLinkElement | null = null;

	// ============================================================================
	// Store Subscriptions
	// ============================================================================

	let unsubscribers: (() => void)[] = [];

	// ============================================================================
	// Custom Theme Loading
	// ============================================================================

	/**
	 * Load custom theme CSS file via a dynamic link element.
	 * Removes any previously loaded custom theme first.
	 */
	function loadCustomTheme(hasCustomTheme: boolean): void {
		// Remove existing custom theme link if present
		if (customThemeLinkEl) {
			customThemeLinkEl.remove();
			customThemeLinkEl = null;
		}

		// If no custom theme, we're done
		if (!hasCustomTheme) {
			return;
		}

		// Create link element for custom theme CSS
		const link = document.createElement('link');
		link.rel = 'stylesheet';
		link.type = 'text/css';
		// Add cache-busting timestamp for dev mode reload
		link.href = `/api/custom-theme.css?t=${Date.now()}`;
		link.id = 'custom-theme-css';

		// Handle load errors gracefully
		link.onerror = () => {
			console.warn('[tap] Custom theme CSS failed to load. Using default theme.');
		};

		// Insert after other stylesheets to ensure custom theme overrides defaults
		document.head.appendChild(link);
		customThemeLinkEl = link;
	}

	// React to custom theme changes
	$effect(() => {
		loadCustomTheme(!!customTheme);
	});

	// ============================================================================
	// Fetch Presentation
	// ============================================================================

	async function fetchPresentation(): Promise<void> {
		try {
			let data: Presentation | null = null;

			// First check for embedded data (static build)
			const embeddedScript = document.getElementById('presentation-data');
			if (embeddedScript) {
				try {
					const parsed = JSON.parse(embeddedScript.textContent || '{}') as Presentation;
					if (parsed.slides && parsed.slides.length > 0) {
						data = parsed;
					}
				} catch {
					// Fall through to API fetch
				}
			}

			// Fetch from API if no embedded data
			if (!data) {
				const response = await fetch('/api/presentation');
				if (!response.ok) {
					throw new Error(`Failed to load presentation: ${response.statusText}`);
				}
				data = (await response.json()) as Presentation;
			}

			// Preload all images before showing the presentation
			isPreloading = true;
			await preloadPresentationImages(data, (progress) => {
				preloadProgress = progress;
			});
			isPreloading = false;

			// Now load and display the presentation
			loadPresentation(data);
			isLoading = false;
		} catch (error) {
			loadError = error instanceof Error ? error.message : 'Failed to load presentation';
			isLoading = false;
			isPreloading = false;
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
	// WebSocket Broadcasting
	// ============================================================================

	// Track the last broadcasted slide index to avoid broadcasting on fragment changes
	let lastBroadcastedSlideIndex = -1;

	function broadcastSlide(): void {
		const client = getWebSocketClient();
		// Read the current slide index from the store
		let currentIndex = 0;
		const unsubscribe = currentSlideIndex.subscribe((value) => {
			currentIndex = value;
		});
		unsubscribe();

		// Only broadcast if the slide index actually changed (not just a fragment reveal)
		if (currentIndex !== lastBroadcastedSlideIndex) {
			lastBroadcastedSlideIndex = currentIndex;
			client.send({
				type: 'slide',
				slideIndex: currentIndex
			});
		}
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

		unsubscribers.push(
			themeOverride.subscribe((value) => {
				currentThemeOverride = value;
			})
		);

		// Set up hash change listener
		const hashCleanup = setupHashChangeListener();
		unsubscribers.push(hashCleanup);

		// Set up keyboard navigation
		const keyboardCleanup = setupKeyboardNavigation({
			onToggleOverview: toggleOverview,
			isOverviewOpen: isOverviewOpenFn,
			onNavigate: broadcastSlide
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

		// Clean up custom theme link
		if (customThemeLinkEl) {
			customThemeLinkEl.remove();
			customThemeLinkEl = null;
		}

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
			{#if isPreloading}
				<p class="text-slide-body font-sans">Loading assets... {Math.round(preloadProgress * 100)}%</p>
			{:else}
				<p class="text-slide-body font-sans">Loading presentation...</p>
			{/if}
		</div>
	{:else if loadError}
		<div class="error-container">
			<h1>Error</h1>
			<p>{loadError}</p>
			<button onclick={() => window.location.reload()}>Reload</button>
		</div>
	{:else if presentationData && currentSlideData}
		<!-- Main slide view -->
		<SlideContainer {aspectRatio} {theme} {themeColors}>
			{#key slideIndex}
				<div
					style="position: absolute; top: 0; left: 0; width: 100%; height: 100%;"
					in:createSlideTransition={{ type: transition, duration: transitionDuration, direction }}
					out:createSlideTransition={{ type: transition, duration: transitionDuration, direction }}
				>
					<SlideRenderer
						slide={currentSlideData}
						visibleFragments={isPrintMode ? 999 : fragmentIndex}
						active={true}
						{direction}
						{transitionDuration}
						{theme}
						{isPrintMode}
					/>
				</div>
			{/key}
		</SlideContainer>

		<!-- Progress bar -->
		<ProgressBar show={showProgressBar} />

		<!-- Connection indicator (hidden in print mode for PDF export) -->
		{#if !isPrintMode}
			<ConnectionIndicator />
		{/if}

		<!-- Slide overview modal -->
		<SlideOverview
			{slides}
			isOpen={isOverviewOpen}
			onClose={closeOverview}
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
