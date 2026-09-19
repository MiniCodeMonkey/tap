import { describe, expect, it } from 'vitest';
import { disposeHighlighter, highlight } from './highlighting';

/**
 * Exercises line transformations (highlighted lines, line numbers, diff
 * mode) against the real Shiki highlighter, not the mocked one every other
 * test in this file's siblings use. Shiki nests one span per token inside
 * each line span, so a regex-based line splitter that stops at the first
 * closing `</span>` truncates every multi-token line down to its first
 * token. Only this path (line spec present) exercises that nesting; plain
 * highlighting with no line spec never called applyLineTransformations, so
 * the bug went unnoticed until a fence with a line-highlight spec actually
 * reached highlight() with real Shiki output.
 */
describe('highlightWithMetadata line transformations against real Shiki output', () => {
	it('keeps every token of every line intact when a line-highlight spec is set', async () => {
		disposeHighlighter();

		const code = ['function greet(name) {', '  var message = "hello " + name;', '  return message;', '}'].join(
			'\n'
		);

		const html = await highlight(code, { language: 'javascript', highlightLines: '3' });

		const container = document.createElement('div');
		container.innerHTML = html;

		const pre = container.querySelector('pre');
		expect(pre?.classList.contains('has-highlighted')).toBe(true);

		const lines = container.querySelectorAll('.line');
		expect(lines).toHaveLength(4);

		const lineTexts = Array.from(lines).map((line) => line.textContent ?? '');
		expect(lineTexts[0]).toBe('function greet(name) {');
		expect(lineTexts[1]).toBe('  var message = "hello " + name;');
		expect(lineTexts[2]).toBe('  return message;');
		expect(lineTexts[3]).toBe('}');

		expect(lines[0]?.classList.contains('highlighted')).toBe(false);
		expect(lines[2]?.classList.contains('highlighted')).toBe(true);

		disposeHighlighter();
	}, 20000);

	it('keeps every token intact with line numbers on', async () => {
		disposeHighlighter();

		const code = ['const a = 1;', 'const b = a + 2;'].join('\n');
		const html = await highlight(code, { language: 'javascript', showLineNumbers: true });

		const container = document.createElement('div');
		container.innerHTML = html;

		const lines = container.querySelectorAll('.line');
		expect(lines).toHaveLength(2);

		const contentTexts = Array.from(container.querySelectorAll('.line-content')).map(
			(el) => el.textContent ?? ''
		);
		expect(contentTexts[0]).toBe('const a = 1;');
		expect(contentTexts[1]).toBe('const b = a + 2;');

		const numberEls = container.querySelectorAll('.line-number');
		expect(numberEls).toHaveLength(2);
		expect(numberEls[0]?.textContent).toBe('1');
		expect(numberEls[0]?.getAttribute('data-line')).toBe('1');
		expect(numberEls[1]?.textContent).toBe('2');

		disposeHighlighter();
	}, 20000);

	it('keeps every token intact in diff mode and marks +/- lines', async () => {
		disposeHighlighter();

		const code = ['function total(items) {', '- return items.length;', '+ return items.length * 2;', '}'].join(
			'\n'
		);
		const html = await highlight(code, { language: 'javascript', isDiff: true });

		const container = document.createElement('div');
		container.innerHTML = html;

		const lines = container.querySelectorAll('.line');
		expect(lines).toHaveLength(4);

		const lineTexts = Array.from(lines).map((line) => line.textContent ?? '');
		expect(lineTexts[1]).toBe('- return items.length;');
		expect(lineTexts[2]).toBe('+ return items.length * 2;');

		expect(lines[1]?.classList.contains('diff-remove')).toBe(true);
		expect(lines[2]?.classList.contains('diff-add')).toBe(true);
		expect(lines[0]?.classList.contains('diff-add')).toBe(false);
		expect(lines[0]?.classList.contains('diff-remove')).toBe(false);

		disposeHighlighter();
	}, 20000);
});
