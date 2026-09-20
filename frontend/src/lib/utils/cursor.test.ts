/**
 * Unit tests for hiding the pointer on an idle fullscreen deck.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { setupCursorAutoHide } from './cursor';

/** Pretend the document is, or is not, fullscreen. */
function setFullscreen(on: boolean): void {
	Object.defineProperty(document, 'fullscreenElement', {
		value: on ? document.documentElement : null,
		configurable: true
	});
}

const hidden = (): boolean => document.body.classList.contains('cursor-hidden');

describe('cursor auto-hide', () => {
	let cleanup: () => void = () => {};

	beforeEach(() => {
		vi.useFakeTimers();
		document.body.className = '';
		setFullscreen(true);
		cleanup = setupCursorAutoHide({ idleMs: 1000 });
	});

	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	it('hides the pointer once it has sat still in fullscreen', () => {
		expect(hidden()).toBe(false);
		vi.advanceTimersByTime(1000);
		expect(hidden()).toBe(true);
	});

	it('brings it back the moment the mouse moves', () => {
		vi.advanceTimersByTime(1000);
		expect(hidden()).toBe(true);

		window.dispatchEvent(new MouseEvent('mousemove'));
		expect(hidden()).toBe(false);
	});

	it('hides it again after the mouse stops', () => {
		window.dispatchEvent(new MouseEvent('mousemove'));
		vi.advanceTimersByTime(999);
		expect(hidden()).toBe(false);
		vi.advanceTimersByTime(1);
		expect(hidden()).toBe(true);
	});

	it('leaves the pointer alone outside fullscreen', () => {
		cleanup();
		document.body.className = '';
		setFullscreen(false);
		cleanup = setupCursorAutoHide({ idleMs: 1000 });

		vi.advanceTimersByTime(5000);
		expect(hidden()).toBe(false);
	});

	it('starts hiding when the deck goes fullscreen, and stops when it leaves', () => {
		cleanup();
		document.body.className = '';
		setFullscreen(false);
		cleanup = setupCursorAutoHide({ idleMs: 1000 });

		setFullscreen(true);
		document.dispatchEvent(new Event('fullscreenchange'));
		vi.advanceTimersByTime(1000);
		expect(hidden()).toBe(true);

		setFullscreen(false);
		document.dispatchEvent(new Event('fullscreenchange'));
		expect(hidden()).toBe(false);
		vi.advanceTimersByTime(5000);
		expect(hidden()).toBe(false);
	});

	it('shows the pointer again on cleanup', () => {
		vi.advanceTimersByTime(1000);
		expect(hidden()).toBe(true);

		cleanup();
		cleanup = () => {};
		expect(hidden()).toBe(false);
	});

	it('does not hide it again after cleanup', () => {
		cleanup();
		cleanup = () => {};
		vi.advanceTimersByTime(5000);
		expect(hidden()).toBe(false);
	});
});
