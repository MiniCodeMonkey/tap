<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type {
		AsciinemaPlayerInstance,
		AsciinemaPlayerOptions
	} from '$lib/types/asciinema.d.ts';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** Source URL or path to the .cast file */
		src: string;
		/** Whether to auto-play the recording */
		autoPlay?: boolean;
		/** Playback speed (1.0 = normal) */
		speed?: number;
		/** Whether to loop the recording */
		loop?: boolean;
		/** Time to start playback from (in seconds) */
		startAt?: number;
		/** Number of terminal columns (auto-detected if not set) */
		cols?: number;
		/** Number of terminal rows (auto-detected if not set) */
		rows?: number;
		/** Maximum idle time between frames (in seconds) */
		idleTimeLimit?: number;
		/** Whether to preload the recording */
		preload?: boolean;
		/** Fit mode for the player */
		fit?: 'width' | 'height' | 'both' | 'none';
		/** Optional poster frame specification */
		poster?: string;
		/** Custom CSS class for the container */
		className?: string;
	}

	let {
		src,
		autoPlay = false,
		speed = 1.0,
		loop = false,
		startAt = 0,
		cols,
		rows,
		idleTimeLimit,
		preload = true,
		fit = 'width',
		poster,
		className = ''
	}: Props = $props();

	// ============================================================================
	// State
	// ============================================================================

	/** Container element for the player */
	let containerEl: HTMLElement | undefined = $state();

	/** The asciinema player instance */
	let player: AsciinemaPlayerInstance | null = $state(null);

	/** Whether the library is loaded */
	let libraryLoaded = $state(false);

	/** Whether loading is in progress */
	let isLoading = $state(true);

	/** Error message if loading fails */
	let errorMessage = $state<string | null>(null);

	/** Current playback speed */
	let currentSpeed = $state(speed);

	/** Whether the player is currently playing */
	let isPlaying = $state(false);

	// ============================================================================
	// Constants
	// ============================================================================

	/** CDN URLs for asciinema-player */
	const ASCIINEMA_PLAYER_JS =
		'https://cdn.jsdelivr.net/npm/asciinema-player@3.9.0/dist/bundle/asciinema-player.min.js';
	const ASCIINEMA_PLAYER_CSS =
		'https://cdn.jsdelivr.net/npm/asciinema-player@3.9.0/dist/bundle/asciinema-player.min.css';

	/** Available playback speeds */
	const SPEED_OPTIONS = [0.5, 1.0, 1.5, 2.0, 3.0];

	// ============================================================================
	// Library Loading
	// ============================================================================

	/**
	 * Load the asciinema-player library from CDN.
	 */
	async function loadLibrary(): Promise<void> {
		// Check if already loaded
		if (window.AsciinemaPlayer) {
			libraryLoaded = true;
			return;
		}

		// Check if already loading (script tag exists)
		const existingScript = document.querySelector(
			`script[src="${ASCIINEMA_PLAYER_JS}"]`
		);
		const existingStyles = document.querySelector(
			`link[href="${ASCIINEMA_PLAYER_CSS}"]`
		);

		// Load CSS if not present
		if (!existingStyles) {
			const link = document.createElement('link');
			link.rel = 'stylesheet';
			link.href = ASCIINEMA_PLAYER_CSS;
			document.head.appendChild(link);
		}

		// Load JS if not present
		if (!existingScript) {
			await new Promise<void>((resolve, reject) => {
				const script = document.createElement('script');
				script.src = ASCIINEMA_PLAYER_JS;
				script.async = true;
				script.onload = () => resolve();
				script.onerror = () =>
					reject(new Error('Failed to load asciinema-player library'));
				document.head.appendChild(script);
			});
		} else {
			// Wait for existing script to load
			await new Promise<void>((resolve) => {
				const checkLoaded = () => {
					if (window.AsciinemaPlayer) {
						resolve();
					} else {
						setTimeout(checkLoaded, 50);
					}
				};
				checkLoaded();
			});
		}

		libraryLoaded = true;
	}

	// ============================================================================
	// Player Management
	// ============================================================================

	/**
	 * Initialize the asciinema player.
	 */
	async function initializePlayer(): Promise<void> {
		if (!containerEl || !libraryLoaded || !window.AsciinemaPlayer) {
			return;
		}

		// Dispose existing player if any
		disposePlayer();

		try {
			// Build options
			const options: AsciinemaPlayerOptions = {
				autoPlay,
				speed: currentSpeed,
				loop,
				startAt,
				preload,
				fit: fit === 'none' ? false : fit,
				controls: true,
				theme: 'monokai' // Default dark theme
			};

			if (cols) options.cols = cols;
			if (rows) options.rows = rows;
			if (idleTimeLimit) options.idleTimeLimit = idleTimeLimit;
			if (poster) options.poster = poster;

			// Create the player
			player = window.AsciinemaPlayer.create(src, containerEl, options);

			// Track playing state
			isPlaying = autoPlay;
			errorMessage = null;
		} catch (err) {
			errorMessage =
				err instanceof Error ? err.message : 'Failed to initialize player';
			player = null;
		} finally {
			isLoading = false;
		}
	}

	/**
	 * Dispose the player instance.
	 */
	function disposePlayer(): void {
		if (player) {
			try {
				player.dispose();
			} catch {
				// Ignore disposal errors
			}
			player = null;
		}
	}

	// ============================================================================
	// Playback Controls
	// ============================================================================

	/**
	 * Toggle play/pause state.
	 */
	function togglePlayPause(): void {
		if (!player) return;

		if (isPlaying) {
			player.pause();
			isPlaying = false;
		} else {
			player.play();
			isPlaying = true;
		}
	}

	/**
	 * Play the recording.
	 */
	function play(): void {
		if (player && !isPlaying) {
			player.play();
			isPlaying = true;
		}
	}

	/**
	 * Pause the recording.
	 */
	function pause(): void {
		if (player && isPlaying) {
			player.pause();
			isPlaying = false;
		}
	}

	// Expose play and pause functions for external use
	// These are declared to avoid unused variable warnings
	void pause;

	/**
	 * Set playback speed.
	 */
	function setSpeed(newSpeed: number): void {
		currentSpeed = newSpeed;
		// Re-initialize player to apply new speed
		// (asciinema-player doesn't support runtime speed changes)
		if (player) {
			const wasPlaying = isPlaying;
			initializePlayer().then(() => {
				if (wasPlaying) {
					play();
				}
			});
		}
	}

	/**
	 * Cycle through playback speeds.
	 */
	function cycleSpeed(): void {
		const currentIndex = SPEED_OPTIONS.indexOf(currentSpeed);
		const nextIndex = (currentIndex + 1) % SPEED_OPTIONS.length;
		setSpeed(SPEED_OPTIONS[nextIndex] ?? 1.0);
	}

	// ============================================================================
	// Lifecycle
	// ============================================================================

	onMount(async () => {
		// Update currentSpeed when speed prop changes
		currentSpeed = speed;

		try {
			await loadLibrary();
			await initializePlayer();
		} catch (err) {
			errorMessage =
				err instanceof Error ? err.message : 'Failed to load asciinema player';
			isLoading = false;
		}
	});

	onDestroy(() => {
		disposePlayer();
	});

	// ============================================================================
	// Reactive Updates
	// ============================================================================

	// Re-initialize when src changes
	$effect(() => {
		// Track src dependency
		const _ = src;
		void _;
		if (libraryLoaded && containerEl) {
			initializePlayer();
		}
	});
</script>

<div
	class="asciinema-player-wrapper {className}"
	class:loading={isLoading}
	class:error={!!errorMessage}
>
	{#if isLoading}
		<div class="loading-state">
			<div class="loading-spinner" aria-hidden="true"></div>
			<span class="loading-text">Loading terminal recording...</span>
		</div>
	{:else if errorMessage}
		<div class="error-state">
			<span class="error-icon" aria-hidden="true">&#9888;</span>
			<span class="error-text">{errorMessage}</span>
			<span class="error-hint"
				>Check that the .cast file exists and is accessible.</span
			>
		</div>
	{:else}
		<!-- Player container -->
		<div class="player-container" bind:this={containerEl}></div>

		<!-- Custom controls overlay -->
		<div class="controls-overlay">
			<button
				class="control-button play-pause"
				onclick={togglePlayPause}
				aria-label={isPlaying ? 'Pause' : 'Play'}
				title={isPlaying ? 'Pause' : 'Play'}
			>
				{#if isPlaying}
					<span class="pause-icon" aria-hidden="true">&#9208;</span>
				{:else}
					<span class="play-icon" aria-hidden="true">&#9655;</span>
				{/if}
			</button>

			<button
				class="control-button speed"
				onclick={cycleSpeed}
				aria-label="Change playback speed"
				title="Playback speed: {currentSpeed}x (click to change)"
			>
				{currentSpeed}x
			</button>
		</div>
	{/if}
</div>
