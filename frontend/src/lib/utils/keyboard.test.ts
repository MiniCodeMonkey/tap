/**
 * Unit tests for keyboard navigation.
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
import { useConnectionStore } from '$lib/stores/websocket';

// Mock the connection store: keyboard.ts only reads presentMode off it, and
// the real module's top-level usePresentationStore.subscribe call would
// otherwise blow up against the mocked presentation store above.
vi.mock('$lib/stores/websocket', async () => {
	const { create } = await import('zustand');
	return { useConnectionStore: create(() => ({ presentMode: false })) };
});

// ============================================================================
// Mock Setup
// ============================================================================

// Mock the presentation store
vi.mock('$lib/stores/presentation', () => ({
	nextSlide: vi.fn(() => true),
	prevSlide: vi.fn(() => true),
	goToSlide: vi.fn(() => true),
	usePresentationStore: { getState: vi.fn(() => ({})) },
	selectTotalSlides: vi.fn(() => 10),
	cycleTheme: vi.fn()
}));

describe('keyboard navigation', () => {
	let cleanup: () => void;
	let keydownHandler: ((event: KeyboardEvent) => void) | null = null;

	beforeEach(() => {
		vi.clearAllMocks();
		useConnectionStore.setState({ presentMode: false });

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

		it('should cycle the theme on T', () => {
			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 't' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(presentationStore.cycleTheme).toHaveBeenCalled();
			expect(preventDefaultSpy).toHaveBeenCalled();
		});

		it('should not cycle the theme on T while an input is focused', () => {
			const inputElement = document.createElement('input');
			Object.defineProperty(document, 'activeElement', {
				value: inputElement,
				configurable: true
			});

			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 't' });
			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(presentationStore.cycleTheme).not.toHaveBeenCalled();
		});

		it('should not cycle the theme on T during tap present', () => {
			useConnectionStore.setState({ presentMode: true });
			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 't' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');
			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(presentationStore.cycleTheme).not.toHaveBeenCalled();
			expect(preventDefaultSpy).not.toHaveBeenCalled();

			useConnectionStore.setState({ presentMode: false });
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

		it('should blur a focused editable element instead of toggling overview or exiting fullscreen', () => {
			const inputElement = document.createElement('input');
			document.body.appendChild(inputElement);
			inputElement.focus();
			Object.defineProperty(document, 'activeElement', {
				value: inputElement,
				configurable: true
			});
			Object.defineProperty(document, 'fullscreenElement', {
				value: document.documentElement,
				configurable: true
			});
			const blurSpy = vi.spyOn(inputElement, 'blur');
			const onToggleOverview = vi.fn();

			cleanup = setupKeyboardNavigation({ onToggleOverview, isOverviewOpen: () => true });

			const event = new KeyboardEvent('keydown', { key: 'Escape' });
			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(blurSpy).toHaveBeenCalled();
			expect(onToggleOverview).not.toHaveBeenCalled();
			expect(document.exitFullscreen).not.toHaveBeenCalled();

			inputElement.remove();
		});
	});

	describe('shortcut overlay', () => {
		function press(key: string): void {
			keydownHandler?.(new KeyboardEvent('keydown', { key }));
		}

		it('should open the overlay on ?', () => {
			const onToggleHelp = vi.fn();
			cleanup = setupKeyboardNavigation({ onToggleHelp, isHelpOpen: () => false });

			const event = new KeyboardEvent('keydown', { key: '?' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');
			keydownHandler?.(event);

			expect(onToggleHelp).toHaveBeenCalledTimes(1);
			expect(preventDefaultSpy).toHaveBeenCalled();
		});

		it('should close the overlay on ? or Escape while it is open', () => {
			const onToggleHelp = vi.fn();
			cleanup = setupKeyboardNavigation({ onToggleHelp, isHelpOpen: () => true });

			press('?');
			press('Escape');

			expect(onToggleHelp).toHaveBeenCalledTimes(2);
		});

		it('should ignore every other shortcut while the overlay is open', () => {
			const onToggleHelp = vi.fn();
			const onToggleOverview = vi.fn();
			cleanup = setupKeyboardNavigation({ onToggleHelp, onToggleOverview, isHelpOpen: () => true });

			for (const key of ['ArrowRight', 'ArrowLeft', 'Home', 'End', 'o', 't', 'f', 's']) {
				press(key);
			}

			expect(presentationStore.nextSlide).not.toHaveBeenCalled();
			expect(presentationStore.prevSlide).not.toHaveBeenCalled();
			expect(presentationStore.goToSlide).not.toHaveBeenCalled();
			expect(presentationStore.cycleTheme).not.toHaveBeenCalled();
			expect(onToggleOverview).not.toHaveBeenCalled();
			expect(onToggleHelp).not.toHaveBeenCalled();
		});

		it('should close only the overlay on Escape, not exit fullscreen', () => {
			Object.defineProperty(document, 'fullscreenElement', {
				value: document.documentElement,
				configurable: true
			});
			const onToggleHelp = vi.fn();
			cleanup = setupKeyboardNavigation({ onToggleHelp, isHelpOpen: () => true });

			press('Escape');

			expect(onToggleHelp).toHaveBeenCalledTimes(1);
			expect(document.exitFullscreen).not.toHaveBeenCalled();
		});

		it('should not open the overlay while the overview is open', () => {
			const onToggleHelp = vi.fn();
			cleanup = setupKeyboardNavigation({ onToggleHelp, isHelpOpen: () => false, isOverviewOpen: () => true });

			press('?');

			expect(onToggleHelp).not.toHaveBeenCalled();
		});

		it('should not open the overlay while an input is focused', () => {
			const input = document.createElement('input');
			Object.defineProperty(document, 'activeElement', { value: input, configurable: true });
			const onToggleHelp = vi.fn();
			cleanup = setupKeyboardNavigation({ onToggleHelp, isHelpOpen: () => false });

			press('?');

			expect(onToggleHelp).not.toHaveBeenCalled();
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
