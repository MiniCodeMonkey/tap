import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { disposeHighlighter, highlightCodeBlocksInElement, parseHighlightLines } from './highlighting';

// Stub Shiki so line highlighting is exercised without the real WASM-backed
// highlighter. codeToHtml returns one <span class="line"> per input line, the
// same shape applyLineTransformations expects from the real library.
vi.mock('shiki', () => ({
	createHighlighter: vi.fn(async () => ({
		codeToHtml: vi.fn((code: string) => {
			const lines = code
				.split('\n')
				.map((line) => `<span class="line">${line}</span>`)
				.join('\n');
			return `<pre class="shiki"><code>${lines}</code></pre>`;
		}),
		loadLanguage: vi.fn(async () => {}),
		dispose: vi.fn()
	}))
}));

describe('highlightCodeBlocksInElement', () => {
	beforeEach(() => {
		disposeHighlighter();
	});

	afterEach(() => {
		disposeHighlighter();
		document.body.innerHTML = '';
	});

	it('reads a data-highlight-lines attribute from the code element and highlights those lines', async () => {
		const container = document.createElement('div');
		container.className = 'slot';
		container.innerHTML =
			'<pre><code class="language-php" data-highlight-lines="3-4">line1\nline2\nline3\nline4\nline5</code></pre>';
		// Attach to the document: highlightCodeBlocksInElement skips a
		// detached block's highlight work (see the isConnected guard test
		// below), so a container the browser would actually render into
		// needs to be in the document, the way useRichBlocks's host is.
		document.body.appendChild(container);

		await highlightCodeBlocksInElement(container);

		const pre = container.querySelector('pre');
		expect(pre?.classList.contains('has-highlighted')).toBe(true);

		const lines = container.querySelectorAll('.line');
		expect(lines).toHaveLength(5);
		expect(lines[0]?.classList.contains('highlighted')).toBe(false);
		expect(lines[2]?.classList.contains('highlighted')).toBe(true);
		expect(lines[3]?.classList.contains('highlighted')).toBe(true);
		expect(lines[4]?.classList.contains('highlighted')).toBe(false);
	});

	it('does not add has-highlighted when there is no data-highlight-lines attribute', async () => {
		const container = document.createElement('div');
		container.className = 'slot';
		container.innerHTML = '<pre><code class="language-js">const x = 1;</code></pre>';
		// Attach to the document, as in the sibling test above: otherwise the
		// isConnected guard skips this block entirely and the assertion below
		// passes without the highlighter having run at all.
		document.body.appendChild(container);

		await highlightCodeBlocksInElement(container);

		const pre = container.querySelector('pre');
		// Prove the highlighter actually ran before asserting has-highlighted
		// is absent, so a regression that skips the block silently can't pass
		// this test for the wrong reason.
		expect(pre?.classList.contains('shiki')).toBe(true);
		expect(pre?.dataset.highlighted).toBe('true');

		expect(pre?.classList.contains('has-highlighted')).toBe(false);
	});

	it('skips a code block whose host is not attached to the document', async () => {
		// Rapid navigation can unmount a slide while this pass is still
		// finding its blocks; a detached container is exactly that case.
		const container = document.createElement('div');
		container.className = 'slot';
		container.innerHTML = '<pre><code class="language-js">const x = 1;</code></pre>';

		await highlightCodeBlocksInElement(container);

		const pre = container.querySelector('pre');
		expect(pre?.classList.contains('shiki')).toBe(false);
		expect(pre?.dataset.highlighted).toBeUndefined();
		expect(container.querySelector('code.language-js')).not.toBeNull();
	});
});

describe('parseHighlightLines', () => {
	it('parses a simple list and range', () => {
		expect(parseHighlightLines('1,3-5,7')).toEqual(new Set([1, 3, 4, 5, 7]));
	});

	it('parses an array of line numbers', () => {
		expect(parseHighlightLines([1, 3, 5])).toEqual(new Set([1, 3, 5]));
	});

	it('clamps a range to the given line count instead of hanging', () => {
		// Without a cap, expanding {1-999999999} iterates a billion times.
		const result = parseHighlightLines('1-999999999', 10);
		expect(result).toEqual(new Set([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]));
	});

	it('drops a range that starts past the given line count', () => {
		const result = parseHighlightLines('50-60', 10);
		expect(result).toEqual(new Set());
	});

	it('clamps a single line number past the given line count', () => {
		const result = parseHighlightLines('1,50', 10);
		expect(result).toEqual(new Set([1]));
	});

	it('falls back to a hard cap when no line count is given', () => {
		const result = parseHighlightLines('9999-10005');
		expect(Math.max(...result)).toBe(10000);
		expect(result.has(10001)).toBe(false);
	});
});
