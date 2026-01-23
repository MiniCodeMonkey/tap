/**
 * WebSocket client for hot reload and presentation sync.
 * Handles auto-reconnection with exponential backoff.
 */

import { writable, type Writable, type Readable, derived } from 'svelte/store';
import type { WebSocketMessage } from '$lib/types';
import { goToSlide, presentation } from '$lib/stores/presentation';

// ============================================================================
// Constants
// ============================================================================

/** Initial reconnect delay in milliseconds */
const INITIAL_RECONNECT_DELAY = 1000;

/** Maximum reconnect delay in milliseconds */
const MAX_RECONNECT_DELAY = 30000;

/** Reconnect delay multiplier for exponential backoff */
const RECONNECT_BACKOFF_MULTIPLIER = 2;

// ============================================================================
// Static Mode Detection
// ============================================================================

/**
 * Whether the application is running in static mode (no backend server).
 * In static mode, there's no WebSocket server or API endpoints available.
 * This is detected by attempting to fetch a test endpoint.
 */
export const staticMode: Writable<boolean> = writable(false);

/**
 * Whether static mode detection has completed.
 */
export const staticModeDetected: Writable<boolean> = writable(false);

/**
 * Detect if we're running in static mode by checking if the API is available.
 * In static mode, the presentation data is embedded in the HTML and there's no backend.
 */
export async function detectStaticMode(): Promise<boolean> {
	// Skip detection on server-side rendering
	if (typeof window === 'undefined') {
		staticModeDetected.set(true);
		return false;
	}

	try {
		// Try to fetch the presentation API endpoint
		// In dev mode, this will succeed; in static mode, it will fail
		const controller = new AbortController();
		const timeoutId = setTimeout(() => controller.abort(), 2000);

		const response = await fetch('/api/presentation', {
			method: 'HEAD',
			signal: controller.signal
		});

		clearTimeout(timeoutId);

		// If we get a response (even an error status), we're not in static mode
		const isStatic = !response.ok;
		staticMode.set(isStatic);
		staticModeDetected.set(true);
		return isStatic;
	} catch {
		// Network error or abort means we're in static mode
		staticMode.set(true);
		staticModeDetected.set(true);
		return true;
	}
}

// ============================================================================
// Connection State Store
// ============================================================================

/**
 * Whether the WebSocket is currently connected.
 */
export const connected: Writable<boolean> = writable(false);

/**
 * Whether live code execution is available.
 * This is true when connected to a WebSocket server (not in static mode).
 */
export const liveExecutionAvailable: Readable<boolean> = derived(
	[connected, staticMode],
	([$connected, $staticMode]) => $connected && !$staticMode
);

// ============================================================================
// WebSocket Client Class
// ============================================================================

/**
 * WebSocket client for hot reload and presentation sync.
 * Automatically reconnects on disconnection with exponential backoff.
 */
export class WebSocketClient {
	private ws: WebSocket | null = null;
	private reconnectDelay: number = INITIAL_RECONNECT_DELAY;
	private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
	private shouldReconnect: boolean = true;
	private url: string;

	constructor(url?: string) {
		// Default to current host with /ws path
		this.url = url ?? this.getDefaultURL();
	}

	/**
	 * Get the default WebSocket URL based on current location.
	 */
	private getDefaultURL(): string {
		if (typeof window === 'undefined') {
			return 'ws://localhost:3000/ws';
		}
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		return `${protocol}//${window.location.host}/ws`;
	}

	/**
	 * Connect to the WebSocket server.
	 */
	connect(): void {
		if (this.ws && this.ws.readyState === WebSocket.OPEN) {
			return; // Already connected
		}

		this.shouldReconnect = true;

		try {
			this.ws = new WebSocket(this.url);
			this.setupEventHandlers();
		} catch {
			this.scheduleReconnect();
		}
	}

	/**
	 * Set up WebSocket event handlers.
	 */
	private setupEventHandlers(): void {
		if (!this.ws) return;

		this.ws.onopen = () => {
			connected.set(true);
			// Reset reconnect delay on successful connection
			this.reconnectDelay = INITIAL_RECONNECT_DELAY;
		};

		this.ws.onclose = () => {
			connected.set(false);
			this.ws = null;
			this.scheduleReconnect();
		};

		this.ws.onerror = () => {
			// Error will trigger close event, which handles reconnection
		};

		this.ws.onmessage = (event: MessageEvent) => {
			this.handleMessage(event.data as string);
		};
	}

	/**
	 * Handle incoming WebSocket messages.
	 */
	private handleMessage(data: string): void {
		try {
			const message = JSON.parse(data) as WebSocketMessage;
			this.dispatchMessage(message);
		} catch {
			// Ignore invalid JSON messages
		}
	}

	/**
	 * Dispatch message to appropriate handler.
	 */
	private dispatchMessage(message: WebSocketMessage): void {
		switch (message.type) {
			case 'connected':
				// Server acknowledged connection
				break;

			case 'reload':
				// Hot reload - refresh the page
				this.handleReload();
				break;

			case 'slide':
				// Sync to specific slide
				this.handleSlideNavigation(message.slideIndex);
				break;
		}
	}

	/**
	 * Handle reload message by refreshing the page.
	 */
	private handleReload(): void {
		if (typeof window !== 'undefined') {
			window.location.reload();
		}
	}

	/**
	 * Handle slide navigation message.
	 */
	private handleSlideNavigation(slideIndex: number | undefined): void {
		if (slideIndex === undefined) return;

		// Check if presentation is loaded before navigating
		let hasPresentation = false;
		const unsubscribe = presentation.subscribe(($presentation) => {
			hasPresentation = $presentation !== null;
		});
		unsubscribe();

		if (hasPresentation) {
			goToSlide(slideIndex);
		}
	}

	/**
	 * Schedule a reconnection attempt with exponential backoff.
	 */
	private scheduleReconnect(): void {
		if (!this.shouldReconnect) return;
		if (this.reconnectTimeout) return; // Already scheduled

		this.reconnectTimeout = setTimeout(() => {
			this.reconnectTimeout = null;
			this.connect();
		}, this.reconnectDelay);

		// Increase delay for next attempt (exponential backoff)
		this.reconnectDelay = Math.min(
			this.reconnectDelay * RECONNECT_BACKOFF_MULTIPLIER,
			MAX_RECONNECT_DELAY
		);
	}

	/**
	 * Disconnect from the WebSocket server.
	 */
	disconnect(): void {
		this.shouldReconnect = false;

		if (this.reconnectTimeout) {
			clearTimeout(this.reconnectTimeout);
			this.reconnectTimeout = null;
		}

		if (this.ws) {
			this.ws.close();
			this.ws = null;
		}

		connected.set(false);
	}

	/**
	 * Send a message to the server.
	 */
	send(message: WebSocketMessage): void {
		if (this.ws && this.ws.readyState === WebSocket.OPEN) {
			this.ws.send(JSON.stringify(message));
		}
	}

	/**
	 * Check if currently connected.
	 */
	isConnected(): boolean {
		return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
	}

	/**
	 * Get the current reconnect delay (for testing).
	 */
	getReconnectDelay(): number {
		return this.reconnectDelay;
	}

	/**
	 * Reset reconnect delay (for testing).
	 */
	resetReconnectDelay(): void {
		this.reconnectDelay = INITIAL_RECONNECT_DELAY;
	}
}

// ============================================================================
// Singleton Instance
// ============================================================================

let clientInstance: WebSocketClient | null = null;

/**
 * Get the singleton WebSocket client instance.
 * Creates a new instance if one doesn't exist.
 */
export function getWebSocketClient(): WebSocketClient {
	if (!clientInstance) {
		clientInstance = new WebSocketClient();
	}
	return clientInstance;
}

/**
 * Connect to the WebSocket server using the singleton client.
 */
export function connectWebSocket(): void {
	getWebSocketClient().connect();
}

/**
 * Disconnect from the WebSocket server using the singleton client.
 */
export function disconnectWebSocket(): void {
	getWebSocketClient().disconnect();
}

// ============================================================================
// Export constants for testing
// ============================================================================

export const WEBSOCKET_CONSTANTS = {
	INITIAL_RECONNECT_DELAY,
	MAX_RECONNECT_DELAY,
	RECONNECT_BACKOFF_MULTIPLIER
};
