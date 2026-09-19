/**
 * Unit tests for mermaid initialization and configuration.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	initializeMermaid,
	isMermaidInitialized,
	resetMermaidInitialization,
	getMermaid,
	renderMermaidDiagram,
	renderMermaidBlocksInElement,
	resetDiagramCounter,
	getMermaidTheme
} from './mermaid';

// Mock mermaid module
vi.mock('mermaid', () => ({
	default: {
		initialize: vi.fn(),
		render: vi.fn()
	}
}));

describe('getMermaidTheme', () => {
	it('uses the base mermaid theme', () => {
		const config = getMermaidTheme();
		expect(config.theme).toBe('base');
	});

	it('uses literal colors matching base.css, not CSS custom properties', () => {
		const config = getMermaidTheme();
		expect(config.themeVariables.background).toBe('#ffffff');
		expect(config.themeVariables.textColor).toBe('#18181b');
		expect(config.themeVariables.primaryBorderColor).toBe('#2563eb');
		// Mermaid's theming engine parses each color to derive shades and
		// throws on a var(...) string it can't parse as a color.
		expect(config.themeVariables.background).not.toContain('var(');
	});

	it('uses a single bare family name for fontFamily, with no commas or quotes', () => {
		const config = getMermaidTheme();
		expect(config.themeVariables.fontFamily).toBe('Inter');
		expect(config.themeVariables.fontFamily).not.toContain(',');
		expect(config.themeVariables.fontFamily).not.toContain('"');
		expect(config.themeVariables.fontFamily).not.toContain("'");
	});

	it('defaults to the basis curve', () => {
		const config = getMermaidTheme();
		expect(config.curve).toBe('basis');
	});

	it('accepts a curve override', () => {
		const config = getMermaidTheme({ curve: 'linear' });
		expect(config.curve).toBe('linear');
	});

	it('merges themeVariables overrides over the base defaults', () => {
		const config = getMermaidTheme({ themeVariables: { primaryColor: '#123456' } });
		expect(config.themeVariables.primaryColor).toBe('#123456');
		// Unrelated defaults are still present.
		expect(config.themeVariables.fontFamily).toBe('Inter');
	});

	it('drops the edge label background when quietStyle is set', () => {
		const config = getMermaidTheme({ quietStyle: 'fill:#000,stroke:#fff' });
		expect(config.themeVariables.edgeLabelBackground).toBe('transparent');
	});
});

describe('mermaid initialization', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		resetMermaidInitialization();
		resetDiagramCounter();
	});

	describe('initializeMermaid', () => {
		it('initializes mermaid with startOnLoad: false', async () => {
			const mermaid = await import('mermaid');

			initializeMermaid();

			expect(mermaid.default.initialize).toHaveBeenCalledWith(
				expect.objectContaining({
					startOnLoad: false,
					securityLevel: 'strict'
				})
			);
		});

		it('only initializes once when called multiple times without overrides', async () => {
			const mermaid = await import('mermaid');

			initializeMermaid();
			initializeMermaid();
			initializeMermaid();

			expect(mermaid.default.initialize).toHaveBeenCalledTimes(1);
		});

		it('reinitializes when the overrides change', async () => {
			const mermaid = await import('mermaid');

			initializeMermaid();
			initializeMermaid({ curve: 'linear' });

			expect(mermaid.default.initialize).toHaveBeenCalledTimes(2);
		});

		it('does not reinitialize when called with the same overrides', async () => {
			const mermaid = await import('mermaid');

			initializeMermaid({ curve: 'linear' });
			initializeMermaid({ curve: 'linear' });

			expect(mermaid.default.initialize).toHaveBeenCalledTimes(1);
		});

		it('uses the base theme config', async () => {
			const mermaid = await import('mermaid');

			initializeMermaid();

			expect(mermaid.default.initialize).toHaveBeenCalledWith(
				expect.objectContaining({
					theme: 'base',
					themeVariables: expect.objectContaining({
						background: '#ffffff'
					})
				})
			);
		});
	});

	describe('isMermaidInitialized', () => {
		it('returns false before initialization', () => {
			expect(isMermaidInitialized()).toBe(false);
		});

		it('returns true after initialization', () => {
			initializeMermaid();
			expect(isMermaidInitialized()).toBe(true);
		});
	});

	describe('resetMermaidInitialization', () => {
		it('resets initialization state', () => {
			initializeMermaid();
			expect(isMermaidInitialized()).toBe(true);

			resetMermaidInitialization();
			expect(isMermaidInitialized()).toBe(false);
		});

		it('allows re-initialization after reset', async () => {
			const mermaid = await import('mermaid');

			initializeMermaid();
			resetMermaidInitialization();
			initializeMermaid();

			expect(mermaid.default.initialize).toHaveBeenCalledTimes(2);
		});
	});

	describe('getMermaid', () => {
		it('returns the mermaid instance', async () => {
			const mermaid = await import('mermaid');
			const instance = getMermaid();
			expect(instance).toBe(mermaid.default);
		});
	});
});

describe('mermaid rendering', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		resetMermaidInitialization();
		resetDiagramCounter();
	});

	afterEach(() => {
		document.body.innerHTML = '';
	});

	describe('renderMermaidDiagram', () => {
		it('renders a valid mermaid diagram', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg>test diagram</svg>' });

			const result = await renderMermaidDiagram('graph TD\nA-->B');

			expect(result.success).toBe(true);
			if (result.success) {
				expect(result.svg).toBe('<svg>test diagram</svg>');
			}
		});

		it('generates unique IDs for each diagram', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg></svg>' });

			await renderMermaidDiagram('graph TD\nA-->B');
			await renderMermaidDiagram('graph TD\nC-->D');

			expect(mockRender).toHaveBeenNthCalledWith(1, 'mermaid-diagram-1', expect.any(String));
			expect(mockRender).toHaveBeenNthCalledWith(2, 'mermaid-diagram-2', expect.any(String));
		});

		it('returns error result when rendering fails', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockRejectedValue(new Error('Parse error'));

			const result = await renderMermaidDiagram('invalid diagram');

			expect(result.success).toBe(false);
			if (!result.success) {
				expect(result.error).toBe('Parse error');
				expect(result.code).toBe('invalid diagram');
			}
		});

		it('appends a quiet classDef when the theme sets a quietStyle', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg></svg>' });

			await renderMermaidDiagram('graph TD\nA-->B', { quietStyle: 'fill:#000,stroke:#fff' });

			expect(mockRender).toHaveBeenCalledWith(
				expect.any(String),
				'graph TD\nA-->B\nclassDef quiet fill:#000,stroke:#fff'
			);
		});

		it('does not append a quiet classDef when the diagram already defines one', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg></svg>' });

			const code = 'graph TD\nA-->B\nclassDef quiet fill:#111';
			await renderMermaidDiagram(code, { quietStyle: 'fill:#000,stroke:#fff' });

			expect(mockRender).toHaveBeenCalledWith(expect.any(String), code);
		});

		it('reports the original code (without the injected classDef) on error', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockRejectedValue(new Error('bad diagram'));

			const result = await renderMermaidDiagram('graph TD\nA-->B', { quietStyle: 'fill:#000' });

			expect(result.success).toBe(false);
			if (!result.success) {
				expect(result.code).toBe('graph TD\nA-->B');
			}
		});

		it('handles non-Error exceptions', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockRejectedValue('string error');

			const result = await renderMermaidDiagram('invalid diagram');

			expect(result.success).toBe(false);
			if (!result.success) {
				expect(result.error).toBe('string error');
			}
		});

		it('initializes mermaid before rendering', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg></svg>' });

			expect(isMermaidInitialized()).toBe(false);
			await renderMermaidDiagram('graph TD\nA-->B');
			expect(isMermaidInitialized()).toBe(true);
		});

		it('does not throw when document.fonts is unavailable (as in this test environment)', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg></svg>' });

			expect(document.fonts).toBeUndefined();
			await expect(renderMermaidDiagram('graph TD\nA-->B')).resolves.toMatchObject({ success: true });
		});
	});

	// Every container below is appended to document.body:
	// renderMermaidBlocksInElement skips a detached element's render work
	// (see the isConnected guard in mermaid.ts), so a container the browser
	// would actually render into needs to be in the document here too, the
	// way useRichBlocks's host actually is.
	describe('renderMermaidBlocksInElement', () => {
		it('renders mermaid code blocks in an element', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg class="rendered">flowchart</svg>' });

			// Create DOM element with mermaid code block
			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-mermaid">graph TD
A-->B</code></pre>
      `;

			await renderMermaidBlocksInElement(container);

			// Check that the pre was replaced with a diagram container
			expect(container.querySelector('pre')).toBeNull();
			const diagram = container.querySelector('.mermaid-diagram') as HTMLElement;
			expect(diagram).not.toBeNull();
			expect(diagram?.innerHTML).toBe('<svg class="rendered">flowchart</svg>');
		});

		it('stores original mermaid code as a data attribute for re-rendering', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg>diagram</svg>' });

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			const mermaidCode = 'graph TD\nA-->B';
			container.innerHTML = `
        <pre><code class="language-mermaid">${mermaidCode}</code></pre>
      `;

			await renderMermaidBlocksInElement(container);

			const diagram = container.querySelector('.mermaid-diagram') as HTMLElement;
			expect(diagram).not.toBeNull();
			expect(diagram?.dataset.mermaidCode).toBe(mermaidCode);
		});

		it('handles multiple mermaid blocks', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg>diagram</svg>' });

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-mermaid">graph TD\nA-->B</code></pre>
        <p>Some text</p>
        <pre><code class="language-mermaid">graph TD\nC-->D</code></pre>
      `;

			await renderMermaidBlocksInElement(container);

			const diagrams = container.querySelectorAll('.mermaid-diagram');
			expect(diagrams.length).toBe(2);
			expect(mockRender).toHaveBeenCalledTimes(2);
		});

		it('applies only the render for the configuration requested last, even when it settles second', async () => {
			// Reproduces a cold ?theme=<slug> load: a render with the base
			// config starts, then the theme's config resolves and starts a
			// second, overlapping render for the same block. The base render
			// (requested first) finishing first is exactly the case that used
			// to win permanently, since by the time the theme render finished
			// second, the original <pre> was already detached and its own
			// replaceWith became a silent no-op.
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);

			let resolveBase: (value: { svg: string }) => void = () => {};
			let resolveTheme: (value: { svg: string }) => void = () => {};
			const basePromise = new Promise<{ svg: string }>((resolve) => {
				resolveBase = resolve;
			});
			const themePromise = new Promise<{ svg: string }>((resolve) => {
				resolveTheme = resolve;
			});

			mockRender.mockImplementationOnce(() => basePromise).mockImplementationOnce(() => themePromise);

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-mermaid">graph TD
A-->B</code></pre>
      `;

			const baseCall = renderMermaidBlocksInElement(container);
			const themeCall = renderMermaidBlocksInElement(container, { curve: 'linear' });

			// The base-config render, requested first, settles first.
			resolveBase({ svg: '<svg class="base">diagram</svg>' });
			await baseCall;

			// The theme-config render, requested second (last), settles after.
			// Its result must still apply: it is the latest request.
			resolveTheme({ svg: '<svg class="theme">diagram</svg>' });
			await themeCall;

			const diagram = container.querySelector('.mermaid-diagram') as HTMLElement;
			expect(diagram?.innerHTML).toBe('<svg class="theme">diagram</svg>');
			expect(container.querySelectorAll('.mermaid-diagram').length).toBe(1);
		});

		it('does nothing when no mermaid blocks exist', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-javascript">const x = 1;</code></pre>
      `;

			await renderMermaidBlocksInElement(container);

			expect(mockRender).not.toHaveBeenCalled();
			expect(container.querySelector('pre')).not.toBeNull();
		});

		it('shows error message when rendering fails', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockRejectedValue(new Error('Syntax error'));

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-mermaid">invalid syntax</code></pre>
      `;

			await renderMermaidBlocksInElement(container);

			// Check that error container was created
			expect(container.querySelector('pre > code.language-mermaid')).toBeNull();
			const errorContainer = container.querySelector('.mermaid-error');
			expect(errorContainer).not.toBeNull();
			expect(errorContainer?.querySelector('.mermaid-error-message')?.textContent).toContain('Syntax error');
			expect(errorContainer?.querySelector('.mermaid-error-code code')?.textContent).toBe('invalid syntax');
		});

		it('escapes HTML in error messages', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockRejectedValue(new Error('<script>alert("xss")</script>'));

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-mermaid"><script>bad</script></code></pre>
      `;

			await renderMermaidBlocksInElement(container);

			const errorMessage = container.querySelector('.mermaid-error-message');
			expect(errorMessage?.innerHTML).not.toContain('<script>');
			expect(errorMessage?.textContent).toContain('<script>');
		});

		it('preserves non-mermaid code blocks', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg>diagram</svg>' });

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-javascript">const x = 1;</code></pre>
        <pre><code class="language-mermaid">graph TD\nA-->B</code></pre>
        <pre><code class="language-python">print("hello")</code></pre>
      `;

			await renderMermaidBlocksInElement(container);

			// Mermaid block should be replaced
			expect(container.querySelector('.mermaid-diagram')).not.toBeNull();
			// Other code blocks should remain
			expect(container.querySelector('code.language-javascript')).not.toBeNull();
			expect(container.querySelector('code.language-python')).not.toBeNull();
		});

		it('re-renders existing diagrams when the overrides change', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender
				.mockResolvedValueOnce({ svg: '<svg>first</svg>' })
				.mockResolvedValueOnce({ svg: '<svg>second</svg>' });

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-mermaid">graph TD\nA-->B</code></pre>
      `;

			// First render with default overrides
			await renderMermaidBlocksInElement(container);
			let diagram = container.querySelector('.mermaid-diagram') as HTMLElement;
			expect(diagram?.innerHTML).toBe('<svg>first</svg>');

			// Re-render with a different curve
			await renderMermaidBlocksInElement(container, { curve: 'linear' });
			diagram = container.querySelector('.mermaid-diagram') as HTMLElement;
			expect(diagram?.innerHTML).toBe('<svg>second</svg>');
		});

		it('does not re-render when the overrides are the same', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg>diagram</svg>' });

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-mermaid">graph TD\nA-->B</code></pre>
      `;

			// First render
			await renderMermaidBlocksInElement(container);
			expect(mockRender).toHaveBeenCalledTimes(1);

			// Same overrides - should skip re-render
			await renderMermaidBlocksInElement(container);
			expect(mockRender).toHaveBeenCalledTimes(1);
		});

		it('re-renders multiple diagrams when the overrides change', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg>diagram</svg>' });

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-mermaid">graph TD\nA-->B</code></pre>
        <pre><code class="language-mermaid">graph TD\nC-->D</code></pre>
      `;

			// First render
			await renderMermaidBlocksInElement(container);
			expect(mockRender).toHaveBeenCalledTimes(2);

			// Overrides change should re-render both
			await renderMermaidBlocksInElement(container, { curve: 'linear' });
			expect(mockRender).toHaveBeenCalledTimes(4);

			const diagrams = container.querySelectorAll('.mermaid-diagram');
			expect(diagrams.length).toBe(2);
		});

		it('stores mermaid code on error containers for retry on override change', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockRejectedValue(new Error('Parse error'));

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			const code = 'invalid syntax';
			container.innerHTML = `
        <pre><code class="language-mermaid">${code}</code></pre>
      `;

			await renderMermaidBlocksInElement(container);

			const errorContainer = container.querySelector('.mermaid-error') as HTMLElement;
			expect(errorContainer).not.toBeNull();
			expect(errorContainer?.dataset.mermaidCode).toBe(code);
		});

		it('retries rendering error containers on override change and converts to a diagram if successful', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender
				.mockRejectedValueOnce(new Error('Parse error'))
				.mockResolvedValueOnce({ svg: '<svg>success</svg>' });

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-mermaid">graph TD\nA-->B</code></pre>
      `;

			// First render fails
			await renderMermaidBlocksInElement(container);
			expect(container.querySelector('.mermaid-error')).not.toBeNull();

			// Override change succeeds
			await renderMermaidBlocksInElement(container, { curve: 'linear' });
			expect(container.querySelector('.mermaid-error')).toBeNull();
			const diagram = container.querySelector('.mermaid-diagram') as HTMLElement;
			expect(diagram?.innerHTML).toBe('<svg>success</svg>');
		});

		it('converts a diagram to an error container if a re-render fails', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender
				.mockResolvedValueOnce({ svg: '<svg>diagram</svg>' })
				.mockRejectedValueOnce(new Error('Incompatible override'));

			const container = document.createElement('div');
			container.className = 'slot';
			document.body.appendChild(container);
			container.innerHTML = `
        <pre><code class="language-mermaid">graph TD\nA-->B</code></pre>
      `;

			// First render succeeds
			await renderMermaidBlocksInElement(container);
			expect(container.querySelector('.mermaid-diagram')).not.toBeNull();

			// Override change fails
			await renderMermaidBlocksInElement(container, { curve: 'linear' });
			expect(container.querySelector('.mermaid-diagram')).toBeNull();
			const errorContainer = container.querySelector('.mermaid-error') as HTMLElement;
			expect(errorContainer).not.toBeNull();
		});
	});

	describe('resetDiagramCounter', () => {
		it('resets the diagram counter', async () => {
			const mermaid = await import('mermaid');
			const mockRender = vi.mocked(mermaid.default.render);
			mockRender.mockResolvedValue({ svg: '<svg></svg>' });

			await renderMermaidDiagram('graph TD\nA-->B');
			resetDiagramCounter();
			await renderMermaidDiagram('graph TD\nC-->D');

			// After reset, IDs should start from 1 again
			expect(mockRender).toHaveBeenNthCalledWith(1, 'mermaid-diagram-1', expect.any(String));
			expect(mockRender).toHaveBeenNthCalledWith(2, 'mermaid-diagram-1', expect.any(String));
		});
	});
});
