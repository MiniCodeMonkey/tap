/**
 * Unit tests for WebSocket client.
 * These tests will run with Vitest (US-075).
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
	WebSocketClient,
	connected,
	WEBSOCKET_CONSTANTS,
	getWebSocketClient,
	connectWebSocket,
	disconnectWebSocket
} from './websocket';
import { presentation, currentSlideIndex, currentFragmentIndex } from './presentation';
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

// Store original WebSocket
const originalWebSocket = globalThis.WebSocket;

describe('WebSocketClient', () => {
	let mockWs: MockWebSocket | null = null;
	let client: WebSocketClient;

	beforeEach(() => {
		// Mock WebSocket constructor
		vi.stubGlobal(
			'WebSocket',
			vi.fn().mockImplementation((url: string) => {
				mockWs = new MockWebSocket(url);
				return mockWs;
			})
		);

		// Mock window.location
		vi.stubGlobal('window', {
			location: {
				protocol: 'http:',
				host: 'localhost:3000',
				reload: vi.fn()
			}
		});

		// Reset stores
		connected.set(false);
		presentation.set(null);
		currentSlideIndex.set(0);
		currentFragmentIndex.set(-1);

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
			let isConnected = false;
			const unsubscribe = connected.subscribe((value) => {
				isConnected = value;
			});

			client.connect();
			mockWs?.simulateOpen();

			expect(isConnected).toBe(true);
			unsubscribe();
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
			let isConnected = true;
			const unsubscribe = connected.subscribe((value) => {
				isConnected = value;
			});

			client.connect();
			mockWs?.simulateOpen();
			client.disconnect();

			expect(isConnected).toBe(false);
			unsubscribe();
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
					{ index: 0, layout: 'default', html: '<p>Slide 1</p>' },
					{ index: 1, layout: 'default', html: '<p>Slide 2</p>' },
					{ index: 2, layout: 'default', html: '<p>Slide 3</p>' }
				]
			};
			presentation.set(testPresentation);

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'slide', slideIndex: 2 });

			let currentIndex = 0;
			const unsubscribe = currentSlideIndex.subscribe((value) => {
				currentIndex = value;
			});

			expect(currentIndex).toBe(2);
			unsubscribe();
		});

		it('should ignore "slide" message with undefined slideIndex', () => {
			const testPresentation: Presentation = {
				config: {},
				slides: [
					{ index: 0, layout: 'default', html: '<p>Slide 1</p>' },
					{ index: 1, layout: 'default', html: '<p>Slide 2</p>' }
				]
			};
			presentation.set(testPresentation);
			currentSlideIndex.set(0);

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'slide', slideIndex: undefined });

			let currentIndex = -1;
			const unsubscribe = currentSlideIndex.subscribe((value) => {
				currentIndex = value;
			});

			expect(currentIndex).toBe(0);
			unsubscribe();
		});

		it('should ignore "slide" message when no presentation is loaded', () => {
			presentation.set(null);
			currentSlideIndex.set(0);

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'slide', slideIndex: 5 });

			let currentIndex = -1;
			const unsubscribe = currentSlideIndex.subscribe((value) => {
				currentIndex = value;
			});

			// Should remain at 0 since navigation should not occur
			expect(currentIndex).toBe(0);
			unsubscribe();
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
		vi.stubGlobal(
			'WebSocket',
			vi.fn().mockImplementation((url: string) => {
				return new MockWebSocket(url);
			})
		);

		vi.stubGlobal('window', {
			location: {
				protocol: 'http:',
				host: 'localhost:3000',
				reload: vi.fn()
			}
		});

		connected.set(false);
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

		let isConnected = true;
		const unsubscribe = connected.subscribe((value) => {
			isConnected = value;
		});

		expect(isConnected).toBe(false);
		unsubscribe();
	});
});

describe('connected store', () => {
	it('should export a writable store', () => {
		expect(connected).toBeDefined();
		expect(typeof connected.subscribe).toBe('function');
		expect(typeof connected.set).toBe('function');
	});

	it('should default to false', () => {
		connected.set(false); // Reset
		let isConnected = true;
		const unsubscribe = connected.subscribe((value) => {
			isConnected = value;
		});
		expect(isConnected).toBe(false);
		unsubscribe();
	});
});

describe('WEBSOCKET_CONSTANTS', () => {
	it('should export configuration constants', () => {
		expect(WEBSOCKET_CONSTANTS.INITIAL_RECONNECT_DELAY).toBe(1000);
		expect(WEBSOCKET_CONSTANTS.MAX_RECONNECT_DELAY).toBe(30000);
		expect(WEBSOCKET_CONSTANTS.RECONNECT_BACKOFF_MULTIPLIER).toBe(2);
	});
});
