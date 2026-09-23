import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

// Mock window.matchMedia for components that check for reduced motion.
// jsdom has no matchMedia of its own, so a stub is defined first and then
// spied on: vi.spyOn, unlike a plain vi.fn(), is undone by a test's
// vi.restoreAllMocks(), so a test's own mockImplementation cannot leak into
// the tests that run after it.
Object.defineProperty(window, 'matchMedia', {
	writable: true,
	configurable: true,
	value: (query: string) => ({
		matches: false,
		media: query,
		onchange: null,
		addListener: vi.fn(),
		removeListener: vi.fn(),
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		dispatchEvent: vi.fn()
	})
});
vi.spyOn(window, 'matchMedia');

// Mock ResizeObserver for SlideCanvas.
class MockResizeObserver {
	observe = vi.fn();
	unobserve = vi.fn();
	disconnect = vi.fn();
}

Object.defineProperty(window, 'ResizeObserver', {
	writable: true,
	value: MockResizeObserver
});

// Mock getBoundingClientRect for scaling tests.
Element.prototype.getBoundingClientRect = vi.fn(() => ({
	width: 1920,
	height: 1080,
	top: 0,
	left: 0,
	bottom: 1080,
	right: 1920,
	x: 0,
	y: 0,
	toJSON: () => ({})
}));
