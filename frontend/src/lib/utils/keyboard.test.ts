/**
 * Unit tests for keyboard navigation.
 * These tests will run once Vitest is configured (US-075).
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	setupKeyboardNavigation,
	triggerNext,
	triggerPrev,
	triggerFullscreen,
	checkFullscreen
} from './keyboard';
import * as presentationStore from '$lib/stores/presentation';

// ============================================================================
// Mock Setup
// ============================================================================

// Mock the presentation store
vi.mock('$lib/stores/presentation', () => ({
	nextSlide: vi.fn(() => true),
	prevSlide: vi.fn(() => true),
	goToSlide: vi.fn(() => true),
	totalSlides: { subscribe: vi.fn((fn: (value: number) => void) => { fn(10); return () => {}; }) }
}));

// Mock svelte/store get function
vi.mock('svelte/store', () => ({
	get: vi.fn(() => 10)
}));

describe('keyboard navigation', () => {
	let cleanup: () => void;
	let keydownHandler: ((event: KeyboardEvent) => void) | null = null;

	beforeEach(() => {
		vi.clearAllMocks();

		// Mock window.addEventListener to capture the handler
		const originalAddEventListener = window.addEventListener;
		vi.spyOn(window, 'addEventListener').mockImplementation(
			(type: string, handler: EventListenerOrEventListenerObject) => {
				if (type === 'keydown' && typeof handler === 'function') {
					keydownHandler = handler as (event: KeyboardEvent) => void;
				}
				originalAddEventListener.call(window, type, handler);
			}
		);

		// Mock document.activeElement
		Object.defineProperty(document, 'activeElement', {
			value: document.body,
			configurable: true
		});

		// Mock fullscreen API
		Object.defineProperty(document, 'fullscreenElement', {
			value: null,
			configurable: true
		});

		document.documentElement.requestFullscreen = vi.fn().mockResolvedValue(undefined);
		document.exitFullscreen = vi.fn().mockResolvedValue(undefined);
	});

	afterEach(() => {
		if (cleanup) {
			cleanup();
		}
		vi.restoreAllMocks();
	});

	describe('setupKeyboardNavigation', () => {
		it('should add keydown event listener', () => {
			cleanup = setupKeyboardNavigation();
			expect(window.addEventListener).toHaveBeenCalledWith('keydown', expect.any(Function));
		});

		it('should return cleanup function', () => {
			cleanup = setupKeyboardNavigation();
			expect(typeof cleanup).toBe('function');
		});

		it('should remove event listener on cleanup', () => {
			const removeEventListenerSpy = vi.spyOn(window, 'removeEventListener');
			cleanup = setupKeyboardNavigation();
			cleanup();
			expect(removeEventListenerSpy).toHaveBeenCalledWith('keydown', expect.any(Function));
		});
	});

	describe('advance keys', () => {
		const advanceKeys = ['ArrowRight', 'ArrowDown', ' ', 'Enter', 'PageDown'];

		advanceKeys.forEach((key) => {
			it(`should call nextSlide on ${key === ' ' ? 'Space' : key}`, () => {
				cleanup = setupKeyboardNavigation();

				const event = new KeyboardEvent('keydown', { key });
				const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

				if (keydownHandler) {
					keydownHandler(event);
				}

				expect(presentationStore.nextSlide).toHaveBeenCalled();
				expect(preventDefaultSpy).toHaveBeenCalled();
			});
		});
	});

	describe('retreat keys', () => {
		const retreatKeys = ['ArrowLeft', 'ArrowUp', 'Backspace', 'PageUp'];

		retreatKeys.forEach((key) => {
			it(`should call prevSlide on ${key}`, () => {
				cleanup = setupKeyboardNavigation();

				const event = new KeyboardEvent('keydown', { key });
				const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

				if (keydownHandler) {
					keydownHandler(event);
				}

				expect(presentationStore.prevSlide).toHaveBeenCalled();
				expect(preventDefaultSpy).toHaveBeenCalled();
			});
		});
	});

	describe('navigation keys', () => {
		it('should go to first slide on Home', () => {
			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 'Home' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(presentationStore.goToSlide).toHaveBeenCalledWith(0);
			expect(preventDefaultSpy).toHaveBeenCalled();
		});

		it('should go to last slide on End', () => {
			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 'End' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(presentationStore.goToSlide).toHaveBeenCalledWith(9); // 10 - 1
			expect(preventDefaultSpy).toHaveBeenCalled();
		});
	});

	describe('special keys', () => {
		it('should open presenter view on S', () => {
			const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 's' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(openSpy).toHaveBeenCalled();
			expect(preventDefaultSpy).toHaveBeenCalled();
		});

		it('should call custom onOpenPresenter callback', () => {
			const onOpenPresenter = vi.fn();
			cleanup = setupKeyboardNavigation({ onOpenPresenter });

			const event = new KeyboardEvent('keydown', { key: 'S' });

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(onOpenPresenter).toHaveBeenCalled();
		});

		it('should toggle overview on O', () => {
			const onToggleOverview = vi.fn();
			cleanup = setupKeyboardNavigation({ onToggleOverview });

			const event = new KeyboardEvent('keydown', { key: 'o' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(onToggleOverview).toHaveBeenCalled();
			expect(preventDefaultSpy).toHaveBeenCalled();
		});

		it('should toggle fullscreen on F', async () => {
			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 'f' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

			if (keydownHandler) {
				keydownHandler(event);
			}

			// Wait for async fullscreen request
			await vi.waitFor(() => {
				expect(document.documentElement.requestFullscreen).toHaveBeenCalled();
			});
			expect(preventDefaultSpy).toHaveBeenCalled();
		});
	});

	describe('escape key', () => {
		it('should close overview when open', () => {
			const onToggleOverview = vi.fn();
			cleanup = setupKeyboardNavigation({
				onToggleOverview,
				isOverviewOpen: () => true
			});

			const event = new KeyboardEvent('keydown', { key: 'Escape' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(onToggleOverview).toHaveBeenCalled();
			expect(preventDefaultSpy).toHaveBeenCalled();
		});

		it('should exit fullscreen when in fullscreen mode', () => {
			Object.defineProperty(document, 'fullscreenElement', {
				value: document.documentElement,
				configurable: true
			});

			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 'Escape' });

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(document.exitFullscreen).toHaveBeenCalled();
		});
	});

	describe('input focus handling', () => {
		it('should skip navigation when input is focused', () => {
			const inputElement = document.createElement('input');
			Object.defineProperty(document, 'activeElement', {
				value: inputElement,
				configurable: true
			});

			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 'ArrowRight' });

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});

		it('should skip navigation when textarea is focused', () => {
			const textareaElement = document.createElement('textarea');
			Object.defineProperty(document, 'activeElement', {
				value: textareaElement,
				configurable: true
			});

			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 'ArrowRight' });

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});

		it('should skip navigation when contenteditable is focused', () => {
			const editableDiv = document.createElement('div');
			editableDiv.setAttribute('contenteditable', 'true');
			Object.defineProperty(document, 'activeElement', {
				value: editableDiv,
				configurable: true
			});

			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 'ArrowRight' });

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
		});
	});

	describe('trigger functions', () => {
		it('triggerNext should call nextSlide', () => {
			const result = triggerNext();
			expect(presentationStore.nextSlide).toHaveBeenCalled();
			expect(result).toBe(true);
		});

		it('triggerPrev should call prevSlide', () => {
			const result = triggerPrev();
			expect(presentationStore.prevSlide).toHaveBeenCalled();
			expect(result).toBe(true);
		});

		it('triggerFullscreen should toggle fullscreen', async () => {
			await triggerFullscreen();
			expect(document.documentElement.requestFullscreen).toHaveBeenCalled();
		});

		it('checkFullscreen should return fullscreen state', () => {
			expect(checkFullscreen()).toBe(false);

			Object.defineProperty(document, 'fullscreenElement', {
				value: document.documentElement,
				configurable: true
			});

			expect(checkFullscreen()).toBe(true);
		});
	});
});
