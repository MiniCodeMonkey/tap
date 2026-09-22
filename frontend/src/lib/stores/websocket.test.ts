/**
 * Unit tests for the WebSocket client.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
	WebSocketClient,
	useConnectionStore,
	WEBSOCKET_CONSTANTS,
	getWebSocketClient,
	connectWebSocket,
	disconnectWebSocket,
	broadcastPresentationState,
	resetBroadcastedState,
	resetPendingInitialState,
	detectStaticMode
} from './websocket';
import { usePresentationStore, resetHashSlideIndexAtLoad, loadPresentation } from './presentation';
import type { Presentation, WebSocketMessage } from '$lib/types';

// Mock WebSocket
class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	readyState: number = MockWebSocket.CONNECTING;
	url: string;
	onopen: (() => void) | null = null;
	onclose: (() => void) | null = null;
	onerror: (() => void) | null = null;
	onmessage: ((event: { data: string }) => void) | null = null;

	constructor(url: string) {
		this.url = url;
	}

	close(): void {
		this.readyState = MockWebSocket.CLOSED;
		if (this.onclose) {
			this.onclose();
		}
	}

	send(_data: string): void {
		// Mock implementation
	}

	// Helper to simulate connection open
	simulateOpen(): void {
		this.readyState = MockWebSocket.OPEN;
		if (this.onopen) {
			this.onopen();
		}
	}

	// Helper to simulate connection close
	simulateClose(): void {
		this.readyState = MockWebSocket.CLOSED;
		if (this.onclose) {
			this.onclose();
		}
	}

	// Helper to simulate error
	simulateError(): void {
		if (this.onerror) {
			this.onerror();
		}
	}

	// Helper to simulate incoming message
	simulateMessage(message: WebSocketMessage): void {
		if (this.onmessage) {
			this.onmessage({ data: JSON.stringify(message) });
		}
	}
}

/**
 * Build a mock WebSocket constructor that carries the readyState statics
 * (OPEN, CLOSED, etc.) the client code reads off the global WebSocket.
 */
function createMockWebSocketConstructor(onCreate?: (ws: MockWebSocket) => void) {
	// A regular function expression, not an arrow function, so the vi.fn()
	// mock stays constructible when the client calls `new WebSocket(...)`.
	const constructor = vi.fn().mockImplementation(function (url: string) {
		const ws = new MockWebSocket(url);
		onCreate?.(ws);
		return ws;
	});
	return Object.assign(constructor, {
		CONNECTING: MockWebSocket.CONNECTING,
		OPEN: MockWebSocket.OPEN,
		CLOSING: MockWebSocket.CLOSING,
		CLOSED: MockWebSocket.CLOSED
	});
}

describe('WebSocketClient', () => {
	let mockWs: MockWebSocket | null = null;
	let client: WebSocketClient;

	beforeEach(() => {
		// Mock WebSocket constructor
		vi.stubGlobal(
			'WebSocket',
			createMockWebSocketConstructor((ws) => {
				mockWs = ws;
			})
		);

		// Mock window.location
		vi.stubGlobal('window', {
			location: {
				protocol: 'http:',
				host: 'localhost:3000',
				reload: vi.fn()
			},
			history: { replaceState: vi.fn() }
		});

		// Reset stores
		useConnectionStore.setState({ connected: false });
		usePresentationStore.setState({ presentation: null, currentSlideIndex: 0, currentFragmentIndex: -1 });
		resetPendingInitialState();

		// Create fresh client
		client = new WebSocketClient();
	});

	afterEach(() => {
		client.disconnect();
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
		mockWs = null;
	});

	describe('constructor', () => {
		it('should create client with default URL', () => {
			const newClient = new WebSocketClient();
			newClient.connect();
			expect(mockWs?.url).toBe('ws://localhost:3000/ws');
			newClient.disconnect();
		});

		it('should create client with custom URL', () => {
			const customUrl = 'ws://custom.example.com/ws';
			const newClient = new WebSocketClient(customUrl);
			newClient.connect();
			expect(mockWs?.url).toBe(customUrl);
			newClient.disconnect();
		});
	});

	describe('connect', () => {
		it('should establish WebSocket connection', () => {
			client.connect();
			expect(mockWs).not.toBeNull();
		});

		it('should set connected to true when connection opens', () => {
			client.connect();
			mockWs?.simulateOpen();

			expect(useConnectionStore.getState().connected).toBe(true);
		});

		it('should not create new connection if already connected', () => {
			client.connect();
			mockWs?.simulateOpen();

			const firstWs = mockWs;
			client.connect();

			expect(mockWs).toBe(firstWs);
		});
	});

	describe('disconnect', () => {
		it('should close WebSocket connection', () => {
			client.connect();
			mockWs?.simulateOpen();

			client.disconnect();

			expect(client.isConnected()).toBe(false);
		});

		it('should set connected to false', () => {
			client.connect();
			mockWs?.simulateOpen();
			client.disconnect();

			expect(useConnectionStore.getState().connected).toBe(false);
		});

		it('should prevent auto-reconnect after disconnect', () => {
			vi.useFakeTimers();

			client.connect();
			mockWs?.simulateOpen();
			client.disconnect();

			// Advance timers past max reconnect delay
			vi.advanceTimersByTime(WEBSOCKET_CONSTANTS.MAX_RECONNECT_DELAY + 1000);

			expect(client.isConnected()).toBe(false);
			vi.useRealTimers();
		});
	});

	describe('message handling', () => {
		it('should handle "connected" message type', () => {
			client.connect();
			mockWs?.simulateOpen();

			// Should not throw
			mockWs?.simulateMessage({ type: 'connected' });

			expect(client.isConnected()).toBe(true);
		});

		it('stores the disk status from a recording message and resets it on connect', () => {
			client.connect();
			mockWs?.simulateOpen();

			mockWs?.simulateMessage({ type: 'recording', disk: 'low' });
			expect(useConnectionStore.getState().diskStatus).toBe('low');

			mockWs?.simulateMessage({ type: 'connected' });
			expect(useConnectionStore.getState().diskStatus).toBe('ok');
		});

		it('sets presentMode from the first "connected" message\'s mode', () => {
			client.connect();
			mockWs?.simulateOpen();

			expect(useConnectionStore.getState().presentMode).toBe(false);

			mockWs?.simulateMessage({ type: 'connected', mode: 'present' });
			expect(useConnectionStore.getState().presentMode).toBe(true);
		});

		describe('deck revision on "connected" messages', () => {
			it('does not reload on the first "connected" message, even when it carries a revision', () => {
				const reloadSpy = vi.fn();
				vi.stubGlobal('window', {
					location: { protocol: 'http:', host: 'localhost:3000', reload: reloadSpy }
				});

				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'connected', revision: 'abc123' });

				expect(reloadSpy).not.toHaveBeenCalled();
			});

			it('does not reload on a reconnect whose "connected" message carries the same revision', () => {
				const reloadSpy = vi.fn();
				vi.stubGlobal('window', {
					location: { protocol: 'http:', host: 'localhost:3000', reload: reloadSpy }
				});

				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'connected', revision: 'abc123' });

				// Simulate the transport dropping and a fresh socket opening in
				// its place, without going through the real onclose/reconnect
				// scheduling path (which uses a live setTimeout this test has no
				// need to wait out).
				if (mockWs) mockWs.readyState = MockWebSocket.CLOSED;
				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'connected', revision: 'abc123' });

				expect(reloadSpy).not.toHaveBeenCalled();
			});

			it('reloads on a reconnect whose "connected" message carries a different revision', () => {
				const reloadSpy = vi.fn();
				vi.stubGlobal('window', {
					location: { protocol: 'http:', host: 'localhost:3000', reload: reloadSpy }
				});

				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'connected', revision: 'abc123' });

				if (mockWs) mockWs.readyState = MockWebSocket.CLOSED;
				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'connected', revision: 'def456' });

				expect(reloadSpy).toHaveBeenCalledTimes(1);
			});
		});

		it('should handle "reload" message by reloading the page', () => {
			const reloadSpy = vi.fn();
			vi.stubGlobal('window', {
				location: {
					protocol: 'http:',
					host: 'localhost:3000',
					reload: reloadSpy
				}
			});

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'reload' });

			expect(reloadSpy).toHaveBeenCalled();
		});

		it('should handle "slide" message by navigating to slide', () => {
			// Set up a presentation with slides
			const testPresentation: Presentation = {
				config: {},
				slides: [
					{ index: 0, layout: 'default', html: '<p>Slide 1</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 },
					{ index: 1, layout: 'default', html: '<p>Slide 2</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 },
					{ index: 2, layout: 'default', html: '<p>Slide 3</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 }
				]
			};
			usePresentationStore.setState({ presentation: testPresentation });

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'slide', slideIndex: 2 });

			expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
		});

		it('should ignore "slide" message with undefined slideIndex', () => {
			const testPresentation: Presentation = {
				config: {},
				slides: [
					{ index: 0, layout: 'default', html: '<p>Slide 1</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 },
					{ index: 1, layout: 'default', html: '<p>Slide 2</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 }
				]
			};
			usePresentationStore.setState({ presentation: testPresentation, currentSlideIndex: 0 });

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'slide', slideIndex: undefined });

			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('buffers a "slide" message received before the presentation loads, and does not apply it yet', () => {
			usePresentationStore.setState({ presentation: null, currentSlideIndex: 0 });

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'slide', slideIndex: 5 });

			// Should remain at 0 since navigation should not occur yet.
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('applies a buffered late-joiner "slide" message once the presentation loads, overriding the initial slide index', () => {
			// Regression test for the hub's late-joiner state: a presenter
			// window opened mid-talk can have its websocket connection (and
			// this message) beat the presentation fetch. The buffered state
			// must still land once the presentation is set, as if it had
			// arrived after the fetch, overriding whatever slide index the
			// caller initialized the store with (a URL hash, in the real app).
			usePresentationStore.setState({ presentation: null, currentSlideIndex: 0 });

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'slide', slideIndex: 1, fragment: -1, step: 0, scrollRevealed: false, initial: true });

			const testPresentation: Presentation = {
				config: {},
				slides: [
					{ index: 0, layout: 'default', html: '<p>1</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 },
					{ index: 1, layout: 'default', html: '<p>2</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 }
				]
			};
			// Simulates loadPresentation's initializeFromURL setting slide 0
			// from the URL hash, then the presentation becoming available.
			usePresentationStore.setState({ currentSlideIndex: 0 });
			usePresentationStore.setState({ presentation: testPresentation });

			expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
		});

		it('should apply fragment, step and scroll reveal from a full state message', () => {
			const testPresentation: Presentation = {
				config: {},
				slides: [
					{ index: 0, layout: 'default', html: '<p>Slide 1</p>', slots: {}, slotOrder: [], fragmentCount: 3, steps: 2 },
					{ index: 1, layout: 'default', html: '<p>Slide 2</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 }
				]
			};
			usePresentationStore.setState({
				presentation: testPresentation,
				currentSlideIndex: 0,
				currentFragmentIndex: -1,
				currentStep: 0,
				scrollRevealed: false
			});

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'slide', slideIndex: 0, fragment: 1, step: 2, scrollRevealed: true });

			const state = usePresentationStore.getState();
			expect(state.currentSlideIndex).toBe(0);
			expect(state.currentFragmentIndex).toBe(1);
			expect(state.currentStep).toBe(2);
			expect(state.scrollRevealed).toBe(true);
		});

		it('should treat a legacy "slide" message without the new fields as "this slide, initial state"', () => {
			const testPresentation: Presentation = {
				config: {},
				slides: [
					{ index: 0, layout: 'default', html: '<p>Slide 1</p>', slots: {}, slotOrder: [], fragmentCount: 2, steps: 1 },
					{ index: 1, layout: 'default', html: '<p>Slide 2</p>', slots: {}, slotOrder: [], fragmentCount: 3, steps: 2 }
				]
			};
			usePresentationStore.setState({
				presentation: testPresentation,
				currentSlideIndex: 0,
				currentFragmentIndex: 1,
				currentStep: 1,
				scrollRevealed: true
			});

			client.connect();
			mockWs?.simulateOpen();
			// A legacy sender only ever includes slideIndex.
			mockWs?.simulateMessage({ type: 'slide', slideIndex: 1 });

			const state = usePresentationStore.getState();
			expect(state.currentSlideIndex).toBe(1);
			expect(state.currentFragmentIndex).toBe(-1);
			expect(state.currentStep).toBe(0);
			expect(state.scrollRevealed).toBe(false);
		});

		it('should ignore a "slide" message that exactly echoes the current state (echo guard)', () => {
			const testPresentation: Presentation = {
				config: {},
				slides: [
					{ index: 0, layout: 'default', html: '<p>Slide 1</p>', slots: {}, slotOrder: [], fragmentCount: 3, steps: 0 }
				]
			};
			usePresentationStore.setState({
				presentation: testPresentation,
				currentSlideIndex: 0,
				currentFragmentIndex: 1,
				currentStep: 0,
				scrollRevealed: false
			});

			const setStateSpy = vi.spyOn(usePresentationStore, 'setState');

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'slide', slideIndex: 0, fragment: 1, step: 0, scrollRevealed: false });

			expect(setStateSpy).not.toHaveBeenCalled();
		});

		it('should ignore invalid JSON messages', () => {
			client.connect();
			mockWs?.simulateOpen();

			// Should not throw when receiving invalid JSON
			if (mockWs?.onmessage) {
				mockWs.onmessage({ data: 'not valid json' });
			}

			expect(client.isConnected()).toBe(true);
		});
	});

	describe('auto-reconnect', () => {
		it('should schedule reconnect on connection close', () => {
			vi.useFakeTimers();

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateClose();

			// Advance past initial delay
			vi.advanceTimersByTime(WEBSOCKET_CONSTANTS.INITIAL_RECONNECT_DELAY + 100);

			// Should have attempted to reconnect (new WebSocket created)
			expect(WebSocket).toHaveBeenCalledTimes(2);
			vi.useRealTimers();
		});

		it('should use exponential backoff for reconnect delay', () => {
			client.connect();
			mockWs?.simulateOpen();

			expect(client.getReconnectDelay()).toBe(WEBSOCKET_CONSTANTS.INITIAL_RECONNECT_DELAY);

			// First close - delay increases
			mockWs?.simulateClose();
			expect(client.getReconnectDelay()).toBe(
				WEBSOCKET_CONSTANTS.INITIAL_RECONNECT_DELAY *
					WEBSOCKET_CONSTANTS.RECONNECT_BACKOFF_MULTIPLIER
			);
		});

		it('should cap reconnect delay at maximum', () => {
			// Set delay close to max
			client.connect();

			// Manually increase delay past max
			for (let i = 0; i < 10; i++) {
				mockWs?.simulateClose();
			}

			expect(client.getReconnectDelay()).toBeLessThanOrEqual(
				WEBSOCKET_CONSTANTS.MAX_RECONNECT_DELAY
			);
		});

		it('should reset reconnect delay on successful connection', () => {
			client.connect();
			mockWs?.simulateClose();

			// Delay should have increased
			expect(client.getReconnectDelay()).toBeGreaterThan(
				WEBSOCKET_CONSTANTS.INITIAL_RECONNECT_DELAY
			);

			// Reconnect successfully
			client.connect();
			mockWs?.simulateOpen();

			// Delay should be reset
			expect(client.getReconnectDelay()).toBe(WEBSOCKET_CONSTANTS.INITIAL_RECONNECT_DELAY);
		});
	});

	describe('send', () => {
		it('should send message when connected', () => {
			client.connect();
			mockWs?.simulateOpen();

			const sendSpy = vi.spyOn(mockWs!, 'send');
			const message: WebSocketMessage = { type: 'slide', slideIndex: 5 };
			client.send(message);

			expect(sendSpy).toHaveBeenCalledWith(JSON.stringify(message));
		});

		it('should not send message when disconnected', () => {
			client.connect();
			// Don't open connection

			const sendSpy = vi.spyOn(mockWs!, 'send');
			const message: WebSocketMessage = { type: 'slide', slideIndex: 5 };
			client.send(message);

			expect(sendSpy).not.toHaveBeenCalled();
		});
	});

	describe('isConnected', () => {
		it('should return false when not connected', () => {
			expect(client.isConnected()).toBe(false);
		});

		it('should return true when connected', () => {
			client.connect();
			mockWs?.simulateOpen();
			expect(client.isConnected()).toBe(true);
		});

		it('should return false after disconnect', () => {
			client.connect();
			mockWs?.simulateOpen();
			client.disconnect();
			expect(client.isConnected()).toBe(false);
		});
	});
});

describe('singleton functions', () => {
	beforeEach(() => {
		vi.stubGlobal('WebSocket', createMockWebSocketConstructor());

		vi.stubGlobal('window', {
			location: {
				protocol: 'http:',
				host: 'localhost:3000',
				reload: vi.fn()
			}
		});

		useConnectionStore.setState({ connected: false });
	});

	afterEach(() => {
		disconnectWebSocket();
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
	});

	it('should return same client instance from getWebSocketClient', () => {
		const client1 = getWebSocketClient();
		const client2 = getWebSocketClient();
		expect(client1).toBe(client2);
	});

	it('should connect via connectWebSocket', () => {
		connectWebSocket();
		expect(WebSocket).toHaveBeenCalled();
	});

	it('should disconnect via disconnectWebSocket', () => {
		connectWebSocket();
		disconnectWebSocket();

		expect(useConnectionStore.getState().connected).toBe(false);
	});
});

describe('connection store', () => {
	it('should default to disconnected', () => {
		useConnectionStore.setState({ connected: false }); // Reset
		expect(useConnectionStore.getState().connected).toBe(false);
	});
});

describe('broadcastPresentationState', () => {
	let mockWs: MockWebSocket | null = null;

	beforeEach(() => {
		vi.stubGlobal(
			'WebSocket',
			createMockWebSocketConstructor((ws) => {
				mockWs = ws;
			})
		);
		vi.stubGlobal('window', {
			location: { protocol: 'http:', host: 'localhost:3000', reload: vi.fn() },
			history: { replaceState: vi.fn() }
		});
		useConnectionStore.setState({ connected: false });
		usePresentationStore.setState({
			presentation: null,
			currentSlideIndex: 0,
			currentFragmentIndex: -1,
			currentStep: 0,
			scrollRevealed: false
		});
		resetBroadcastedState();
		resetPendingInitialState();
		connectWebSocket();
		mockWs?.simulateOpen();
	});

	afterEach(() => {
		disconnectWebSocket();
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
		mockWs = null;
	});

	it('should send the full current state', () => {
		usePresentationStore.setState({ currentSlideIndex: 2, currentFragmentIndex: 1, currentStep: 3, scrollRevealed: true });

		const sendSpy = vi.spyOn(mockWs!, 'send');
		broadcastPresentationState();

		expect(sendSpy).toHaveBeenCalledWith(
			JSON.stringify({ type: 'slide', slideIndex: 2, fragment: 1, step: 3, scrollRevealed: true })
		);
	});

	it('should not resend when the state has not changed since the last broadcast', () => {
		usePresentationStore.setState({ currentSlideIndex: 1 });
		broadcastPresentationState();

		const sendSpy = vi.spyOn(mockWs!, 'send');
		broadcastPresentationState();

		expect(sendSpy).not.toHaveBeenCalled();
	});

	it('should resend once the state changes again', () => {
		usePresentationStore.setState({ currentSlideIndex: 1 });
		broadcastPresentationState();

		usePresentationStore.setState({ currentFragmentIndex: 0 });
		const sendSpy = vi.spyOn(mockWs!, 'send');
		broadcastPresentationState();

		expect(sendSpy).toHaveBeenCalledWith(
			JSON.stringify({ type: 'slide', slideIndex: 1, fragment: 0, step: 0, scrollRevealed: false })
		);
	});

	it('should resend a local navigation back to a slide a remote client just pushed on us (local, remote, local-back)', () => {
		// Regression test for the dedupe only tracking sends: viewer -> 2,
		// presenter -> 3 (viewer applies remote state), viewer -> back to 2
		// must still send, even though "2" was this client's very first
		// broadcast.
		const testPresentation: Presentation = {
			config: {},
			slides: [
				{ index: 0, layout: 'default', html: '<p>1</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 },
				{ index: 1, layout: 'default', html: '<p>2</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 },
				{ index: 2, layout: 'default', html: '<p>3</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 },
				{ index: 3, layout: 'default', html: '<p>4</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0 }
			]
		};
		usePresentationStore.setState({ presentation: testPresentation });

		// Local: viewer navigates to slide 2 and broadcasts it.
		usePresentationStore.setState({ currentSlideIndex: 2 });
		broadcastPresentationState();

		// Remote: presenter navigates to slide 3, viewer applies the remote state.
		mockWs?.simulateMessage({ type: 'slide', slideIndex: 3, fragment: -1, step: 0, scrollRevealed: false });
		expect(usePresentationStore.getState().currentSlideIndex).toBe(3);

		// Local-back: viewer navigates back to slide 2 and must broadcast again.
		usePresentationStore.setState({ currentSlideIndex: 2 });
		const sendSpy = vi.spyOn(mockWs!, 'send');
		broadcastPresentationState();

		expect(sendSpy).toHaveBeenCalledWith(
			JSON.stringify({ type: 'slide', slideIndex: 2, fragment: -1, step: 0, scrollRevealed: false })
		);
	});
});

describe('detectStaticMode', () => {
	afterEach(() => {
		document.getElementById('presentation-data')?.remove();
		vi.unstubAllGlobals();
		vi.restoreAllMocks();
	});

	it('should detect static mode from the embedded #presentation-data element without a network request', async () => {
		const script = document.createElement('script');
		script.id = 'presentation-data';
		script.type = 'application/json';
		script.textContent = '{"slides":[]}';
		document.body.appendChild(script);

		const fetchSpy = vi.fn();
		vi.stubGlobal('fetch', fetchSpy);

		const isStatic = await detectStaticMode();

		expect(isStatic).toBe(true);
		expect(useConnectionStore.getState().staticMode).toBe(true);
		expect(useConnectionStore.getState().staticModeDetected).toBe(true);
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it('should detect dev mode when no embedded element is present and the API responds', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({ ok: true })
		);

		const isStatic = await detectStaticMode();

		expect(isStatic).toBe(false);
		expect(useConnectionStore.getState().staticMode).toBe(false);
		expect(useConnectionStore.getState().staticModeDetected).toBe(true);
	});

	it('should detect static mode when no embedded element is present and the API request fails', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockRejectedValue(new Error('network error'))
		);

		const isStatic = await detectStaticMode();

		expect(isStatic).toBe(true);
		expect(useConnectionStore.getState().staticMode).toBe(true);
	});
});

describe('hub late-joiner state vs the URL hash', () => {
	let mockWs: MockWebSocket | null = null;

	/** Stub `window` with a given URL hash, the way a real page load would set it. */
	function stubWindowWithHash(hash: string): void {
		vi.stubGlobal('window', {
			location: { protocol: 'http:', host: 'localhost:3000', hash },
			history: { replaceState: vi.fn() },
			addEventListener: vi.fn(),
			removeEventListener: vi.fn()
		});
	}

	function makePresentation(slideCount: number): Presentation {
		return {
			config: {},
			slides: Array.from({ length: slideCount }, (_, i) => ({
				index: i,
				layout: 'default',
				html: `<p>${i + 1}</p>`,
				slots: {},
				slotOrder: [],
				fragmentCount: 2,
				steps: 0
			}))
		};
	}

	beforeEach(() => {
		resetHashSlideIndexAtLoad();
		resetPendingInitialState();
		resetBroadcastedState();
		usePresentationStore.setState({ presentation: null, currentSlideIndex: 0, currentFragmentIndex: -1 });
	});

	afterEach(() => {
		disconnectWebSocket();
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
		mockWs = null;
	});

	it('applies the hub state fully when the page loaded with no hash', () => {
		stubWindowWithHash('');

		vi.stubGlobal(
			'WebSocket',
			createMockWebSocketConstructor((ws) => {
				mockWs = ws;
			})
		);
		connectWebSocket();
		mockWs?.simulateOpen();

		// Late-joiner message (initial: true) arrives before the presentation has loaded.
		mockWs?.simulateMessage({ type: 'slide', slideIndex: 2, fragment: 1, step: 0, scrollRevealed: false, initial: true });

		// loadPresentation records the hash and runs initializeFromURL, the
		// same sequence App.tsx/PresenterApp.tsx run on a real page load.
		loadPresentation(makePresentation(5));

		expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
		expect(usePresentationStore.getState().currentFragmentIndex).toBe(1);
	});

	it('applies a live navigation immediately when the hub has no state yet, even as the first message received', () => {
		// Regression test: when nobody has navigated yet, the hub sends no
		// register-time state at all (see internal/server/websocket.go), so
		// the very first "slide" message a client receives can be a live
		// navigation broadcast by another client, not the hub's own state.
		// This is keyed off the message's own `initial` field, not off
		// arrival order, so a live navigation that happens to arrive first
		// is never wrongly weighed against the hash as if it were the hub's
		// late-joiner state.
		stubWindowWithHash('#5'); // 0-based index 4

		vi.stubGlobal(
			'WebSocket',
			createMockWebSocketConstructor((ws) => {
				mockWs = ws;
			})
		);
		connectWebSocket();
		mockWs?.simulateOpen();

		// A live navigation (no `initial` field) arrives first, before the
		// presentation has loaded, racing the presentation fetch exactly
		// like a hub late-joiner state would - but this one is not one.
		mockWs?.simulateMessage({ type: 'slide', slideIndex: 2, fragment: -1, step: 0, scrollRevealed: false });

		loadPresentation(makePresentation(5));

		// It must be applied outright, the same as any other live
		// navigation, regardless of the hash naming a different slide (4).
		expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
	});

	it('takes the hub fragment/step/scroll but keeps the slide when the hash names the same slide', () => {
		stubWindowWithHash('#3'); // 0-based index 2, matches the hub's slide below

		vi.stubGlobal(
			'WebSocket',
			createMockWebSocketConstructor((ws) => {
				mockWs = ws;
			})
		);
		connectWebSocket();
		mockWs?.simulateOpen();

		mockWs?.simulateMessage({ type: 'slide', slideIndex: 2, fragment: 1, step: 0, scrollRevealed: false, initial: true });

		loadPresentation(makePresentation(5));

		expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
		expect(usePresentationStore.getState().currentFragmentIndex).toBe(1);
	});

	it('leaves the hash slide in its initial state when the hash names a different slide than the hub', () => {
		stubWindowWithHash('#5'); // 0-based index 4

		vi.stubGlobal(
			'WebSocket',
			createMockWebSocketConstructor((ws) => {
				mockWs = ws;
			})
		);
		connectWebSocket();
		mockWs?.simulateOpen();

		// Hub's late-joiner state (initial: true) names slide index 1 (a different slide).
		mockWs?.simulateMessage({ type: 'slide', slideIndex: 1, fragment: 1, step: 0, scrollRevealed: false, initial: true });

		loadPresentation(makePresentation(5));

		const state = usePresentationStore.getState();
		expect(state.currentSlideIndex).toBe(4);
		expect(state.currentFragmentIndex).toBe(-1);
	});

	it('does not let a discarded hub state block the first real broadcast after a hash-wins load', () => {
		// Regression test for the send-side dedupe: when the hash wins over
		// the hub's late-joiner state, nothing should be remembered as "our"
		// last-broadcast state, so the very next local navigation still sends.
		stubWindowWithHash('#5'); // 0-based index 4

		vi.stubGlobal(
			'WebSocket',
			createMockWebSocketConstructor((ws) => {
				mockWs = ws;
			})
		);
		connectWebSocket();
		mockWs?.simulateOpen();

		// Hub's late-joiner state (initial: true) names a different slide (index 1); discarded.
		mockWs?.simulateMessage({ type: 'slide', slideIndex: 1, fragment: 0, step: 0, scrollRevealed: false, initial: true });

		loadPresentation(makePresentation(5));

		expect(usePresentationStore.getState().currentSlideIndex).toBe(4);

		// The user navigates locally, back to the exact slide/fragment the
		// hub state named (index 1). Without the fix, this send would be
		// wrongly skipped because it looks identical to a remembered state.
		usePresentationStore.setState({ currentSlideIndex: 1, currentFragmentIndex: 0 });
		const sendSpy = vi.spyOn(mockWs!, 'send');
		broadcastPresentationState();

		expect(sendSpy).toHaveBeenCalledWith(
			JSON.stringify({ type: 'slide', slideIndex: 1, fragment: 0, step: 0, scrollRevealed: false })
		);
	});

	it('lets the hub state win outright on a reconnect, even when it names a different slide than the load hash', () => {
		// Regression test: presenter loads at #2 (0-based index 1). The first
		// `initial` message matches the hash, so it applies. The presenter's
		// socket then drops and a second `initial` message arrives on
		// reconnect - the hub has since moved to slide 3 (index 3) because
		// the viewer navigated forward while the presenter was offline. This
		// second `initial` message must win outright: the load-time hash
		// only ever gets to arbitrate the first one.
		stubWindowWithHash('#2'); // 0-based index 1

		vi.stubGlobal(
			'WebSocket',
			createMockWebSocketConstructor((ws) => {
				mockWs = ws;
			})
		);
		connectWebSocket();
		mockWs?.simulateOpen();

		// First `initial` message: matches the hash, applies normally.
		mockWs?.simulateMessage({ type: 'slide', slideIndex: 1, fragment: -1, step: 0, scrollRevealed: false, initial: true });
		loadPresentation(makePresentation(5));
		expect(usePresentationStore.getState().currentSlideIndex).toBe(1);

		// Reconnect: the hub's second `initial` message names a different
		// slide (index 3). It must win outright, not be discarded against
		// the load-time hash.
		mockWs?.simulateMessage({ type: 'slide', slideIndex: 3, fragment: -1, step: 0, scrollRevealed: false, initial: true });
		expect(usePresentationStore.getState().currentSlideIndex).toBe(3);
	});

	it('a reconnect still lets the hub state win outright even when the first connection never received an initial message at all', () => {
		// Regression test: a presenter loads at #1 onto an empty hub -
		// nobody has navigated yet, so the hub sends no `initial` message
		// on that first connection, and hasResolvedInitialHubState is never
		// set from a message. The socket then drops; by the time it
		// reconnects, the hub has state (the audience moved on) and sends
		// its first-ever `initial` message, on this reconnect. It must
		// still win outright, not be discarded against the load-time hash,
		// even though no earlier message ever flipped
		// hasResolvedInitialHubState - see WebSocketClient's
		// hasOpenedBefore, set in onopen rather than onmessage for exactly
		// this case.
		vi.useFakeTimers();
		stubWindowWithHash('#1'); // 0-based index 0

		vi.stubGlobal(
			'WebSocket',
			createMockWebSocketConstructor((ws) => {
				mockWs = ws;
			})
		);
		connectWebSocket();
		mockWs?.simulateOpen();
		loadPresentation(makePresentation(5));
		// No `initial` message at all: the hub had nothing to send.

		mockWs?.simulateClose();
		vi.advanceTimersByTime(WEBSOCKET_CONSTANTS.INITIAL_RECONNECT_DELAY + 100);
		mockWs?.simulateOpen();

		// The hub's first-ever `initial` message, on the reconnect, names
		// a different slide (index 2) than the load hash (index 0).
		mockWs?.simulateMessage({ type: 'slide', slideIndex: 2, fragment: -1, step: 0, scrollRevealed: false, initial: true });
		expect(usePresentationStore.getState().currentSlideIndex).toBe(2);

		vi.useRealTimers();
	});
});

describe('WEBSOCKET_CONSTANTS', () => {
	it('should export configuration constants', () => {
		expect(WEBSOCKET_CONSTANTS.INITIAL_RECONNECT_DELAY).toBe(1000);
		expect(WEBSOCKET_CONSTANTS.MAX_RECONNECT_DELAY).toBe(30000);
		expect(WEBSOCKET_CONSTANTS.RECONNECT_BACKOFF_MULTIPLIER).toBe(2);
	});
});
