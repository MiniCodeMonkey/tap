/**
 * WebSocket client for hot reload and presentation sync.
 * Handles auto-reconnection with exponential backoff.
 */

import { create } from 'zustand';
import type { WebSocketMessage } from '$lib/types';
import {
	applyRemoteState,
	usePresentationStore,
	setThemeOverride,
	getHashSlideIndexAtLoad
} from '$lib/stores/presentation';

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
// Connection Store
// ============================================================================

export interface ConnectionState {
	connected: boolean;
	reconnecting: boolean;
	reconnectAttempt: number;
	/**
	 * Whether the application is running in static mode (no backend server).
	 * In static mode, there's no WebSocket server or API endpoints available.
	 */
	staticMode: boolean;
	/** Whether static mode detection has completed. */
	staticModeDetected: boolean;
}

const initialConnectionState: ConnectionState = {
	connected: false,
	reconnecting: false,
	reconnectAttempt: 0,
	staticMode: false,
	staticModeDetected: false
};

export const useConnectionStore = create<ConnectionState>(() => ({ ...initialConnectionState }));

/**
 * Whether live code execution is available.
 * This is true when connected to a WebSocket server (not in static mode).
 */
export const selectLiveExecutionAvailable = (state: ConnectionState): boolean => {
	return state.connected && !state.staticMode;
};

// ============================================================================
// Static Mode Detection
// ============================================================================

/**
 * Detect if we're running in static mode by checking if the API is available.
 * In static mode, the presentation data is embedded in the HTML and there's no backend.
 *
 * The one clear rule: a static build's HTML always carries the presentation
 * data in a `#presentation-data` element (see internal/builder/builder.go),
 * so its presence alone settles static mode without waiting on a network
 * round trip. A dev server never renders that element - it always serves
 * the presentation from `/api/presentation` - so its absence falls back to
 * probing that endpoint.
 */
export async function detectStaticMode(): Promise<boolean> {
	// Skip detection on server-side rendering
	if (typeof window === 'undefined') {
		useConnectionStore.setState({ staticModeDetected: true });
		return false;
	}

	if (document.getElementById('presentation-data')) {
		useConnectionStore.setState({ staticMode: true, staticModeDetected: true });
		return true;
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
		useConnectionStore.setState({ staticMode: isStatic, staticModeDetected: true });
		return isStatic;
	} catch {
		// Network error or abort means we're in static mode
		useConnectionStore.setState({ staticMode: true, staticModeDetected: true });
		return true;
	}
}

// ============================================================================
// Late-joiner state (hub state received before the presentation has loaded)
// ============================================================================

interface PendingInitialState {
	slideIndex: number;
	fragment: number;
	step: number;
	scrollRevealed: boolean;
	/**
	 * Mirrors the message's own `initial` field (see WebSocketMessage): true
	 * only for the hub's register-time state. Only that one is weighed
	 * against the URL hash (see applyHubLateJoinerState); anything else
	 * buffered here is a live broadcast that happened to race the
	 * presentation fetch, and applies exactly as it always has.
	 */
	isHubLateJoinerState: boolean;
}

/**
 * A remote "slide" message received while `usePresentationStore`'s
 * presentation was still null, buffered here and applied once it becomes
 * available (see the subscription below). This is what lets the hub's
 * late-joiner state - sent right after this client connects - be resolved
 * against the URL hash even when it arrives before the presentation fetch
 * resolves.
 */
let pendingInitialState: PendingInitialState | null = null;

/**
 * Whether this page load has already weighed a hub late-joiner state
 * against the URL hash once. The hash only ever describes where the page
 * itself was loaded, so it only gets a say over the very first `initial`
 * message. Every `initial` message after that is the hub's state on a
 * reconnect - the socket dropped and came back, the page never reloaded -
 * and the hash from the original load has nothing to do with where the
 * talk is now, so the hub state wins outright.
 */
let hasResolvedInitialHubState = false;

/**
 * Resolve the hub's late-joiner state against the URL hash the page loaded
 * with, per the sync design:
 * - This is a reconnect (a later `initial` message on the same page load):
 *   the hub state wins outright, regardless of the hash. A presenter that
 *   reconnects mid-talk must land on wherever the audience is now, not on
 *   the slide named by a hash read once at page load.
 * - No hash: the hub state wins outright (a presenter window opened mid-talk
 *   lands on the live slide, fragment and step).
 * - Hash names the same slide as the hub state: take the hub's fragment,
 *   step and scroll state (a reload keeps the revealed fragments).
 * - Hash names a different slide: the hash wins. The store is left exactly
 *   as initializeFromURL set it - this function applies nothing and
 *   remembers nothing, so the first real navigation after this load is not
 *   skipped by the send-side broadcast dedupe as a false no-op.
 */
function applyHubLateJoinerState(incoming: {
	slideIndex: number;
	fragment: number;
	step: number;
	scrollRevealed: boolean;
}): void {
	if (!hasResolvedInitialHubState) {
		hasResolvedInitialHubState = true;
		const hashSlideIndex = getHashSlideIndexAtLoad();
		if (hashSlideIndex !== null) {
			const total = usePresentationStore.getState().presentation?.slides.length ?? 0;
			const clampedHash =
				total > 0 ? Math.min(Math.max(hashSlideIndex, 0), total - 1) : hashSlideIndex;
			if (clampedHash !== incoming.slideIndex) {
				return;
			}
		}
	}

	if (applyRemoteState(incoming)) {
		const applied = usePresentationStore.getState();
		rememberBroadcastedState({
			slideIndex: applied.currentSlideIndex,
			fragment: applied.currentFragmentIndex,
			step: applied.currentStep,
			scrollRevealed: applied.scrollRevealed
		});
	}
}

// Apply a buffered state as soon as a presentation loads. This runs after
// loadPresentation's initializeFromURL, since both happen synchronously
// within the same loadPresentation() call, so getHashSlideIndexAtLoad()
// already reflects this load's URL hash by the time this fires.
usePresentationStore.subscribe((state, previousState) => {
	if (state.presentation === null || previousState.presentation !== null) {
		return;
	}
	if (pendingInitialState === null) {
		return;
	}
	const initial = pendingInitialState;
	pendingInitialState = null;
	if (initial.isHubLateJoinerState) {
		applyHubLateJoinerState(initial);
	} else {
		applyRemoteState(initial);
	}
});

/**
 * Reset the buffered late-joiner state and the once-per-load hash
 * resolution (for testing).
 */
export function resetPendingInitialState(): void {
	pendingInitialState = null;
	hasResolvedInitialHubState = false;
}

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
	/**
	 * Whether this client's socket has already opened once before, in this
	 * page load. The hub sends its `initial` message on register, but sends
	 * none at all when the hub is empty (nobody has navigated yet), so a
	 * page that loads onto an empty hub never gets a message that would set
	 * `hasResolvedInitialHubState`. Without this, that client's first
	 * reconnect (the hub now has state, so it sends one) would still be
	 * wrongly weighed against the load-time hash as if it were the very
	 * first `initial` message. Set in onopen, not onmessage, since it must
	 * already be true before any message this connection receives is
	 * handled.
	 */
	private hasOpenedBefore: boolean = false;

	/**
	 * Whether this page load (or, in a test, this client instance) has
	 * received its first "connected" message yet. Set together with
	 * firstRevision the first time one arrives; every "connected" message
	 * after that is a reconnect, compared against firstRevision instead of
	 * remembered.
	 */
	private hasSeenFirstConnected: boolean = false;

	/**
	 * The deck revision from this page load's first "connected" message
	 * (see internal/server/websocket.go's Revision field). undefined until
	 * that first message arrives, or if it carried no revision at all (a
	 * hub that has never had a presentation set).
	 */
	private firstRevision: string | undefined = undefined;

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
			useConnectionStore.setState({ connected: true, reconnecting: false, reconnectAttempt: 0 });
			// Reset reconnect delay on successful connection
			this.reconnectDelay = INITIAL_RECONNECT_DELAY;

			// This client has connected before in this page load, so this
			// open is a reconnect: any `initial` message on it must win
			// outright (see hasOpenedBefore's doc comment), even when the
			// earlier connection never received one itself (an empty hub
			// sends none) and so never set this on its own.
			if (this.hasOpenedBefore) {
				hasResolvedInitialHubState = true;
			}
			this.hasOpenedBefore = true;
		};

		this.ws.onclose = () => {
			useConnectionStore.setState({ connected: false });
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
				this.handleConnected(message.revision);
				break;

			case 'reload':
				// Hot reload - refresh the page
				this.handleReload();
				break;

			case 'slide':
				// Sync to the sender's slide, fragment, step and scroll reveal state
				this.handleSlideState(message);
				break;

			case 'theme':
				// Switch to a different theme
				this.handleThemeChange(message.theme);
				break;
		}
	}

	/**
	 * Handle a "connected" message: remembers the revision carried by the
	 * first one this page load receives, and reloads the page on any later
	 * one (a reconnect - the socket dropped and came back, or the hub
	 * itself restarted) whose revision differs from that first one. This is
	 * how a window left open through a `tap dev` restart, or a deck reload
	 * while its socket was down, notices the deck changed instead of going
	 * on showing the old one until someone reloads manually.
	 *
	 * Never reloads on the very first "connected" message, even when the
	 * hub already has a revision by then (a page that loads after the hub
	 * has been running a while) - there is nothing to compare it against
	 * yet. Never loops: a reload starts a new page load, and the new
	 * WebSocketClient's first "connected" message is recorded, not
	 * compared, exactly as this one's was.
	 */
	private handleConnected(revision: string | undefined): void {
		if (!this.hasSeenFirstConnected) {
			this.hasSeenFirstConnected = true;
			this.firstRevision = revision;
			return;
		}
		if (revision !== undefined && revision !== this.firstRevision) {
			this.handleReload();
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
	 * Handle a slide message: apply the sender's slide index, fragment, step
	 * and scroll reveal state. A field the sender omitted (an old client, or
	 * a message that only changed the slide) means "this slide, initial
	 * state", matching the pre-fragment-sync behavior exactly.
	 */
	private handleSlideState(message: WebSocketMessage): void {
		if (message.slideIndex === undefined) return;

		const fragment = message.fragment ?? -1;
		const step = message.step ?? 0;
		const scrollRevealed = message.scrollRevealed ?? false;
		const incoming = { slideIndex: message.slideIndex, fragment, step, scrollRevealed };

		// The hub sets `initial` only on the one message it sends a client
		// right on register (see internal/server/websocket.go); every other
		// message, including the very first one a client happens to receive
		// when the hub has no state yet (nobody has navigated), is a live
		// navigation and applies as such. This is a server-asserted fact,
		// not something inferred from arrival order or timing.
		const isHubLateJoinerState = message.initial === true;

		const state = usePresentationStore.getState();
		if (state.presentation === null) {
			// The presentation hasn't loaded yet - buffer this message and
			// apply it once loadPresentation's initializeFromURL has run (see
			// the store subscription above), which resolves it against the
			// URL hash the same way it would have been resolved here if the
			// presentation had already loaded.
			pendingInitialState = { ...incoming, isHubLateJoinerState };
			return;
		}

		if (isHubLateJoinerState) {
			applyHubLateJoinerState(incoming);
			return;
		}

		// Skip when the incoming state exactly matches ours already.
		// This prevents resetting fragment state when receiving our own broadcast,
		// which the hub echoes back to the sender along with every other client.
		if (
			message.slideIndex === state.currentSlideIndex &&
			fragment === state.currentFragmentIndex &&
			step === state.currentStep &&
			scrollRevealed === state.scrollRevealed
		) {
			return;
		}

		if (applyRemoteState({ slideIndex: message.slideIndex, fragment, step, scrollRevealed })) {
			// Remember this as "our" last-known state too. Without this, a
			// later local navigation back to a state a remote client just
			// pushed on us would be skipped by the send-side dedupe below,
			// even though the other clients never actually received it from
			// us (they received it as a broadcast of their own action, or
			// from a third client). Example: viewer -> 2 (broadcasts 2),
			// presenter -> 3 (viewer applies remote 3), viewer -> back to 2:
			// without this update the viewer's remembered state would still
			// be 2 from its first broadcast, so the second "2" send would be
			// wrongly skipped as a no-op.
			// Read back the state actually applied, not the raw message
			// fields: applyRemoteState clamps out-of-range values, so what
			// landed in the store can differ from what was received.
			const applied = usePresentationStore.getState();
			rememberBroadcastedState({
				slideIndex: applied.currentSlideIndex,
				fragment: applied.currentFragmentIndex,
				step: applied.currentStep,
				scrollRevealed: applied.scrollRevealed
			});
		}
	}

	/**
	 * Handle theme change message.
	 * Updates the theme override store to switch themes instantly. An
	 * unknown slug is accepted here too: the theme loader falls back to
	 * `base` for anything it doesn't recognize or hasn't been ported yet.
	 */
	private handleThemeChange(theme: string | undefined): void {
		if (!theme) return;
		setThemeOverride(theme);
	}

	/**
	 * Schedule a reconnection attempt with exponential backoff.
	 */
	private scheduleReconnect(): void {
		if (!this.shouldReconnect) return;
		if (this.reconnectTimeout) return; // Already scheduled

		useConnectionStore.setState((state) => ({
			reconnecting: true,
			reconnectAttempt: state.reconnectAttempt + 1
		}));

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
	 * Disconnect from the WebSocket server. Only ever called from a
	 * component unmount (App.tsx/PresenterApp.tsx), which in practice means
	 * the page itself is going away - so resetting hasOpenedBefore here
	 * costs nothing in production, and keeps a later connect() (a fresh
	 * mount within the same page, as in a test) starting clean rather than
	 * carrying over this client's connection history.
	 */
	disconnect(): void {
		this.shouldReconnect = false;
		this.hasOpenedBefore = false;
		this.hasSeenFirstConnected = false;
		this.firstRevision = undefined;

		if (this.reconnectTimeout) {
			clearTimeout(this.reconnectTimeout);
			this.reconnectTimeout = null;
		}

		if (this.ws) {
			this.ws.close();
			this.ws = null;
		}

		useConnectionStore.setState({ connected: false, reconnecting: false, reconnectAttempt: 0 });
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
// Presentation State Broadcasting
// ============================================================================

interface PresentationSyncState {
	slideIndex: number;
	fragment: number;
	step: number;
	scrollRevealed: boolean;
}

/**
 * The last state this client either broadcast or applied from a remote
 * client, so an unchanged navigation attempt is not resent. Tracking remote
 * state here too (not just sends) matters: without it, navigating back to a
 * slide a remote client just pushed on us looks like a no-op locally and the
 * send is skipped, even though no other client actually received it from us.
 */
let lastBroadcastedState: PresentationSyncState | null = null;

/**
 * Record state received from (and applied for) a remote client as this
 * client's own last-known broadcast state, so the send-side dedupe in
 * broadcastPresentationState compares against reality instead of only
 * against what this client itself has sent.
 */
function rememberBroadcastedState(state: PresentationSyncState): void {
	lastBroadcastedState = state;
}

/**
 * Broadcast the current slide, fragment, step and scroll reveal state to
 * other connected clients. Every navigation action (next, previous, go to
 * slide, fragment reveal/hide, step forward/back, scroll reveal) calls this
 * after it runs, so the viewer and the presenter mirror each other exactly.
 * Skips the send if the state is identical to the last broadcast, since
 * navigation at the start or end of the deck can be attempted with no effect.
 */
export function broadcastPresentationState(): void {
	const { currentSlideIndex, currentFragmentIndex, currentStep, scrollRevealed } =
		usePresentationStore.getState();
	const state: PresentationSyncState = {
		slideIndex: currentSlideIndex,
		fragment: currentFragmentIndex,
		step: currentStep,
		scrollRevealed
	};

	if (
		lastBroadcastedState !== null &&
		lastBroadcastedState.slideIndex === state.slideIndex &&
		lastBroadcastedState.fragment === state.fragment &&
		lastBroadcastedState.step === state.step &&
		lastBroadcastedState.scrollRevealed === state.scrollRevealed
	) {
		return;
	}

	lastBroadcastedState = state;
	getWebSocketClient().send({
		type: 'slide',
		slideIndex: state.slideIndex,
		fragment: state.fragment,
		step: state.step,
		scrollRevealed: state.scrollRevealed
	});
}

/**
 * Reset the last-broadcast state tracked by broadcastPresentationState (for testing).
 */
export function resetBroadcastedState(): void {
	lastBroadcastedState = null;
}

// ============================================================================
// Export constants for testing
// ============================================================================

export const WEBSOCKET_CONSTANTS = {
	INITIAL_RECONNECT_DELAY,
	MAX_RECONNECT_DELAY,
	RECONNECT_BACKOFF_MULTIPLIER
};
