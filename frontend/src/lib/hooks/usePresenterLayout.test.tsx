import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { cleanup, render } from '@testing-library/react';
import { usePresenterLayout, type PresenterLayoutState } from './usePresenterLayout';

let latest: PresenterLayoutState;

function Probe({ configured }: { configured?: string }) {
	latest = usePresenterLayout(configured);
	return <span>{latest.layout}</span>;
}

/** Installs a matchMedia stub whose matches value can be changed. */
function stubMatchMedia(matches: boolean) {
	const listeners = new Set<(event: MediaQueryListEvent) => void>();
	const list = {
		matches,
		media: '',
		addEventListener: (_: string, handler: (event: MediaQueryListEvent) => void) =>
			listeners.add(handler),
		removeEventListener: (_: string, handler: (event: MediaQueryListEvent) => void) =>
			listeners.delete(handler)
	};
	Object.defineProperty(window, 'matchMedia', {
		configurable: true,
		writable: true,
		value: vi.fn(() => list)
	});
	return {
		setMatches(next: boolean) {
			list.matches = next;
			for (const handler of listeners) {
				handler({ matches: next } as MediaQueryListEvent);
			}
		}
	};
}

beforeEach(() => {
	window.localStorage.clear();
	window.history.replaceState(null, '', '/presenter');
});

afterEach(() => {
	cleanup();
});

describe('usePresenterLayout', () => {
	it('starts on standard with nothing configured', () => {
		stubMatchMedia(false);
		render(<Probe />);
		expect(latest.layout).toBe('standard');
		expect(latest.notesSizeMode).toBe('manual');
	});

	it('takes the deck default when storage is empty', () => {
		stubMatchMedia(false);
		render(<Probe configured="notes-first" />);
		expect(latest.layout).toBe('notes-first');
	});

	it('lets a stored layout beat the deck default', () => {
		stubMatchMedia(false);
		window.localStorage.setItem('tap-presenter-layout', 'duo');
		render(<Probe configured="notes-first" />);
		expect(latest.layout).toBe('duo');
	});

	it('lets a url parameter beat storage, without writing it back', () => {
		stubMatchMedia(false);
		window.localStorage.setItem('tap-presenter-layout', 'duo');
		window.history.replaceState(null, '', '/presenter?layout=notes-only');
		render(<Probe />);
		expect(latest.layout).toBe('notes-only');
		expect(window.localStorage.getItem('tap-presenter-layout')).toBe('duo');
	});

	it('stores a chosen layout and lets it beat the url parameter', () => {
		stubMatchMedia(false);
		window.history.replaceState(null, '', '/presenter?layout=notes-only');
		render(<Probe />);
		act(() => latest.setLayout('duo'));
		expect(latest.layout).toBe('duo');
		expect(window.localStorage.getItem('tap-presenter-layout')).toBe('duo');
	});

	it('cycles through the available layouts', () => {
		stubMatchMedia(false);
		render(<Probe />);
		act(() => latest.cycleLayout());
		expect(latest.layout).toBe('notes-first');
	});

	it('re-resolves when the screen becomes narrow and back again', () => {
		const media = stubMatchMedia(false);
		window.localStorage.setItem('tap-presenter-layout', 'duo');
		render(<Probe />);
		expect(latest.layout).toBe('duo');
		act(() => media.setMatches(true));
		expect(latest.layout).toBe('notes-first');
		expect(window.localStorage.getItem('tap-presenter-layout')).toBe('duo');
		act(() => media.setMatches(false));
		expect(latest.layout).toBe('duo');
	});

	it('falls back to standard when matchMedia is missing', () => {
		Object.defineProperty(window, 'matchMedia', {
			configurable: true,
			writable: true,
			value: undefined
		});
		render(<Probe />);
		expect(latest.layout).toBe('standard');
		expect(latest.isNarrow).toBe(false);
	});

	it('stores the notes size mode', () => {
		stubMatchMedia(false);
		render(<Probe />);
		act(() => latest.setNotesSizeMode('fit'));
		expect(latest.notesSizeMode).toBe('fit');
		expect(window.localStorage.getItem('tap-presenter-notes-size-mode')).toBe('fit');
	});
});
