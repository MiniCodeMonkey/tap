import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

// Mock window.matchMedia for components that check for reduced motion
Object.defineProperty(window, 'matchMedia', {
	writable: true,
	value: vi.fn().mockImplementation((query: string) => ({
		matches: false,
		media: query,
		onchange: null,
		addListener: vi.fn(), // deprecated
		removeListener: vi.fn(), // deprecated
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		dispatchEvent: vi.fn()
	}))
});

// Mock ResizeObserver for SlideContainer
class MockResizeObserver {
	observe = vi.fn();
	unobserve = vi.fn();
	disconnect = vi.fn();
}

Object.defineProperty(window, 'ResizeObserver', {
	writable: true,
	value: MockResizeObserver
});

// Mock getBoundingClientRect for scaling tests
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
