import { describe, expect, it, beforeEach } from 'vitest';
import { createThemeLoader, isThemePorted, listThemes, loadTheme, resetThemeLoaderState } from './loader';

beforeEach(() => {
	resetThemeLoaderState();
});

/** A fake theme list: one ported theme (with matching stylesheet/definition entries) and one not. */
const FAKE_SUMMARIES = [
	{ slug: 'base', name: 'Base', polarity: 'light' as const, pitch: 'The plain fallback.' },
	{ slug: 'fake-ported', name: 'Fake Ported', polarity: 'dark' as const, pitch: 'A theme with a folder.' },
	{ slug: 'fake-unported', name: 'Fake Unported', polarity: 'light' as const, pitch: 'A theme with no folder yet.' }
];

function createFakeLoader() {
	return createThemeLoader({
		summaries: FAKE_SUMMARIES,
		stylesheets: {
			'./fake-ported/theme.css': () => Promise.resolve()
		},
		definitions: {
			'./fake-ported/theme.json': {
				default: { name: 'Fake Ported', polarity: 'dark', pitch: 'A theme with a folder.' }
			}
		}
	});
}

describe('listThemes', () => {
	it('lists base first', () => {
		const themes = listThemes();
		expect(themes.length).toBeGreaterThan(0);
		expect(themes[0].slug).toBe('base');
	});

	it('includes every built-in theme, ported or not', () => {
		const themes = listThemes();
		const slugs = themes.map((t) => t.slug);
		expect(slugs).toContain('terminal');
		expect(slugs).toContain('swiss');
		expect(themes.length).toBe(21);
	});

	it('gives every theme a name, polarity, and pitch', () => {
		for (const theme of listThemes()) {
			expect(theme.name).not.toBe('');
			expect(['light', 'dark']).toContain(theme.polarity);
			expect(theme.pitch).not.toBe('');
		}
	});
});

describe('isThemePorted', () => {
	it('is true for a theme with a folder', () => {
		expect(isThemePorted('terminal')).toBe(true);
	});

	it('is false for an unknown slug', () => {
		expect(isThemePorted('not-a-real-theme')).toBe(false);
	});

	it('is true for a fake theme with matching stylesheet and definition entries', () => {
		const loader = createFakeLoader();
		expect(loader.isThemePorted('fake-ported')).toBe(true);
	});

	it('is false for a theme listed in themes.json with no folder yet', () => {
		const loader = createFakeLoader();
		expect(loader.isThemePorted('fake-unported')).toBe(false);
	});
});

describe('loadTheme', () => {
	it('resolves a ported theme to its own definition', async () => {
		const theme = await loadTheme('terminal');
		expect(theme.slug).toBe('terminal');
		expect(theme.name).toBe('Terminal');
		expect(theme.polarity).toBe('dark');
		expect(theme.mermaid?.curve).toBe('stepAfter');
	});

	it('falls back to base for an unknown slug', async () => {
		const theme = await loadTheme('not-a-real-theme');
		expect(theme.slug).toBe('base');
	});

	it('falls back to base for a slug listed in themes.json with no folder yet', async () => {
		const loader = createFakeLoader();
		const theme = await loader.loadTheme('fake-unported');
		expect(theme.slug).toBe('base');
	});

	it('resolves a fake ported theme to its own definition', async () => {
		const loader = createFakeLoader();
		const theme = await loader.loadTheme('fake-ported');
		expect(theme.slug).toBe('fake-ported');
		expect(theme.name).toBe('Fake Ported');
	});

	it('resolving base returns no mermaid overrides', async () => {
		const theme = await loadTheme('base');
		expect(theme.slug).toBe('base');
		expect(theme.mermaid).toBeUndefined();
	});

	it('is safe to call twice in a row for the same theme', async () => {
		const first = await loadTheme('terminal');
		const second = await loadTheme('terminal');
		expect(first).toEqual(second);
	});
});
