import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render, waitFor } from '@testing-library/react';
import type { Slide as SlideData } from '$lib/types';
import {
	__resetDeckComponentCachesForTests,
	DeckComponent,
	loadDeckComponent,
	resolveComponentURL
} from './DeckComponent';
import { Slot } from './Slot';
import { heldBlockers, resetBlockersForTests, subscribeToBlockers, type ReadyBlockerKind } from '$lib/ready/blockers';

afterEach(() => cleanup());
beforeEach(() => __resetDeckComponentCachesForTests());

function makeSlide(overrides: Partial<SlideData> = {}): SlideData {
	return {
		index: 0,
		layout: 'component',
		html: '',
		slots: {},
		slotOrder: [],
		fragmentCount: 0,
		steps: 0,
		...overrides
	};
}

describe('resolveComponentURL', () => {
	it('resolves a relative URL against the document base, not against the caller', () => {
		expect(resolveComponentURL('components/Foo-abcd.js')).toBe(
			new URL('components/Foo-abcd.js', document.baseURI).href
		);
	});
});

describe('loadDeckComponent', () => {
	it('caches the importer promise by URL: a second load for the same URL does not import again', async () => {
		const importer = vi.fn().mockResolvedValue({ default: () => null });

		await loadDeckComponent('/components/A-1.js', importer);
		await loadDeckComponent('/components/A-1.js', importer);

		expect(importer).toHaveBeenCalledTimes(1);
	});

	it('resolves the module default export', async () => {
		function Widget() {
			return null;
		}
		const importer = vi.fn().mockResolvedValue({ default: Widget });

		await expect(loadDeckComponent('/components/B-1.js', importer)).resolves.toBe(Widget);
	});

	it('rejects with a clear message when the module has no default export', async () => {
		const importer = vi.fn().mockResolvedValue({ named: () => null });

		await expect(loadDeckComponent('/components/C-1.js', importer)).rejects.toThrow(
			/no default export/
		);
	});

});

/**
 * Simulates a static build's index.html, which embeds presentation JSON in
 * a `#presentation-data` script tag (see internal/builder/builder.go).
 * DeckComponent treats that element's presence as "this is not a
 * server-backed dev runtime" (see isDevRuntime in DeckComponent.tsx).
 */
function markAsStaticBuild(): () => void {
	const script = document.createElement('script');
	script.id = 'presentation-data';
	script.type = 'application/json';
	script.textContent = '{}';
	document.body.appendChild(script);
	return () => script.remove();
}

describe('DeckComponent', () => {
	const originalDev = import.meta.env.DEV;

	afterEach(() => {
		(import.meta.env as { DEV: boolean }).DEV = originalDev;
	});

	it('renders the loaded component with the deck component props', async () => {
		function Widget({ props }: { props: Record<string, unknown> }) {
			return <p data-testid="widget">{String(props.label)}</p>;
		}
		const importer = vi.fn().mockResolvedValue({ default: Widget });
		const slide = makeSlide();

		const { findByTestId } = render(
			<DeckComponent
				source="slides/Widget.jsx"
				url="/components/Widget-1.js"
				props={{ label: 'hello' }}
				slots={{}}
				slide={slide}
				step={0}
				steps={1}
				active
				printMode={false}
				importer={importer}
			/>
		);

		expect((await findByTestId('widget')).textContent).toBe('hello');
	});

	it('shows an error card with the class tap export images detects and the source path, in dev, on a build error', () => {
		(import.meta.env as { DEV: boolean }).DEV = true;
		const slide = makeSlide();

		const { container } = render(
			<DeckComponent
				source="slides/Broken.jsx"
				url="/components/Broken-1.js"
				buildError="slides/Broken.jsx:3:7: Unexpected &quot;}&quot;"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active
				printMode={false}
			/>
		);

		const card = container.querySelector('.deck-error-card');
		expect(card).not.toBeNull();
		expect(card?.getAttribute('data-source')).toBe('slides/Broken.jsx');
		expect(card?.getAttribute('data-message')).toBe('slides/Broken.jsx:3:7: Unexpected "}"');
		expect(card?.textContent).toContain('Unexpected');
	});

	it('renders the caller-provided fallback instead of the error card outside of dev, on a build error', () => {
		(import.meta.env as { DEV: boolean }).DEV = false;
		const unmark = markAsStaticBuild();
		const slide = makeSlide();

		const { container, queryByTestId } = render(
			<DeckComponent
				source="slides/Broken.jsx"
				url="/components/Broken-2.js"
				buildError="slides/Broken.jsx:3:7: bad"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active
				printMode={false}
				buildFallback={<p data-testid="fallback">fallback content</p>}
			/>
		);

		expect(container.querySelector('.deck-error-card')).toBeNull();
		expect(queryByTestId('fallback')).not.toBeNull();
		unmark();
	});

	it('shows an error card with the class tap export images detects when the loaded module has no default export, in dev', async () => {
		(import.meta.env as { DEV: boolean }).DEV = true;
		const importer = vi.fn().mockResolvedValue({ named: () => null });
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		const slide = makeSlide();

		const { container } = render(
			<DeckComponent
				source="slides/NoDefault.jsx"
				url="/components/NoDefault-1.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active
				printMode={false}
				importer={importer}
			/>
		);

		await waitFor(() => expect(container.querySelector('.deck-error-card')).not.toBeNull());
		expect(container.querySelector('.deck-error-card')?.getAttribute('data-source')).toBe('slides/NoDefault.jsx');
		consoleSpy.mockRestore();
	});

	it('falls back to the caller-provided content outside of dev when the module fails to load', async () => {
		(import.meta.env as { DEV: boolean }).DEV = false;
		const unmark = markAsStaticBuild();
		const importer = vi.fn().mockRejectedValue(new Error('network-ish failure'));
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		const slide = makeSlide();

		const { container, findByTestId } = render(
			<DeckComponent
				source="slides/Fails.jsx"
				url="/components/Fails-1.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active
				printMode={false}
				importer={importer}
				buildFallback={<p data-testid="fallback">fallback content</p>}
			/>
		);

		expect(await findByTestId('fallback')).toBeTruthy();
		expect(container.querySelector('.deck-error-card')).toBeNull();
		consoleSpy.mockRestore();
		unmark();
	});

	it('renders on a later mount after an earlier import for the same URL rejected, instead of repeating the failure', async () => {
		(import.meta.env as { DEV: boolean }).DEV = false;
		const unmark = markAsStaticBuild();
		function Widget() {
			return <p data-testid="widget">recovered</p>;
		}
		const importer = vi
			.fn()
			.mockRejectedValueOnce(new Error('Failed to fetch dynamically imported module'))
			.mockResolvedValueOnce({ default: Widget });
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		const slide = makeSlide();

		const first = render(
			<DeckComponent
				source="slides/Recovers.jsx"
				url="/components/Recovers-1.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active
				printMode={false}
				importer={importer}
				buildFallback={<p data-testid="fallback">fallback content</p>}
			/>
		);
		await first.findByTestId('fallback');
		first.unmount();

		const second = render(
			<DeckComponent
				source="slides/Recovers.jsx"
				url="/components/Recovers-1.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active
				printMode={false}
				importer={importer}
				buildFallback={<p data-testid="fallback">fallback content</p>}
			/>
		);

		expect((await second.findByTestId('widget')).textContent).toBe('recovered');
		consoleSpy.mockRestore();
		unmark();
	});

	it('renders a placeholder card instead of the component in a preview render when the module exports preview = false', async () => {
		function Widget() {
			return <p data-testid="widget">rendered</p>;
		}
		const importer = vi.fn().mockResolvedValue({ default: Widget, preview: false });
		const slide = makeSlide();

		const { container, queryByTestId } = render(
			<DeckComponent
				source="slides/Heavy.jsx"
				url="/components/Heavy-1.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active={false}
				printMode
				preview
				importer={importer}
			/>
		);

		await waitFor(() => expect(container.querySelector('.deck-component-preview-placeholder')).not.toBeNull());
		expect(container.textContent).toContain('slides/Heavy.jsx');
		expect(queryByTestId('widget')).toBeNull();
	});

	it('renders the component as usual in a preview render when it does not opt out', async () => {
		function Widget() {
			return <p data-testid="widget">rendered</p>;
		}
		const importer = vi.fn().mockResolvedValue({ default: Widget });
		const slide = makeSlide();

		const { findByTestId } = render(
			<DeckComponent
				source="slides/Light.jsx"
				url="/components/Light-1.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active={false}
				printMode
				preview
				importer={importer}
			/>
		);

		expect(await findByTestId('widget')).toBeTruthy();
	});

	it('keeps showing the error card across a step change, but recovers once the url/source change to a working bundle', async () => {
		(import.meta.env as { DEV: boolean }).DEV = true;
		const importer = vi.fn().mockResolvedValue({ named: () => null });
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		const slide = makeSlide();

		const { container, rerender } = render(
			<DeckComponent
				source="slides/Broken.jsx"
				url="/components/Broken-3.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={3}
				active
				printMode={false}
				importer={importer}
			/>
		);

		await waitFor(() => expect(container.querySelector('.deck-error-card')).not.toBeNull());

		// A step change on the same bundle must not clear the error: retrying
		// the same broken bundle on every press would flicker the card.
		rerender(
			<DeckComponent
				source="slides/Broken.jsx"
				url="/components/Broken-3.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={1}
				steps={3}
				active
				printMode={false}
				importer={importer}
			/>
		);
		expect(container.querySelector('.deck-error-card')).not.toBeNull();

		// A different bundle (the deck's file was fixed, so tap serves a new
		// url) is a fresh mount: its error boundary must not carry over the
		// previous bundle's failure.
		function FixedWidget() {
			return <p data-testid="fixed">fixed</p>;
		}
		const fixedImporter = vi.fn().mockResolvedValue({ default: FixedWidget });
		rerender(
			<DeckComponent
				source="slides/Broken.jsx"
				url="/components/Broken-3-fixed.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={1}
				steps={3}
				active
				printMode={false}
				importer={fixedImporter}
			/>
		);

		await waitFor(() => expect(container.querySelector('[data-testid="fixed"]')).not.toBeNull());
		expect(container.querySelector('.deck-error-card')).toBeNull();
		consoleSpy.mockRestore();
	});

	it('shows the audience-safe form (a hidden card plus a marker) when the page is opened with ?present=true, in dev', () => {
		(import.meta.env as { DEV: boolean }).DEV = true;
		window.history.pushState({}, '', '/?present=true');
		const slide = makeSlide();

		try {
			const { container } = render(
				<DeckComponent
					source="slides/Broken.jsx"
					url="/components/Broken-safe.js"
					buildError="slides/Broken.jsx:3:7: bad"
					props={{}}
					slots={{}}
					slide={slide}
					step={0}
					steps={0}
					active
					printMode={false}
				/>
			);

			const card = container.querySelector('.deck-error-card');
			expect(card).not.toBeNull();
			expect(card?.hasAttribute('hidden')).toBe(true);
			expect(card?.getAttribute('data-message')).toBe('slides/Broken.jsx:3:7: bad');
			expect(container.querySelector('.deck-error-marker')?.textContent).toBe('component error');
		} finally {
			window.history.pushState({}, '', '/');
		}
	});

	it('renders the build fallback next to the marker when a mounted component throws at render, with ?present=true, in dev', async () => {
		(import.meta.env as { DEV: boolean }).DEV = true;
		window.history.pushState({}, '', '/?present=true');
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		function Throws(): never {
			throw new Error('boom');
		}
		const importer = vi.fn().mockResolvedValue({ default: Throws });
		const slide = makeSlide();

		try {
			const { container, findByTestId } = render(
				<DeckComponent
					source="slides/Throws.jsx"
					url="/components/Throws-present.js"
					props={{}}
					slots={{}}
					slide={slide}
					step={0}
					steps={0}
					active
					printMode={false}
					importer={importer}
					buildFallback={<p data-testid="fallback">the slide's normal content</p>}
				/>
			);

			expect(await findByTestId('fallback')).toBeTruthy();
			const card = container.querySelector('.deck-error-card');
			expect(card?.hasAttribute('hidden')).toBe(true);
			expect(container.querySelector('.deck-error-marker')?.textContent).toBe('component error');
		} finally {
			window.history.pushState({}, '', '/');
			consoleSpy.mockRestore();
		}
	});

	it('switches to the audience-safe form when fullscreen is entered, without remounting', () => {
		(import.meta.env as { DEV: boolean }).DEV = true;
		const slide = makeSlide();

		const { container } = render(
			<DeckComponent
				source="slides/Broken.jsx"
				url="/components/Broken-livefullscreen.js"
				buildError="slides/Broken.jsx:3:7: bad"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active
				printMode={false}
			/>
		);

		// Not fullscreen yet: the full card, not the safe form.
		expect(container.querySelector('.deck-error-card')?.hasAttribute('hidden')).toBe(false);

		Object.defineProperty(document, 'fullscreenElement', {
			value: document.createElement('div'),
			configurable: true
		});
		act(() => {
			document.dispatchEvent(new Event('fullscreenchange'));
		});

		expect(container.querySelector('.deck-error-card')?.hasAttribute('hidden')).toBe(true);
		expect(container.querySelector('.deck-error-marker')).not.toBeNull();

		Object.defineProperty(document, 'fullscreenElement', { value: null, configurable: true });
	});

	it('shows the full error card when fullscreen, since ?present=true is absent, in dev', () => {
		(import.meta.env as { DEV: boolean }).DEV = true;
		Object.defineProperty(document, 'fullscreenElement', {
			value: document.createElement('div'),
			configurable: true
		});
		const slide = makeSlide();

		try {
			const { container } = render(
				<DeckComponent
					source="slides/Broken.jsx"
					url="/components/Broken-fullscreen.js"
					buildError="slides/Broken.jsx:3:7: bad"
					props={{}}
					slots={{}}
					slide={slide}
					step={0}
					steps={0}
					active
					printMode={false}
				/>
			);

			const card = container.querySelector('.deck-error-card');
			expect(card?.hasAttribute('hidden')).toBe(true);
			expect(container.querySelector('.deck-error-marker')).not.toBeNull();
		} finally {
			Object.defineProperty(document, 'fullscreenElement', { value: null, configurable: true });
		}
	});

	it('keeps the full error card when ?present=true is set but ?debug=true is also set, in dev', () => {
		(import.meta.env as { DEV: boolean }).DEV = true;
		window.history.pushState({}, '', '/?present=true&debug=true');
		const slide = makeSlide();

		try {
			const { container } = render(
				<DeckComponent
					source="slides/Broken.jsx"
					url="/components/Broken-debug.js"
					buildError="slides/Broken.jsx:3:7: bad"
					props={{}}
					slots={{}}
					slide={slide}
					step={0}
					steps={0}
					active
					printMode={false}
				/>
			);

			const card = container.querySelector('.deck-error-card');
			expect(card?.hasAttribute('hidden')).toBe(false);
			expect(card?.textContent).toContain('bad');
			expect(container.querySelector('.deck-error-marker')).toBeNull();
		} finally {
			window.history.pushState({}, '', '/');
		}
	});

	it('times out a component that never resolves after 8 seconds, evicting the cache so a remount retries', async () => {
		vi.useFakeTimers();
		(import.meta.env as { DEV: boolean }).DEV = true;
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		const neverResolves = () => new Promise(() => {});
		function Widget() {
			return <p data-testid="widget">recovered</p>;
		}
		const importer = vi.fn().mockImplementation((url: string) => {
			// First mount hangs forever; a second mount for the same URL (the
			// retry after eviction) resolves normally.
			return importer.mock.calls.length === 1 ? neverResolves() : Promise.resolve({ default: Widget });
		});
		const slide = makeSlide();

		const first = render(
			<DeckComponent
				source="slides/Hangs.jsx"
				url="/components/Hangs-1.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active
				printMode={false}
				importer={importer}
			/>
		);

		await act(async () => {
			await vi.advanceTimersByTimeAsync(8000);
		});

		const card = first.container.querySelector('.deck-error-card');
		expect(card).not.toBeNull();
		expect(card?.textContent).toContain('component did not load within 8 seconds: slides/Hangs.jsx');
		first.unmount();

		const second = render(
			<DeckComponent
				source="slides/Hangs.jsx"
				url="/components/Hangs-1.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active
				printMode={false}
				importer={importer}
			/>
		);
		await vi.waitFor(() => expect(second.container.querySelector('[data-testid="widget"]')).not.toBeNull());

		consoleSpy.mockRestore();
		vi.useRealTimers();
	});

	it('never times out a component in print mode', async () => {
		vi.useFakeTimers();
		(import.meta.env as { DEV: boolean }).DEV = true;
		const neverResolves = vi.fn().mockImplementation(() => new Promise(() => {}));
		const slide = makeSlide();

		const { container } = render(
			<DeckComponent
				source="slides/HangsInPrint.jsx"
				url="/components/HangsInPrint-1.js"
				props={{}}
				slots={{}}
				slide={slide}
				step={0}
				steps={0}
				active
				printMode
				importer={neverResolves}
			/>
		);

		await vi.advanceTimersByTimeAsync(60000);

		expect(container.querySelector('.deck-error-card')).toBeNull();
		vi.useRealTimers();
	});

	it('loads the component css as a stylesheet link, once per URL', async () => {
		function Widget() {
			return <p data-testid="widget">rendered</p>;
		}
		const importer = vi.fn().mockResolvedValue({ default: Widget });
		const slide = makeSlide();
		const props = {
			source: 'slides/Styled.jsx',
			url: '/components/Styled-1.js',
			css: '/components/Styled-1.css',
			props: {},
			slots: {},
			slide,
			step: 0,
			steps: 0,
			active: true,
			printMode: false,
			importer
		};

		const first = render(<DeckComponent {...props} />);
		await first.findByTestId('widget');
		first.unmount();

		const second = render(<DeckComponent {...props} />);
		await second.findByTestId('widget');

		const links = document.head.querySelectorAll('link[data-deck-component]');
		expect(links.length).toBe(1);
	});

	it("keeps a Slot's rendered DOM node identity stable across a step prop change", async () => {
		// Regression coverage for a field report: a terminal-theme typing
		// animation on a slot paragraph must not replay on every step press
		// of a whole-slide component. It can only replay if the paragraph's
		// DOM node is destroyed and recreated, so this pins the node
		// identity itself across a step change instead of asserting on any
		// particular theme's CSS.
		function WithSlot({ slots }: { slots: Record<string, string | undefined> }) {
			return <Slot html={slots.default} />;
		}
		const importer = vi.fn().mockResolvedValue({ default: WithSlot });
		const slide = makeSlide();
		const baseProps = {
			source: 'slides/Sub.jsx',
			url: '/components/Sub-1.js',
			props: {},
			slots: { default: '<p data-testid="subtitle">AWS &middot; Azure &middot; GCP</p>' },
			slide,
			steps: 3,
			active: true,
			printMode: false,
			importer
		};

		const { findByTestId, rerender } = render(<DeckComponent {...baseProps} step={0} />);
		const before = await findByTestId('subtitle');

		rerender(<DeckComponent {...baseProps} step={1} />);
		const after = await findByTestId('subtitle');

		expect(after).toBe(before);
	});
});

describe('DeckComponent and the ready signal', () => {
	beforeEach(() => resetBlockersForTests());

	function renderComponent(importer: (url: string) => Promise<unknown>, url = '/components/Chart-1.js') {
		return render(
			<DeckComponent
				source="slides/Chart.jsx"
				url={url}
				props={{}}
				slots={{}}
				slide={makeSlide()}
				step={0}
				steps={0}
				active
				printMode={false}
				importer={importer}
			/>
		);
	}

	it('holds a component blocker until the bundle has loaded and rendered', async () => {
		let resolveImport!: (module: unknown) => void;
		const importer = vi.fn(
			() =>
				new Promise((resolve) => {
					resolveImport = resolve;
				})
		);
		const { container } = renderComponent(importer);
		expect(heldBlockers()).toEqual(['component']);

		await act(async () => {
			resolveImport({ default: () => <p className="chart">chart</p> });
		});

		await waitFor(() => expect(container.querySelector('.chart')).not.toBeNull());
		await waitFor(() => expect(heldBlockers()).toEqual([]));
	});

	it('holds an error-card blocker from a failed load until the error card is on screen', async () => {
		const seen: ReadyBlockerKind[][] = [];
		const unsubscribe = subscribeToBlockers(() => seen.push(heldBlockers()));
		let rejectImport!: (error: Error) => void;
		const importer = vi.fn(
			() =>
				new Promise((_, reject) => {
					rejectImport = reject;
				})
		);
		const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
		const { container } = renderComponent(importer, '/components/Broken-1.js');

		await act(async () => {
			rejectImport(new Error('boom'));
		});

		await waitFor(() => expect(container.querySelector('.deck-error-card')).not.toBeNull());
		await waitFor(() => expect(heldBlockers()).toEqual([]));
		unsubscribe();
		consoleError.mockRestore();
		expect(seen.some((kinds) => kinds.includes('error-card'))).toBe(true);
	});

	it('holds nothing for a component with a build error', () => {
		render(
			<DeckComponent
				source="slides/Broken.jsx"
				url=""
				buildError="slides/Broken.jsx:3:7: bad"
				props={{}}
				slots={{}}
				slide={makeSlide()}
				step={0}
				steps={0}
				active
				printMode={false}
			/>
		);
		expect(heldBlockers()).toEqual([]);
	});

	it('releases its blocker when it unmounts before the bundle loads', () => {
		const importer = vi.fn(() => new Promise(() => {}));
		const { unmount } = renderComponent(importer, '/components/Never-1.js');
		expect(heldBlockers()).toEqual(['component']);

		unmount();
		expect(heldBlockers()).toEqual([]);
	});
});
