/**
 * Syntax highlighting utilities using Shiki.
 * Provides lazy initialization and caching of the highlighter instance.
 */

import type { BundledLanguage, BundledTheme, Highlighter } from 'shiki';

// ============================================================================
// Types
// ============================================================================

/**
 * Options for the highlight function.
 */
export interface HighlightOptions {
	/** The language to highlight (e.g., 'javascript', 'python') */
	language?: string;
	/** The theme to use for highlighting */
	theme?: BundledTheme;
}

/**
 * Configuration for initializing the highlighter.
 */
export interface HighlighterConfig {
	/** Themes to preload */
	themes?: BundledTheme[];
	/** Languages to preload */
	languages?: BundledLanguage[];
}

// ============================================================================
// Constants
// ============================================================================

/**
 * Default themes to load.
 */
export const DEFAULT_THEMES: BundledTheme[] = [
	'github-dark',
	'github-light',
	'one-dark-pro',
	'dracula',
	'nord'
];

/**
 * Common languages to support for presentations.
 * These are preloaded for fast highlighting.
 */
export const COMMON_LANGUAGES: BundledLanguage[] = [
	'javascript',
	'typescript',
	'python',
	'sql',
	'go',
	'rust',
	'bash',
	'json',
	'html',
	'css',
	'markdown',
	'yaml',
	'toml',
	'shell',
	'tsx',
	'jsx',
	'log'
];

/**
 * Default theme for highlighting.
 */
export const DEFAULT_THEME: BundledTheme = 'github-dark';

/**
 * Language aliases for common language names.
 */
export const LANGUAGE_ALIASES: Record<string, BundledLanguage> = {
	js: 'javascript',
	ts: 'typescript',
	py: 'python',
	sh: 'bash',
	zsh: 'bash',
	yml: 'yaml',
	md: 'markdown',
	golang: 'go',
	rs: 'rust',
	psql: 'sql',
	pgsql: 'sql',
	mysql: 'sql',
	sqlite: 'sql'
};

// ============================================================================
// Highlighter Instance
// ============================================================================

/** Cached highlighter instance */
let highlighterInstance: Highlighter | null = null;

/** Promise for pending highlighter initialization */
let highlighterPromise: Promise<Highlighter> | null = null;

/** Set of loaded languages */
const loadedLanguages = new Set<string>();

/** Set of loaded themes */
const loadedThemes = new Set<string>();

// ============================================================================
// Public Functions
// ============================================================================

/**
 * Initialize the Shiki highlighter with lazy theme loading.
 * This function is idempotent - calling it multiple times returns the same instance.
 *
 * @param config Optional configuration for themes and languages to preload
 * @returns Promise resolving to the highlighter instance
 */
export async function initHighlighter(config?: HighlighterConfig): Promise<Highlighter> {
	// Return existing instance if available
	if (highlighterInstance) {
		return highlighterInstance;
	}

	// Return pending promise if initialization is in progress
	if (highlighterPromise) {
		return highlighterPromise;
	}

	// Start initialization
	highlighterPromise = createHighlighter(config);

	try {
		highlighterInstance = await highlighterPromise;
		return highlighterInstance;
	} finally {
		highlighterPromise = null;
	}
}

/**
 * Highlight code with syntax highlighting.
 *
 * @param code The code to highlight
 * @param options Highlighting options (language, theme)
 * @returns Promise resolving to highlighted HTML string
 */
export async function highlight(code: string, options?: HighlightOptions): Promise<string> {
	const highlighter = await initHighlighter();

	const language = resolveLanguage(options?.language);
	const theme = options?.theme ?? DEFAULT_THEME;

	// Ensure the language is loaded
	await ensureLanguageLoaded(highlighter, language);

	// Ensure the theme is loaded
	await ensureThemeLoaded(highlighter, theme);

	// Generate highlighted HTML
	return highlighter.codeToHtml(code, {
		lang: language,
		theme: theme
	});
}

/**
 * Get the current highlighter instance if it has been initialized.
 * Returns null if not yet initialized.
 */
export function getHighlighter(): Highlighter | null {
	return highlighterInstance;
}

/**
 * Check if the highlighter has been initialized.
 */
export function isHighlighterReady(): boolean {
	return highlighterInstance !== null;
}

/**
 * Dispose of the highlighter instance and release resources.
 */
export function disposeHighlighter(): void {
	if (highlighterInstance) {
		highlighterInstance.dispose();
		highlighterInstance = null;
	}
	highlighterPromise = null;
	loadedLanguages.clear();
	loadedThemes.clear();
}

/**
 * Fallback language when no language is specified.
 * Using 'log' as it provides minimal highlighting suitable for plain text.
 */
export const FALLBACK_LANGUAGE: BundledLanguage = 'log';

/**
 * Resolve a language name to its canonical Shiki language identifier.
 * Handles aliases like 'js' -> 'javascript'.
 *
 * @param language The language name to resolve
 * @returns The canonical language identifier
 */
export function resolveLanguage(language?: string): BundledLanguage {
	if (!language || language.trim() === '') {
		return FALLBACK_LANGUAGE;
	}

	const normalized = language.toLowerCase().trim();

	// Check aliases first
	if (normalized in LANGUAGE_ALIASES) {
		return LANGUAGE_ALIASES[normalized] as BundledLanguage;
	}

	// Return as-is (Shiki will handle unknown languages gracefully)
	return normalized as BundledLanguage;
}

/**
 * Check if a language is in the list of common preloaded languages.
 *
 * @param language The language to check
 * @returns True if the language is commonly supported
 */
export function isCommonLanguage(language: string): boolean {
	const resolved = resolveLanguage(language);
	return COMMON_LANGUAGES.includes(resolved as BundledLanguage);
}

// ============================================================================
// Internal Functions
// ============================================================================

/**
 * Create a new Shiki highlighter instance.
 */
async function createHighlighter(config?: HighlighterConfig): Promise<Highlighter> {
	// Dynamic import for code splitting
	const { createHighlighter: shikiCreateHighlighter } = await import('shiki');

	const themes = config?.themes ?? DEFAULT_THEMES;
	const languages = config?.languages ?? COMMON_LANGUAGES;

	const highlighter = await shikiCreateHighlighter({
		themes,
		langs: languages
	});

	// Track loaded themes and languages
	for (const theme of themes) {
		loadedThemes.add(theme);
	}
	for (const lang of languages) {
		loadedLanguages.add(lang);
	}

	return highlighter;
}

/**
 * Ensure a language is loaded into the highlighter.
 * Loads the language lazily if not already loaded.
 */
async function ensureLanguageLoaded(
	highlighter: Highlighter,
	language: BundledLanguage
): Promise<void> {
	if (loadedLanguages.has(language)) {
		return;
	}

	try {
		await highlighter.loadLanguage(language);
		loadedLanguages.add(language);
	} catch {
		// If language fails to load, it will fall back to plain text
		// We don't need to throw here - Shiki handles unknown languages gracefully
		loadedLanguages.add(language); // Mark as "loaded" to avoid repeated attempts
	}
}

/**
 * Ensure a theme is loaded into the highlighter.
 * Loads the theme lazily if not already loaded.
 */
async function ensureThemeLoaded(highlighter: Highlighter, theme: BundledTheme): Promise<void> {
	if (loadedThemes.has(theme)) {
		return;
	}

	try {
		await highlighter.loadTheme(theme);
		loadedThemes.add(theme);
	} catch {
		// If theme fails to load, use default theme
		loadedThemes.add(theme); // Mark as "loaded" to avoid repeated attempts
	}
}
