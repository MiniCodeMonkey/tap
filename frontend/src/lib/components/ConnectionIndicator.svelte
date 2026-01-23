<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		connected,
		reconnecting,
		reconnectAttempt,
		staticMode,
		staticModeDetected
	} from '$lib/stores/websocket';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** Theme name for styling */
		theme?: string;
	}

	let { theme = 'minimal' }: Props = $props();

	// ============================================================================
	// State
	// ============================================================================

	let isConnected = $state(false);
	let isReconnecting = $state(false);
	let attemptNumber = $state(0);
	let isStaticMode = $state(false);
	let staticDetectionComplete = $state(false);

	// ============================================================================
	// Store Subscriptions
	// ============================================================================

	let unsubscribers: (() => void)[] = [];

	onMount(() => {
		unsubscribers.push(
			connected.subscribe((value) => {
				isConnected = value;
			})
		);

		unsubscribers.push(
			reconnecting.subscribe((value) => {
				isReconnecting = value;
			})
		);

		unsubscribers.push(
			reconnectAttempt.subscribe((value) => {
				attemptNumber = value;
			})
		);

		unsubscribers.push(
			staticMode.subscribe((value) => {
				isStaticMode = value;
			})
		);

		unsubscribers.push(
			staticModeDetected.subscribe((value) => {
				staticDetectionComplete = value;
			})
		);
	});

	onDestroy(() => {
		unsubscribers.forEach((unsub) => unsub());
	});

	// ============================================================================
	// Computed Values
	// ============================================================================

	/**
	 * Whether to show the indicator.
	 * Don't show in static mode or when connected.
	 * Only show after static mode detection is complete to avoid flashing.
	 */
	let shouldShow = $derived(
		staticDetectionComplete && !isStaticMode && !isConnected
	);

	/**
	 * Status text to display.
	 */
	let statusText = $derived(() => {
		if (isReconnecting && attemptNumber > 0) {
			return `Reconnecting... (${attemptNumber})`;
		}
		return 'Disconnected';
	});
</script>

{#if shouldShow}
	<div
		class="connection-indicator theme-{theme}"
		class:reconnecting={isReconnecting}
		role="status"
		aria-live="polite"
		aria-label={statusText()}
	>
		<span class="indicator-dot" class:pulse={isReconnecting}></span>
		<span class="indicator-text">{statusText()}</span>
	</div>
{/if}

<style>
	.connection-indicator {
		/* Position in corner of slide */
		position: fixed;
		top: 1rem;
		right: 1rem;
		z-index: 1000;

		/* Layout */
		display: flex;
		align-items: center;
		gap: 0.5rem;

		/* Appearance */
		padding: 0.5rem 0.75rem;
		border-radius: 6px;
		background-color: var(--indicator-bg, rgba(0, 0, 0, 0.7));
		color: var(--indicator-text, #ffffff);
		font-size: 0.75rem;
		font-family: system-ui, -apple-system, sans-serif;

		/* Animation for appearance */
		animation: slideIn 0.3s ease-out;
	}

	@keyframes slideIn {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.indicator-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background-color: var(--indicator-dot, #ef4444);
		flex-shrink: 0;
	}

	.indicator-dot.pulse {
		animation: pulse 1.5s ease-in-out infinite;
	}

	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
			transform: scale(1);
		}
		50% {
			opacity: 0.5;
			transform: scale(0.8);
		}
	}

	.indicator-text {
		white-space: nowrap;
	}

	/* Theme-specific styles */

	/* Minimal theme */
	:global(.theme-minimal) .connection-indicator {
		--indicator-bg: rgba(0, 0, 0, 0.8);
		--indicator-text: #ffffff;
		--indicator-dot: #ef4444;
	}

	:global(.theme-minimal) .connection-indicator.reconnecting {
		--indicator-dot: #fbbf24;
	}

	/* Terminal theme */
	:global(.theme-terminal) .connection-indicator {
		--indicator-bg: rgba(0, 0, 0, 0.9);
		--indicator-text: #00ff00;
		--indicator-dot: #ff0000;
		font-family: 'SF Mono', Monaco, 'Cascadia Code', Consolas, monospace;
		border: 1px solid #00ff00;
	}

	:global(.theme-terminal) .connection-indicator.reconnecting {
		--indicator-dot: #ffb000;
	}

	/* Gradient theme */
	:global(.theme-gradient) .connection-indicator {
		--indicator-bg: rgba(255, 255, 255, 0.15);
		--indicator-text: #ffffff;
		--indicator-dot: #ef4444;
		backdrop-filter: blur(8px);
		-webkit-backdrop-filter: blur(8px);
		border: 1px solid rgba(255, 255, 255, 0.2);
	}

	:global(.theme-gradient) .connection-indicator.reconnecting {
		--indicator-dot: #fbbf24;
	}

	/* Brutalist theme */
	:global(.theme-brutalist) .connection-indicator {
		--indicator-bg: #ffffff;
		--indicator-text: #000000;
		--indicator-dot: #ff0000;
		border: 3px solid #000000;
		border-radius: 0;
		text-transform: uppercase;
		font-weight: 700;
		font-family: 'Arial Black', 'Helvetica Bold', sans-serif;
	}

	:global(.theme-brutalist) .connection-indicator.reconnecting {
		--indicator-dot: #ffa500;
	}

	/* Keynote theme */
	:global(.theme-keynote) .connection-indicator {
		--indicator-bg: rgba(0, 0, 0, 0.75);
		--indicator-text: #ffffff;
		--indicator-dot: #ff3b30;
		border-radius: 8px;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
	}

	:global(.theme-keynote) .connection-indicator.reconnecting {
		--indicator-dot: #ff9500;
	}

	/* Reduced motion support */
	@media (prefers-reduced-motion: reduce) {
		.connection-indicator {
			animation: none;
		}

		.indicator-dot.pulse {
			animation: none;
		}
	}
</style>
