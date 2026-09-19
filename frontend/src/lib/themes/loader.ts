/**
 * Loads and lists Tap themes.
 *
 * A theme is a folder under `lib/themes/<slug>/` holding `theme.css` (loaded
 * on demand) and `theme.json` (name, polarity, pitch, mermaid settings).
 * `base` has neither: its stylesheet is bundled unconditionally through
 * `app.css`, so it never needs a network round trip or an on-demand import.
 *
 * The list of every known theme (slug, name, polarity, pitch), including
 * ones not yet ported, comes from `internal/themes/themes.json`: the same
 * file Go reads for config validation and the CLI theme pickers.
 */

import themesList from '../../../../internal/themes/themes.json';
import type { MermaidThemeVariables } from '../utils/mermaid';

// ============================================================================
// Types
// ============================================================================

export interface ThemeSummary {
	slug: string;
	name: string;
	polarity: 'light' | 'dark';
	pitch: string;
}

export interface ThemeMermaidConfig {
	themeVariables?: MermaidThemeVariables;
	quietStyle?: string;
	curve?: 'basis' | 'linear' | 'natural' | 'step' | 'stepAfter' | 'stepBefore';
}

/** The shape of a theme's `theme.json` file. */
interface ThemeJSON {
	name: string;
	polarity: 'light' | 'dark';
	pitch: string;
	mermaid?: ThemeMermaidConfig;
}

/** `theme.json` plus the slug it was loaded for. */
export interface ThemeDefinition extends ThemeJSON {
	slug: string;
}

// ============================================================================
// Theme discovery
// ============================================================================

/** A lazy CSS module importer, as import.meta.glob for every theme's theme.css returns. */
type StylesheetImporter = () => Promise<unknown>;

/** An eagerly-loaded theme.json module, as import.meta.glob (eager) for every theme's theme.json returns. */
interface DefinitionModule {
	default: ThemeJSON;
}

/**
 * The data a theme loader instance reads from: the full theme list, and the
 * two `import.meta.glob` maps keyed by path (e.g. `./terminal/theme.css`).
 * Factored out from the module-level glob calls below so tests can inject
 * a fake theme list and fake, empty maps instead of depending on which
 * theme folders happen to exist on disk when the test runs.
 */
export interface ThemeLoaderSources {
	summaries: ThemeSummary[];
	stylesheets: Record<string, StylesheetImporter>;
	definitions: Record<string, DefinitionModule>;
}

/** A theme loader instance: the same functions this module exports, bound to one set of sources. */
export interface ThemeLoader {
	listThemes(): ThemeSummary[];
	isThemePorted(slug: string): boolean;
	loadTheme(slug: string): Promise<ThemeDefinition>;
	resetThemeLoaderState(): void;
}

const SLUG_FROM_PATH = /^\.\/([^/]+)\/theme\.json$/;

/**
 * Wait for web fonts to finish loading. A theme's CSS imports its fonts as a
 * side effect, and switching `[data-theme]` before those fonts are ready
 * flashes fallback-font text. `document.fonts` is unavailable in some test
 * environments, so this is a no-op there.
 */
async function waitForFonts(): Promise<void> {
	if (typeof document !== 'undefined' && document.fonts?.ready) {
		await document.fonts.ready;
	}
}

/**
 * Builds a theme loader against a given set of sources. The module-level
 * `listThemes`/`isThemePorted`/`loadTheme`/`resetThemeLoaderState` below are
 * this, called with the real `themes.json` list and the real
 * `import.meta.glob` maps; tests call it directly with fake sources so the
 * "unported theme" case doesn't depend on which theme folders exist on disk.
 */
export function createThemeLoader({ summaries, stylesheets, definitions }: ThemeLoaderSources): ThemeLoader {
	/** Slugs that have a folder (a theme.json was found in `definitions`). */
	function portedSlugs(): Set<string> {
		const slugs = new Set<string>();
		for (const path of Object.keys(definitions)) {
			const match = SLUG_FROM_PATH.exec(path);
			const slug = match?.[1];
			if (slug) {
				slugs.add(slug);
			}
		}
		return slugs;
	}

	/** The base theme's summary from the theme list, used as the loadTheme fallback. */
	function baseSummary(): ThemeSummary {
		const base = summaries.find((theme) => theme.slug === 'base');
		return base ?? { slug: 'base', name: 'Base', polarity: 'light', pitch: '' };
	}

	/** Tracks the CSS module most recently loaded, so re-selecting the same theme is a no-op. */
	let loadedCssSlug: string | null = null;

	function listThemes(): ThemeSummary[] {
		return summaries;
	}

	function isThemePorted(slug: string): boolean {
		return portedSlugs().has(slug);
	}

	async function loadTheme(slug: string): Promise<ThemeDefinition> {
		if (!portedSlugs().has(slug)) {
			return baseSummary();
		}

		const jsonPath = `./${slug}/theme.json`;
		const json = definitions[jsonPath]?.default;
		if (!json) {
			return baseSummary();
		}

		if (loadedCssSlug !== slug) {
			const cssPath = `./${slug}/theme.css`;
			const importer = stylesheets[cssPath];
			if (importer) {
				await importer();
			}
			await waitForFonts();
			loadedCssSlug = slug;
		}

		return { slug, ...json };
	}

	function resetThemeLoaderState(): void {
		loadedCssSlug = null;
	}

	return { listThemes, isThemePorted, loadTheme, resetThemeLoaderState };
}

const defaultLoader = createThemeLoader({
	summaries: themesList as ThemeSummary[],
	// Lazy CSS importers, keyed by path, e.g. `./terminal/theme.css`. One entry per theme folder.
	stylesheets: import.meta.glob('./*/theme.css'),
	// Eagerly-loaded theme.json modules, keyed by path, e.g. `./terminal/theme.json`.
	definitions: import.meta.glob('./*/theme.json', { eager: true }) as Record<string, DefinitionModule>
});

// ============================================================================
// Public API
// ============================================================================

/**
 * Every known theme (slug, name, polarity, pitch), in `themes.json` order
 * (`base` first). Includes themes with no folder yet: the `t` key cycles
 * over this full list, and an unported theme simply falls back to `base`
 * when selected.
 */
export const listThemes = defaultLoader.listThemes;

/** Whether a theme folder (theme.css and theme.json) exists for slug. */
export const isThemePorted = defaultLoader.isThemePorted;

/**
 * Resolve a theme by slug: loads its CSS (including its fonts) and returns
 * its definition. An unknown slug, or one with no ported folder yet,
 * resolves to `base`, whose stylesheet is already loaded unconditionally.
 *
 * Safe to call repeatedly (including with the same slug back to back): CSS
 * for a theme already loaded is not re-imported.
 */
export const loadTheme = defaultLoader.loadTheme;

/** Resets loader state between tests. */
export const resetThemeLoaderState = defaultLoader.resetThemeLoaderState;
