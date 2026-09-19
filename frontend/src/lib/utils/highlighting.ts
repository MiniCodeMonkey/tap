/**
 * Syntax highlighting utilities using Shiki.
 * Provides lazy initialization and caching of the highlighter instance.
 * Supports advanced features: line highlighting, line numbers, titles, and diffs.
 *
 * Uses Shiki's CSS variables theme instead of a bundled theme: token colors
 * come from `--shiki-*` custom properties defined per Tap theme in CSS, so
 * highlighted code follows whichever theme is active without re-highlighting.
 */

import type { BundledLanguage, Highlighter } from 'shiki';
import { createCssVariablesTheme } from '@shikijs/core';

// ============================================================================
// Types
// ============================================================================

/**
 * Options for the highlight function.
 */
export interface HighlightOptions {
	/** The language to highlight (e.g., 'javascript', 'python') */
	language?: string;
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
	/** Languages to preload */
	languages?: BundledLanguage[];
}

// ============================================================================
// Constants
// ============================================================================

/**
 * The Shiki theme used for all highlighting.
 * Token colors are CSS custom properties (`--shiki-foreground`,
 * `--shiki-token-keyword`, etc.) so a theme's stylesheet controls the
 * palette instead of picking a bundled Shiki theme per Tap theme.
 */
export const CSS_VARIABLES_THEME = createCssVariablesTheme({
	name: 'tap-css-variables',
	variablePrefix: '--shiki-',
	fontStyle: true
});

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

// ============================================================================
// Public Functions
// ============================================================================

/**
 * Initialize the Shiki highlighter with lazy language loading.
 * This function is idempotent - calling it multiple times returns the same instance.
 *
 * @param config Optional configuration for languages to preload
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
 * @param options Highlighting options (language, etc.)
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

	// Ensure the language is loaded
	await ensureLanguageLoaded(highlighter, language);

	// Count lines for auto-sizing, and to clamp a highlight-lines range below.
	const lines = code.split('\n');
	const lineCount = lines.length;

	// Parse highlight lines if provided, clamped to this block's line count.
	const highlightedLines = parseHighlightLines(options?.highlightLines, lineCount);

	// Determine if auto-sizing should be applied
	const autoSized = shouldAutoSize(lineCount, options?.maxHeight);

	// Generate base highlighted HTML using the CSS variables theme
	let html = highlighter.codeToHtml(code, {
		lang: language,
		theme: CSS_VARIABLES_THEME
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

	const languages = config?.languages ?? COMMON_LANGUAGES;

	const highlighter = await shikiCreateHighlighter({
		themes: [CSS_VARIABLES_THEME],
		langs: languages
	});

	// Track loaded languages
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

// ============================================================================
// Line Highlighting Functions
// ============================================================================

/**
 * A hard cap on the highest line number parseHighlightLines will expand a
 * range up to, used when the caller doesn't know the code block's actual
 * line count. Without any cap, a spec like "{1-999999999}" iterates a
 * billion times and hangs the tab.
 */
const HIGHLIGHT_LINE_HARD_CAP = 10000;

/**
 * Parse highlight lines specification.
 * Supports formats: "1,3-5,7" or [1, 3, 4, 5, 7]
 *
 * @param spec The highlight specification
 * @param lineCount The code block's actual line count, used to clamp a
 *   range's upper bound. Falls back to a hard cap when omitted, since a
 *   range is otherwise expanded by iterating every number in it.
 * @returns Set of line numbers to highlight (1-indexed)
 */
export function parseHighlightLines(spec?: number[] | string, lineCount?: number): Set<number> {
	const lines = new Set<number>();

	if (!spec) {
		return lines;
	}

	const maxLine = lineCount && lineCount > 0 ? lineCount : HIGHLIGHT_LINE_HARD_CAP;

	// Handle array input
	if (Array.isArray(spec)) {
		for (const line of spec) {
			if (typeof line === 'number' && line > 0 && line <= maxLine) {
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
			const end = Math.min(parseInt(endStr || '', 10), maxLine);

			if (!isNaN(start) && !isNaN(end) && start > 0 && start <= maxLine && end >= start) {
				for (let i = start; i <= end; i++) {
					lines.add(i);
				}
			}
		} else {
			// Single line: "3"
			const line = parseInt(part, 10);
			if (!isNaN(line) && line > 0 && line <= maxLine) {
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
 *
 * Works on a parsed DOM fragment rather than the HTML string: Shiki nests
 * one span per token inside each line's `<span class="line">`, so a regex
 * that looks for the line span's closing tag stops at the first token's
 * closing tag instead, truncating every multi-token line down to its first
 * token. Parsing into a `<template>` and moving each line's existing child
 * nodes (its token spans, however many there are) into a `.line-content`
 * wrapper keeps every token intact.
 */
function applyLineTransformations(html: string, options: LineTransformOptions): string {
	const { highlightedLines, showLineNumbers, isDiff, lineCount } = options;

	// If no transformations needed, return as-is
	if (highlightedLines.size === 0 && !showLineNumbers && !isDiff) {
		return html;
	}

	// Shiki generates: <pre class="..."><code>...lines...</code></pre>
	// A <template>'s innerHTML parses into (and serializes back from) its
	// `.content` DocumentFragment without inserting anything into the live
	// document, so this is safe to run on untrusted-looking markup.
	const template = document.createElement('template');
	template.innerHTML = html;
	const pre = template.content.querySelector('pre');
	const code = template.content.querySelector('code');
	if (!pre || !code) {
		return html;
	}

	// Shiki wraps each line in a top-level <span class="line"> directly
	// under <code>; anything with an additional class (say, from a future
	// Shiki version) still matches, since this checks for the class, not an
	// exact attribute string.
	const lineSpans = Array.from(code.querySelectorAll<HTMLElement>(':scope > span.line'));

	if (lineSpans.length > 0) {
		lineSpans.forEach((lineSpan, index) => transformLineElement(lineSpan, index + 1, options));
	} else {
		// Fallback for output with no per-line spans (e.g. a language Shiki
		// highlights as one block): split the plain text by newline and
		// build a line span per line from scratch.
		const rawLines = (code.textContent ?? '').split('\n');
		code.textContent = '';
		rawLines.forEach((lineText, index) => {
			const lineSpan = document.createElement('span');
			lineSpan.textContent = lineText;
			code.appendChild(lineSpan);
			if (index < rawLines.length - 1) {
				code.appendChild(document.createTextNode('\n'));
			}
			transformLineElement(lineSpan, index + 1, options);
		});
	}

	// Add line number width as a CSS variable, and mark the block as having
	// highlighted lines so CSS can dim the rest.
	pre.style.setProperty('--line-num-width', `${String(lineCount).length}ch`);
	if (highlightedLines.size > 0) {
		pre.classList.add('has-highlighted');
	}

	return template.innerHTML;
}

/**
 * Transform one line's `<span>` in place: set its classes (line, highlighted,
 * diff-add/diff-remove) and wrap its existing children, whatever tokens
 * Shiki nested inside it, in a `.line-content` span, prefixed by a
 * `.line-number` span when requested.
 */
function transformLineElement(lineSpan: HTMLElement, lineNum: number, options: LineTransformOptions): void {
	const { highlightedLines, showLineNumbers, isDiff } = options;

	const classes: string[] = ['line'];
	if (highlightedLines.has(lineNum)) {
		classes.push('highlighted');
	}
	if (isDiff) {
		const trimmedContent = (lineSpan.textContent ?? '').trimStart();
		if (trimmedContent.startsWith('+')) {
			classes.push('diff-add');
		} else if (trimmedContent.startsWith('-')) {
			classes.push('diff-remove');
		}
	}
	lineSpan.className = classes.join(' ');

	// Move every existing child (Shiki's per-token spans, or a single text
	// node) into a .line-content wrapper, preserving all of them intact.
	const contentSpan = document.createElement('span');
	contentSpan.className = 'line-content';
	while (lineSpan.firstChild) {
		contentSpan.appendChild(lineSpan.firstChild);
	}

	if (showLineNumbers) {
		const numberSpan = document.createElement('span');
		numberSpan.className = 'line-number';
		numberSpan.dataset.line = String(lineNum);
		numberSpan.textContent = String(lineNum);
		lineSpan.appendChild(numberSpan);
	}

	lineSpan.appendChild(contentSpan);
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
// DOM-based Highlighting
// ============================================================================

/**
 * Find and highlight all code blocks within a DOM element.
 * Replaces plain <pre><code class="language-*"> blocks with syntax-highlighted versions.
 * Skips mermaid and asciinema blocks which are handled separately.
 *
 * @param element The DOM element to search within
 * @returns Promise resolving when all code blocks are highlighted
 */
export async function highlightCodeBlocksInElement(element: HTMLElement): Promise<void> {
	// Find all code blocks that need highlighting (skip mermaid and
	// asciinema). Scoped to .slot descendants so DOM a deck-supplied
	// component owns outside of a slot is never mutated here. The `:is()`
	// wrapper around the descendant+child combinator is needed for jsdom's
	// selector engine to evaluate correctly when chained with :not(); real
	// browsers accept the unwrapped form too, but this form works in both.
	const codeBlocks = element.querySelectorAll<HTMLElement>(
		':is(.slot pre) > code[class*="language-"]:not(.language-mermaid):not(.language-asciinema)'
	);

	if (codeBlocks.length === 0) {
		return;
	}

	// Process each code block
	const promises = Array.from(codeBlocks).map(async (codeBlock) => {
		const pre = codeBlock.parentElement;
		if (!pre || pre.dataset.highlighted === 'true' || pre.dataset.highlighted === 'processing') {
			return; // Already highlighted or processing
		}

		// The host slide may have been unmounted by rapid navigation since
		// this pass started; skip this block's highlight work instead of
		// doing it for an element nothing will ever show.
		if (!pre.isConnected) {
			return;
		}

		// Extract language from class
		const classMatch = codeBlock.className.match(/language-(\w+)/);
		const language = classMatch?.[1] ?? 'text';

		// Get the code content
		const code = codeBlock.textContent ?? '';

		// A line-highlight spec from the fence's info string (e.g. "```php
		// {3-4}") arrives as a data attribute on this element: the Go parser
		// carries it there because goldmark's own renderer keeps only the
		// language from the info string and drops everything else.
		const highlightLines = codeBlock.dataset.highlightLines;

		// Mark as processing to prevent duplicate attempts
		pre.dataset.highlighted = 'processing';

		try {
			// Highlight the code with the CSS variables theme
			const highlightedHtml = await highlight(code, { language, highlightLines });

			// Create a temporary container to parse the HTML
			const temp = document.createElement('div');
			temp.innerHTML = highlightedHtml;

			// Get the new pre element from Shiki's output
			const newPre = temp.querySelector('pre');
			if (newPre) {
				// Mark as highlighted to prevent re-processing
				newPre.dataset.highlighted = 'true';
				// Preserve any existing classes on the original pre
				newPre.className = `${newPre.className} ${pre.className}`.trim();
				pre.replaceWith(newPre);
			} else {
				pre.dataset.highlighted = 'true';
			}
		} catch (err) {
			// On error, leave the original code block in place and show the error
			// message under it instead of losing the source.
			pre.dataset.highlighted = 'error';
			const errorMessage = err instanceof Error ? err.message : String(err);
			const errorEl = document.createElement('div');
			errorEl.className = 'code-block-error-message';
			errorEl.textContent = `Failed to highlight code block: ${errorMessage}`;
			pre.after(errorEl);
			console.error('Failed to highlight code block:', err);
		}
	});

	await Promise.all(promises);
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
 * pre.has-highlighted - A code block that has at least one highlighted line,
 *   so its other lines can be dimmed in CSS.
 *
 * Recommended CSS:
 *
 * .has-highlighted .line:not(.highlighted) {
 *   opacity: 0.5;
 * }
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
