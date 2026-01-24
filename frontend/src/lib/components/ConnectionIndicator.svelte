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
	/* Legacy CSS removed - will be replaced with Tailwind in US-108 */
	/* Animation-related styles preserved below */

	.connection-indicator {
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
