/**
 * Syntax highlighting utilities using Shiki.
 * Provides lazy initialization and caching of the highlighter instance.
 * Supports advanced features: line highlighting, line numbers, titles, and diffs.
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
	/** Lines to highlight (e.g., [1, 3, 4, 5] or "1,3-5") */
	highlightLines?: number[] | string;
	/** Whether to show line numbers */
	showLineNumbers?: boolean;
	/** Code block title (e.g., filename) */
	title?: string;
	/** Whether this is a diff (enables +/- line detection) */
	isDiff?: boolean;
	/** Maximum height in viewport height units (for auto-sizing) */
	maxHeight?: number;
}

/**
 * Parsed highlight range from syntax like "1,3-5,7"
 */
export interface HighlightRange {
	lines: Set<number>;
}

/**
 * Result of highlighting with metadata.
 */
export interface HighlightResult {
	/** The highlighted HTML */
	html: string;
	/** Line count in the code */
	lineCount: number;
	/** Whether auto-sizing was applied */
	autoSized: boolean;
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
	const result = await highlightWithMetadata(code, options);
	return result.html;
}

/**
 * Highlight code with syntax highlighting and return additional metadata.
 *
 * @param code The code to highlight
 * @param options Highlighting options
 * @returns Promise resolving to HighlightResult with HTML and metadata
 */
export async function highlightWithMetadata(
	code: string,
	options?: HighlightOptions
): Promise<HighlightResult> {
	const highlighter = await initHighlighter();

	const language = resolveLanguage(options?.language);
	const theme = options?.theme ?? DEFAULT_THEME;

	// Ensure the language is loaded
	await ensureLanguageLoaded(highlighter, language);

	// Ensure the theme is loaded
	await ensureThemeLoaded(highlighter, theme);

	// Parse highlight lines if provided
	const highlightedLines = parseHighlightLines(options?.highlightLines);

	// Count lines for auto-sizing
	const lines = code.split('\n');
	const lineCount = lines.length;

	// Determine if auto-sizing should be applied
	const autoSized = shouldAutoSize(lineCount, options?.maxHeight);

	// Generate base highlighted HTML
	let html = highlighter.codeToHtml(code, {
		lang: language,
		theme: theme
	});

	// Apply line-based transformations
	html = applyLineTransformations(html, {
		highlightedLines,
		showLineNumbers: options?.showLineNumbers ?? false,
		isDiff: options?.isDiff ?? false,
		lineCount
	});

	// Wrap with title if provided
	if (options?.title) {
		html = wrapWithTitle(html, options.title);
	}

	// Apply auto-sizing container if needed
	if (autoSized && options?.maxHeight) {
		html = wrapWithAutoSize(html, options.maxHeight);
	}

	return {
		html,
		lineCount,
		autoSized
	};
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

// ============================================================================
// Line Highlighting Functions
// ============================================================================

/**
 * Parse highlight lines specification.
 * Supports formats: "1,3-5,7" or [1, 3, 4, 5, 7]
 *
 * @param spec The highlight specification
 * @returns Set of line numbers to highlight (1-indexed)
 */
export function parseHighlightLines(spec?: number[] | string): Set<number> {
	const lines = new Set<number>();

	if (!spec) {
		return lines;
	}

	// Handle array input
	if (Array.isArray(spec)) {
		for (const line of spec) {
			if (typeof line === 'number' && line > 0) {
				lines.add(line);
			}
		}
		return lines;
	}

	// Handle string input like "1,3-5,7"
	const parts = spec.split(',').map((p) => p.trim());

	for (const part of parts) {
		if (part.includes('-')) {
			// Range: "3-5"
			const [startStr, endStr] = part.split('-').map((s) => s.trim());
			const start = parseInt(startStr || '', 10);
			const end = parseInt(endStr || '', 10);

			if (!isNaN(start) && !isNaN(end) && start > 0 && end >= start) {
				for (let i = start; i <= end; i++) {
					lines.add(i);
				}
			}
		} else {
			// Single line: "3"
			const line = parseInt(part, 10);
			if (!isNaN(line) && line > 0) {
				lines.add(line);
			}
		}
	}

	return lines;
}

/**
 * Options for line transformations.
 */
interface LineTransformOptions {
	highlightedLines: Set<number>;
	showLineNumbers: boolean;
	isDiff: boolean;
	lineCount: number;
}

/**
 * Apply line-based transformations to highlighted HTML.
 * Adds line highlighting, line numbers, and diff markers.
 */
function applyLineTransformations(html: string, options: LineTransformOptions): string {
	const { highlightedLines, showLineNumbers, isDiff, lineCount } = options;

	// If no transformations needed, return as-is
	if (highlightedLines.size === 0 && !showLineNumbers && !isDiff) {
		return html;
	}

	// Parse the HTML structure
	// Shiki generates: <pre class="..."><code>...lines...</code></pre>
	// We need to wrap each line in a span for styling

	// Extract content between <code> tags
	const codeMatch = html.match(/<code[^>]*>([\s\S]*?)<\/code>/);
	if (!codeMatch || codeMatch[1] === undefined) {
		return html;
	}

	const preMatch = html.match(/(<pre[^>]*>)/);
	const preOpen = preMatch?.[1] ?? '<pre>';

	const codeOpenMatch = html.match(/<code[^>]*>/);
	const codeOpen = codeOpenMatch?.[0] ?? '<code>';

	const codeContent = codeMatch[1];

	// Split by line (handling various line structures)
	// Shiki typically wraps each line in a span with class="line"
	const lineRegex = /<span class="line">([\s\S]*?)<\/span>/g;
	const lineMatches = [...codeContent.matchAll(lineRegex)];

	let processedLines: string[];

	if (lineMatches.length > 0) {
		// Modern Shiki output with line spans
		processedLines = lineMatches.map((match, index) => {
			const lineNum = index + 1;
			const lineContent = match[1] ?? '';
			return processLine(lineNum, lineContent, options);
		});
	} else {
		// Fallback: split by newlines
		const rawLines = codeContent.split('\n');
		processedLines = rawLines.map((lineContent, index) => {
			const lineNum = index + 1;
			return processLine(lineNum, lineContent, options);
		});
	}

	// Calculate line number width for padding
	const lineNumWidth = String(lineCount).length;

	// Add line number width as CSS variable
	const preWithVar = preOpen.replace(
		/<pre/,
		`<pre style="--line-num-width: ${lineNumWidth}ch"`
	);

	// Reconstruct the HTML
	return `${preWithVar}${codeOpen}${processedLines.join('\n')}</code></pre>`;
}

/**
 * Process a single line, adding classes and content as needed.
 */
function processLine(lineNum: number, content: string, options: LineTransformOptions): string {
	const classes: string[] = ['line'];
	const { highlightedLines, showLineNumbers, isDiff } = options;

	// Check if this line should be highlighted
	if (highlightedLines.has(lineNum)) {
		classes.push('highlighted');
	}

	// Check for diff markers
	if (isDiff) {
		const trimmedContent = stripHtmlTags(content).trimStart();
		if (trimmedContent.startsWith('+')) {
			classes.push('diff-add');
		} else if (trimmedContent.startsWith('-')) {
			classes.push('diff-remove');
		}
	}

	// Build the line HTML
	let lineHtml = '';

	if (showLineNumbers) {
		lineHtml += `<span class="line-number" data-line="${lineNum}">${lineNum}</span>`;
	}

	lineHtml += `<span class="line-content">${content}</span>`;

	return `<span class="${classes.join(' ')}">${lineHtml}</span>`;
}

/**
 * Strip HTML tags from content (for diff detection).
 */
function stripHtmlTags(html: string): string {
	return html.replace(/<[^>]*>/g, '');
}

/**
 * Wrap highlighted code with a title header.
 */
function wrapWithTitle(html: string, title: string): string {
	const escapedTitle = escapeHtml(title);
	return `<div class="code-block-wrapper">
<div class="code-block-title">${escapedTitle}</div>
${html}
</div>`;
}

/**
 * Escape HTML special characters.
 */
function escapeHtml(text: string): string {
	return text
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#039;');
}

// ============================================================================
// Auto-sizing Functions
// ============================================================================

/**
 * Default maximum height in vh for auto-sizing.
 */
export const DEFAULT_MAX_HEIGHT = 70;

/**
 * Line count threshold for auto-sizing consideration.
 */
export const AUTO_SIZE_LINE_THRESHOLD = 20;

/**
 * Determine if auto-sizing should be applied based on line count.
 */
function shouldAutoSize(lineCount: number, maxHeight?: number): boolean {
	// Only auto-size if maxHeight is specified and content is large
	if (!maxHeight) {
		return false;
	}
	return lineCount > AUTO_SIZE_LINE_THRESHOLD;
}

/**
 * Wrap code block with auto-sizing container.
 */
function wrapWithAutoSize(html: string, maxHeight: number): string {
	return `<div class="code-block-auto-size" style="max-height: ${maxHeight}vh; overflow-y: auto;">
${html}
</div>`;
}

// ============================================================================
// CSS Classes Reference
// ============================================================================
/**
 * CSS classes used by the highlighting system:
 *
 * .line - Base class for each line
 * .line.highlighted - Line that should be highlighted
 * .line.diff-add - Diff line with addition (+)
 * .line.diff-remove - Diff line with removal (-)
 * .line-number - Line number element
 * .line-content - Actual code content
 * .code-block-wrapper - Container for code block with title
 * .code-block-title - Title bar above code block
 * .code-block-auto-size - Container for auto-sized code blocks
 *
 * Recommended CSS:
 *
 * .line.highlighted {
 *   background-color: rgba(255, 255, 0, 0.1);
 *   display: block;
 * }
 *
 * .line.diff-add {
 *   background-color: rgba(0, 255, 0, 0.1);
 * }
 *
 * .line.diff-remove {
 *   background-color: rgba(255, 0, 0, 0.1);
 * }
 *
 * .line-number {
 *   display: inline-block;
 *   width: var(--line-num-width, 2ch);
 *   text-align: right;
 *   padding-right: 1em;
 *   color: #666;
 *   user-select: none;
 * }
 *
 * .code-block-title {
 *   background: #1e1e1e;
 *   color: #999;
 *   padding: 0.5em 1em;
 *   font-size: 0.85em;
 *   border-bottom: 1px solid #333;
 *   font-family: inherit;
 * }
 */
