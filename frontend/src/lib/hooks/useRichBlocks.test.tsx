import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/react';
import { StrictMode, useRef, useState } from 'react';
import type { Slide } from '$lib/types';
import { useRichBlocks, type LiveCodeBlockPortal } from './useRichBlocks';
import type { MermaidThemeOverrides } from '../utils/mermaid';
import { disposeHighlighter } from '../utils/highlighting';

// Mock the shiki module the same way mermaid.test.ts mocks 'mermaid': stub
// the third-party library so the hook is exercised end to end without
// pulling in the real WASM-backed highlighter.
vi.mock('shiki', () => ({
	createHighlighter: vi.fn(async () => ({
		codeToHtml: vi.fn(
			() =>
				'<pre class="shiki" style="background-color:var(--shiki-background)"><code><span class="line">const x = 1;</span></code></pre>'
		),
		loadLanguage: vi.fn(async () => {}),
		dispose: vi.fn()
	}))
}));

// Stub asciinema-player the same way: every created player is a fresh
// object with its own dispose spy, collected in createdAsciinemaPlayers so
// tests can assert on exactly which instances were disposed and when.
const createdAsciinemaPlayers: { dispose: ReturnType<typeof vi.fn> }[] = [];
vi.mock('asciinema-player', () => ({
	create: vi.fn(() => {
		const player = {
			dispose: vi.fn(),
			getCurrentTime: vi.fn(() => 0),
			getDuration: vi.fn(() => 0),
			play: vi.fn(async () => {}),
			pause: vi.fn(async () => {}),
			seek: vi.fn(async () => {}),
			addEventListener: vi.fn(),
			element: document.createElement('div')
		};
		createdAsciinemaPlayers.push(player);
		return player;
	})
}));
vi.mock('asciinema-player/dist/bundle/asciinema-player.css', () => ({}));

function createSlide(overrides: Partial<Slide> = {}): Slide {
	return {
		index: 0,
		layout: 'default',
		html: '',
		slots: {},
		slotOrder: [],
		fragmentCount: 0,
		steps: 1,
		...overrides
	};
}

function Harness({ slide, active }: { slide: Slide; active: boolean }) {
	const ref = useRef<HTMLDivElement>(null);
	useRichBlocks(ref, { slide, active, printMode: false });
	return <div ref={ref} className="slot" dangerouslySetInnerHTML={{ __html: slide.slots.body ?? '' }} />;
}

function ThemeSwitchHarness({
	slide,
	mermaidOverrides
}: {
	slide: Slide;
	mermaidOverrides?: MermaidThemeOverrides;
}) {
	const ref = useRef<HTMLDivElement>(null);
	useRichBlocks(ref, { slide, active: true, printMode: false, mermaidOverrides });
	return <div ref={ref} className="slot" dangerouslySetInnerHTML={{ __html: slide.slots.body ?? '' }} />;
}

function LiveCodeHarness({ slide }: { slide: Slide }) {
	const ref = useRef<HTMLDivElement>(null);
	const [portals, setPortals] = useState<LiveCodeBlockPortal[]>([]);
	useRichBlocks(ref, { slide, active: true, printMode: false, onLiveCodeBlocksChange: setPortals });
	return (
		<div>
			<div ref={ref} className="slot" dangerouslySetInnerHTML={{ __html: slide.slots.body ?? '' }} />
			<output data-testid="portal-count">{portals.length}</output>
		</div>
	);
}

describe('useRichBlocks', () => {
	beforeEach(() => {
		disposeHighlighter();
		createdAsciinemaPlayers.length = 0;
	});

	afterEach(() => {
		cleanup();
		disposeHighlighter();
	});

	it('replaces a plain code block with shiki-highlighted output after the effect runs', async () => {
		const slide = createSlide({
			slots: { body: '<pre><code class="language-js">const x = 1;</code></pre>' },
			slotOrder: ['body']
		});

		const { container } = render(<Harness slide={slide} active />);

		await waitFor(() => {
			expect(container.querySelector('pre.shiki')).not.toBeNull();
		});

		expect(container.querySelector('pre > code.language-js')).toBeNull();
	});

	it('does nothing when the slide is not active', async () => {
		const slide = createSlide({
			slots: { body: '<pre><code class="language-js">const x = 1;</code></pre>' },
			slotOrder: ['body']
		});

		const { container } = render(<Harness slide={slide} active={false} />);

		// Give any stray async work a chance to run.
		await new Promise((resolve) => setTimeout(resolve, 0));

		expect(container.querySelector('pre.shiki')).toBeNull();
		expect(container.querySelector('pre > code.language-js')).not.toBeNull();
	});

	it('marks the processed element with a short hash instead of the raw slot content', async () => {
		const slide = createSlide({
			slots: { body: '<pre><code class="language-js">const x = 1;</code></pre>' },
			slotOrder: ['body']
		});

		const { container } = render(<Harness slide={slide} active />);

		await waitFor(() => {
			expect(container.querySelector('pre.shiki')).not.toBeNull();
		});

		const element = container.querySelector('[data-rich-processed]') as HTMLElement;
		expect(element).not.toBeNull();
		expect(element.dataset.richProcessed).not.toBe(slide.slots.body);
		expect(element.dataset.richProcessed?.length).toBeLessThan((slide.slots.body ?? '').length);
	});

	it('reprocesses when the slot HTML changes, and skips when it stays the same', async () => {
		const firstSlide = createSlide({
			slots: { body: '<pre><code class="language-js">const x = 1;</code></pre>' },
			slotOrder: ['body']
		});

		const { container, rerender } = render(<Harness slide={firstSlide} active />);
		await waitFor(() => expect(container.querySelector('pre.shiki')).not.toBeNull());
		const firstHash = (container.querySelector('[data-rich-processed]') as HTMLElement).dataset
			.richProcessed;

		// Re-rendering with the exact same slot content must not touch the
		// marker: the hook detects it as already processed and skips reprocessing.
		rerender(<Harness slide={{ ...firstSlide }} active />);
		await new Promise((resolve) => setTimeout(resolve, 0));
		expect(
			(container.querySelector('[data-rich-processed]') as HTMLElement).dataset.richProcessed
		).toBe(firstHash);

		// Different slot content must produce a different hash, so the
		// changed HTML is detected as new and reprocessed.
		const secondSlide = createSlide({
			slots: { body: '<pre><code class="language-js">const y = 2;</code></pre>' },
			slotOrder: ['body']
		});
		rerender(<Harness slide={secondSlide} active />);
		await waitFor(() => {
			const hash = (container.querySelector('[data-rich-processed]') as HTMLElement).dataset
				.richProcessed;
			expect(hash).not.toBe(firstHash);
		});
	});

	it('replaces a driver code block\'s pre with a portal container and reports it', async () => {
		const slide = createSlide({
			slots: {
				body: '<pre><code class="language-sql" data-code-block-index="0">SELECT 1;</code></pre>'
			},
			slotOrder: ['body'],
			codeBlocks: [{ language: 'sql', code: 'SELECT 1;', driver: 'sqlite', connection: 'demo' }]
		});

		const { container, getByTestId } = render(<LiveCodeHarness slide={slide} />);

		await waitFor(() => {
			expect(getByTestId('portal-count')).toHaveTextContent('1');
		});

		expect(container.querySelector('pre > code.language-sql')).toBeNull();
		expect(container.querySelector('.live-code-block-portal')).not.toBeNull();
	});

	it('leaves a plain code block (no driver) for normal highlighting', async () => {
		const slide = createSlide({
			slots: {
				body: '<pre><code class="language-js" data-code-block-index="0">const x = 1;</code></pre>'
			},
			slotOrder: ['body'],
			codeBlocks: [{ language: 'js', code: 'const x = 1;' }]
		});

		const { getByTestId } = render(<LiveCodeHarness slide={slide} />);

		await waitFor(() => {
			expect(getByTestId('portal-count')).toHaveTextContent('0');
		});
	});

	it('mounts the live code block on its own pre by data-code-block-index, even when the DOM order differs from source order', async () => {
		// Regression test for pairing codeBlocks[i] with the i-th <pre> in the
		// DOM (position-based pairing): a two-column layout can render
		// ::right before ::left, so the DOM order here (js block first, sql
		// block second) is the reverse of source/codeBlocks order (sql at
		// index 0, js at index 1). Position-based pairing would portal the
		// driver widget onto the js block's pre instead of the sql block's.
		const slide = createSlide({
			slots: {
				body:
					'<pre><code class="language-js" data-code-block-index="1">const x = 1;</code></pre>' +
					'<pre><code class="language-sql" data-code-block-index="0">SELECT 1;</code></pre>'
			},
			slotOrder: ['body'],
			codeBlocks: [
				{ language: 'sql', code: 'SELECT 1;', driver: 'sqlite', connection: 'demo' },
				{ language: 'js', code: 'const x = 1;' }
			]
		});

		const { container, getByTestId } = render(<LiveCodeHarness slide={slide} />);

		await waitFor(() => {
			expect(getByTestId('portal-count')).toHaveTextContent('1');
		});

		// The sql block (codeBlocks[0], the one with a driver) must have been
		// portaled, not the js block that happens to come first in the DOM.
		// The js block's own source text survives (it goes on to be
		// highlighted, which rewrites its markup but not its content), while
		// the sql block's source text is gone, replaced by the portal wrapper.
		expect(container.querySelector('.live-code-block-portal')).not.toBeNull();
		expect(container.textContent).not.toContain('SELECT 1;');
		expect(container.textContent).toContain('const x = 1;');
	});

	it('removes the pre for a valid map code block', async () => {
		const mapCode = 'start: 40.7128, -74.0060\nend: 34.0522, -118.2437';
		const slide = createSlide({
			slots: {
				body: `<pre><code class="language-map" data-code-block-index="0">${mapCode}</code></pre>`
			},
			slotOrder: ['body'],
			codeBlocks: [{ language: 'map', code: mapCode }]
		});

		const { container } = render(<LiveCodeHarness slide={slide} />);

		await waitFor(() => {
			expect(container.querySelector('pre')).toBeNull();
		});
	});

	it('finishes highlighting under React.StrictMode, where the effect runs, cleans up, and runs again', async () => {
		const slide = createSlide({
			slots: { body: '<pre><code class="language-js">const x = 1;</code></pre>' },
			slotOrder: ['body']
		});

		const { container } = render(
			<StrictMode>
				<Harness slide={slide} active />
			</StrictMode>
		);

		await waitFor(() => {
			expect(container.querySelector('pre.shiki')).not.toBeNull();
		});

		expect(container.querySelector('pre > code.language-js')).toBeNull();
	});

	it('finishes highlighting even when the theme resolves (mermaidOverrides changes) before Shiki completes', async () => {
		// Mirrors loading with `?theme=<slug>`: App.tsx starts with the base
		// theme, then swaps mermaidOverrides once loadTheme resolves, which
		// re-runs this hook for the same slide content while the first run's
		// async chain (initHighlighter -> highlight) is still in flight.
		const slide = createSlide({
			slots: { body: '<pre><code class="language-js">const x = 1;</code></pre>' },
			slotOrder: ['body']
		});

		const { container, rerender } = render(<ThemeSwitchHarness slide={slide} />);
		rerender(<ThemeSwitchHarness slide={slide} mermaidOverrides={{ curve: 'linear' }} />);

		await waitFor(() => {
			expect(container.querySelector('pre.shiki')).not.toBeNull();
		});

		expect(container.querySelector('pre > code.language-js')).toBeNull();
	});

	it('leaves an invalid map code block in place for error display', async () => {
		const mapCode = 'start: not-a-coordinate';
		const slide = createSlide({
			slots: {
				body: `<pre><code class="language-map" data-code-block-index="0">${mapCode}</code></pre>`
			},
			slotOrder: ['body'],
			codeBlocks: [{ language: 'map', code: mapCode }]
		});

		render(<LiveCodeHarness slide={slide} />);

		// Give the synchronous mount pass a turn; the map pre should survive it.
		await new Promise((resolve) => setTimeout(resolve, 0));

		expect(document.querySelector('pre')).not.toBeNull();
	});

	it('stops processing once the host is unmounted by rapid navigation, instead of highlighting a detached node', async () => {
		const slide = createSlide({
			slots: { body: '<pre><code class="language-js">const x = 1;</code></pre>' },
			slotOrder: ['body']
		});

		const { container, unmount } = render(<Harness slide={slide} active />);
		// Keep a reference to the hook's own host element (the div the ref is
		// attached to, `elementRef.current` inside the hook): React's unmount
		// empties `container` itself, so assertions after unmount need their
		// own handle on the (now detached, but still intact) subtree the hook
		// was working on.
		const host = container.querySelector('[data-rich-processed]');
		expect(host).not.toBeNull();
		expect(host?.querySelector('pre')).not.toBeNull();

		// Unmount synchronously, right after mount: the effect's async chain
		// has only run its synchronous portion (through mountRichCodeBlocks)
		// and is suspended on its first await, exactly like a slide changed
		// out from under it mid-navigation.
		unmount();
		expect(host?.isConnected).toBe(false);

		// Give any in-flight async work every chance to run; it must not
		// highlight a node that is no longer in the document.
		await new Promise((resolve) => setTimeout(resolve, 20));

		expect(host?.querySelector('pre.shiki')).toBeNull();
		expect(host?.querySelector('pre > code.language-js')).not.toBeNull();
	});

	it('disposes the mounted asciinema player when the host unmounts', async () => {
		const slide = createSlide({
			slots: { body: '<pre><code class="language-asciinema">src: ./demo.cast</code></pre>' },
			slotOrder: ['body']
		});

		const { container, unmount } = render(<Harness slide={slide} active />);

		await waitFor(() => {
			expect(container.querySelector('.asciinema-player-wrapper')).not.toBeNull();
		});
		expect(createdAsciinemaPlayers).toHaveLength(1);
		expect(createdAsciinemaPlayers[0]?.dispose).not.toHaveBeenCalled();

		unmount();

		expect(createdAsciinemaPlayers[0]?.dispose).toHaveBeenCalledTimes(1);
	});

	it("disposes the previous slide's asciinema player when the content changes, and mounts a new one", async () => {
		const firstSlide = createSlide({
			slots: { body: '<pre><code class="language-asciinema">src: ./one.cast</code></pre>' },
			slotOrder: ['body']
		});
		const secondSlide = createSlide({
			index: 1,
			slots: { body: '<pre><code class="language-asciinema">src: ./two.cast</code></pre>' },
			slotOrder: ['body']
		});

		const { container, rerender } = render(<Harness slide={firstSlide} active />);

		await waitFor(() => {
			expect(container.querySelector('.asciinema-player-wrapper')).not.toBeNull();
		});
		expect(createdAsciinemaPlayers).toHaveLength(1);
		const firstPlayer = createdAsciinemaPlayers[0];

		rerender(<Harness slide={secondSlide} active />);

		await waitFor(() => {
			expect(createdAsciinemaPlayers).toHaveLength(2);
		});
		expect(firstPlayer?.dispose).toHaveBeenCalledTimes(1);
		expect(createdAsciinemaPlayers[1]?.dispose).not.toHaveBeenCalled();
	});

	it('does not dispose the asciinema player merely because mermaidOverrides changes for the same content', async () => {
		const slide = createSlide({
			slots: { body: '<pre><code class="language-asciinema">src: ./demo.cast</code></pre>' },
			slotOrder: ['body']
		});

		const { container, rerender } = render(<ThemeSwitchHarness slide={slide} />);

		await waitFor(() => {
			expect(container.querySelector('.asciinema-player-wrapper')).not.toBeNull();
		});
		expect(createdAsciinemaPlayers).toHaveLength(1);

		rerender(<ThemeSwitchHarness slide={slide} mermaidOverrides={{ curve: 'linear' }} />);

		// Give any stray async work a chance to run.
		await new Promise((resolve) => setTimeout(resolve, 20));

		expect(createdAsciinemaPlayers).toHaveLength(1);
		expect(createdAsciinemaPlayers[0]?.dispose).not.toHaveBeenCalled();
	});
});
