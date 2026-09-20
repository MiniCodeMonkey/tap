import { afterEach, describe, expect, it } from 'vitest';
import {
	availableLayouts,
	isPresenterLayout,
	nextLayout,
	readStoredLayout,
	resolveLayout,
	resolveNotesSizeMode,
	writeStoredLayout
} from './presenterLayout';

afterEach(() => {
	window.localStorage.clear();
});

describe('isPresenterLayout', () => {
	it('accepts every known id and rejects anything else', () => {
		expect(isPresenterLayout('notes-only')).toBe(true);
		expect(isPresenterLayout('sidebar')).toBe(false);
		expect(isPresenterLayout(null)).toBe(false);
	});
});

describe('availableLayouts', () => {
	it('offers all five layouts on a wide screen', () => {
		expect(availableLayouts(false).map((entry) => entry.id)).toEqual([
			'standard',
			'notes-first',
			'duo',
			'slide-only',
			'notes-only'
		]);
	});

	it('drops the two-slide layouts on a narrow screen', () => {
		expect(availableLayouts(true).map((entry) => entry.id)).toEqual([
			'standard',
			'notes-first',
			'notes-only'
		]);
	});
});

describe('resolveLayout', () => {
	it('falls back to standard when nothing is set', () => {
		expect(resolveLayout({ isNarrow: false })).toBe('standard');
	});

	it('prefers the URL over storage, and storage over the deck', () => {
		expect(
			resolveLayout({
				fromUrl: 'duo',
				fromStorage: 'notes-only',
				fromConfig: 'slide-only',
				isNarrow: false
			})
		).toBe('duo');
		expect(
			resolveLayout({ fromStorage: 'notes-only', fromConfig: 'slide-only', isNarrow: false })
		).toBe('notes-only');
		expect(resolveLayout({ fromConfig: 'slide-only', isNarrow: false })).toBe('slide-only');
	});

	it('skips an unrecognised value and takes the next source', () => {
		expect(resolveLayout({ fromUrl: 'sidebar', fromStorage: 'duo', isNarrow: false })).toBe('duo');
		expect(resolveLayout({ fromUrl: 'sidebar', isNarrow: false })).toBe('standard');
	});

	it('falls back to notes-first when the winner does not fit a narrow screen', () => {
		expect(resolveLayout({ fromStorage: 'duo', isNarrow: true })).toBe('notes-first');
		expect(resolveLayout({ fromStorage: 'slide-only', isNarrow: true })).toBe('notes-first');
	});
});

describe('nextLayout', () => {
	it('cycles through the available layouts and wraps', () => {
		expect(nextLayout('standard', false)).toBe('notes-first');
		expect(nextLayout('notes-only', false)).toBe('standard');
		expect(nextLayout('notes-first', true)).toBe('notes-only');
		expect(nextLayout('notes-only', true)).toBe('standard');
	});

	it('starts from the beginning when the current layout is unavailable', () => {
		expect(nextLayout('duo', true)).toBe('standard');
	});
});

describe('resolveNotesSizeMode', () => {
	it('defaults to manual and prefers the URL', () => {
		expect(resolveNotesSizeMode({})).toBe('manual');
		expect(resolveNotesSizeMode({ fromStorage: 'fit' })).toBe('fit');
		expect(resolveNotesSizeMode({ fromUrl: 'manual', fromStorage: 'fit' })).toBe('manual');
		expect(resolveNotesSizeMode({ fromUrl: 'huge' })).toBe('manual');
	});
});

describe('stored layout', () => {
	it('round-trips through localStorage and ignores junk', () => {
		expect(readStoredLayout()).toBeNull();
		writeStoredLayout('duo');
		expect(readStoredLayout()).toBe('duo');
		window.localStorage.setItem('tap-presenter-layout', 'sidebar');
		expect(readStoredLayout()).toBeNull();
	});
});
