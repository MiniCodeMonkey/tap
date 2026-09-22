import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/react';
import { Slide } from './Slide';
import { resolveLayout } from '../layouts/registry';
import type { Slide as SlideData } from '$lib/types';
import { loadPresentation, resetPresentation } from '$lib/stores/presentation';
import { useConnectionStore } from '$lib/stores/websocket';

vi.mock('../layouts/registry', async (importOriginal) => {
	const actual = await importOriginal<typeof import('../layouts/registry')>();
	return { ...actual, resolveLayout: vi.fn(actual.resolveLayout) };
});

const deckComponentSpy = vi.fn((props: { source: string }) => (
	<p data-testid={`deck-component-${props.source}`}>{props.source}</p>
));
vi.mock('./DeckComponent', () => ({
	DeckComponent: (props: Record<string, unknown>) => deckComponentSpy(props as { source: string })
}));

const actualRegistry = await vi.importActual<typeof import('../layouts/registry')>('../layouts/registry');

afterEach(() => {
	cleanup();
	// React retries a failed render synchronously, calling resolveLayout a second
	// time, so a throwing test must keep returning the throwing layout for the
	// whole test and reset back to the real implementation afterward.
	vi.mocked(resolveLayout).mockImplementation(actualRegistry.resolveLayout);
	deckComponentSpy.mockClear();
});

function makeSlide(overrides: Partial<SlideData> = {}): SlideData {
	return {
		index: 0,
		layout: 'default',
		html: '',
		slots: { default: '<p>Hello</p>' },
		slotOrder: ['default'],
		fragmentCount: 0,
		steps: 0,
		...overrides
	};
}

describe('Slide', () => {
	it('renders both declared slots for a multi-slot layout', () => {
		const slide = makeSlide({
			layout: 'big-stat',
			slots: { default: '<h1>99%</h1>', caption: '<p>uptime</p>' },
			slotOrder: ['default', 'caption']
		});

		const { container } = render(
			<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={5} />
		);

		expect(container.querySelector('.slot-default')?.innerHTML).toContain('99%');
		expect(container.querySelector('.slot-caption')?.innerHTML).toContain('uptime');
	});

	it('renders an unregistered slot name after the declared ones', () => {
		const slide = makeSlide({
			layout: 'default',
			slots: { default: '<p>main</p>', extra: '<p>bonus</p>' },
			slotOrder: ['default', 'extra']
		});

		const { container } = render(
			<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
		);

		const slotElements = Array.from(container.querySelectorAll('.slot'));
		const defaultIndex = slotElements.findIndex((el) => el.classList.contains('slot-default'));
		const extraIndex = slotElements.findIndex((el) => el.classList.contains('slot-extra'));

		expect(defaultIndex).toBeGreaterThanOrEqual(0);
		expect(extraIndex).toBeGreaterThan(defaultIndex);
		expect(container.querySelector('.slot-extra')?.innerHTML).toContain('bonus');
	});

	it('sets data-layout, data-index and data-total on the slide root', () => {
		const slide = makeSlide({ index: 2, layout: 'quote' });

		const { container } = render(
			<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={5} />
		);

		const root = container.querySelector('.slide');
		expect(root).toHaveAttribute('data-layout', 'quote');
		expect(root).toHaveAttribute('data-index', '3');
		expect(root).toHaveAttribute('data-total', '5');
	});

	it('sets data-slide-numbers="off" only when the deck sets slideNumbers: false', () => {
		const slide = makeSlide();
		function slideNumbersAttribute(): string | null {
			const { container, unmount } = render(
				<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
			);
			const value = container.querySelector('.slide')!.getAttribute('data-slide-numbers');
			unmount();
			return value;
		}

		try {
			resetPresentation();
			expect(slideNumbersAttribute()).toBeNull();

			loadPresentation({ config: { slideNumbers: true }, slides: [slide] });
			expect(slideNumbersAttribute()).toBeNull();

			loadPresentation({ config: { slideNumbers: false }, slides: [slide] });
			expect(slideNumbersAttribute()).toBe('off');
		} finally {
			resetPresentation();
		}
	});

	it('sets data-deck-title from the deck title, and leaves it out when there is none', () => {
		const slide = makeSlide();
		function deckTitleAttribute(): string | null {
			const { container, unmount } = render(
				<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
			);
			const value = container.querySelector('.slide')!.getAttribute('data-deck-title');
			unmount();
			return value;
		}

		try {
			resetPresentation();
			expect(deckTitleAttribute()).toBeNull();

			loadPresentation({ config: { title: '  ' }, slides: [slide] });
			expect(deckTitleAttribute()).toBeNull();

			loadPresentation({ config: { title: 'Quarterly Review' }, slides: [slide] });
			expect(deckTitleAttribute()).toBe('Quarterly Review');
		} finally {
			resetPresentation();
		}
	});

	it('keeps slide-content as a plain wrapper with no data attributes', () => {
		const slide = makeSlide({ index: 2, layout: 'quote' });

		const { container } = render(
			<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={5} />
		);

		const content = container.querySelector('.slide-content');
		expect(content).toBeInTheDocument();
		expect(content).not.toHaveAttribute('data-layout');
	});

	it('does not wrap content in scroll-content when the slide has no scroll directive', () => {
		const slide = makeSlide();

		const { container } = render(
			<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
		);

		expect(container.querySelector('.scroll-content')).not.toBeInTheDocument();
	});

	it('wraps content in scroll-content when the slide has scroll: true', () => {
		const slide = makeSlide({ scroll: true });

		const { container } = render(
			<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
		);

		expect(container.querySelector('.scroll-content')).toBeInTheDocument();
	});

	it('strips a raw map code fence when rendering as a preview', () => {
		const slide = makeSlide({
			slots: {
				default: '<p>Map slide</p><pre><code class="language-map">start: [0, 0]\nend: [1, 1]</code></pre>'
			}
		});

		const { container } = render(
			<Slide slide={slide} active={false} printMode={false} preview fragmentIndex={-1} step={0} total={1} />
		);

		expect(container.querySelector('pre code.language-map')).not.toBeInTheDocument();
		expect(container.textContent).toContain('Map slide');
	});

	it('keeps a raw map code fence when not rendering as a preview', () => {
		const slide = makeSlide({
			slots: {
				default: '<p>Map slide</p><pre><code class="language-map">start: [0, 0]\nend: [1, 1]</code></pre>'
			}
		});

		const { container } = render(
			<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
		);

		expect(container.querySelector('pre code.language-map')).toBeInTheDocument();
	});

	it('renders the tag and badge as their own elements', () => {
		const slide = makeSlide({ tag: '// workshop', badge: 'v2.0' });

		const { container } = render(
			<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
		);

		expect(container.querySelector('.slide-tag')).toHaveTextContent('// workshop');
		expect(container.querySelector('.slide-badge')).toHaveTextContent('v2.0');
	});

	it('omits the tag and badge elements when the slide has none', () => {
		const slide = makeSlide();

		const { container } = render(
			<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
		);

		expect(container.querySelector('.slide-tag')).not.toBeInTheDocument();
		expect(container.querySelector('.slide-badge')).not.toBeInTheDocument();
	});

	it('shows the error card in dev mode when the layout throws', () => {
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.mocked(resolveLayout).mockReturnValue({
			component: () => {
				throw new Error('boom');
			},
			slots: ['default']
		});

		const slide = makeSlide({ index: 2 });
		const { container } = render(
			<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
		);

		expect(container.textContent).toContain('Slide 3 failed to render');
		consoleSpy.mockRestore();
	});

	it('shows the audience-safe marker instead of the full card when the page is opened with ?present=true', () => {
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.mocked(resolveLayout).mockReturnValue({
			component: () => {
				throw new Error('boom');
			},
			slots: ['default']
		});
		window.history.pushState({}, '', '/?present=true');

		const slide = makeSlide({ index: 2 });
		try {
			const { container } = render(
				<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
			);

			const card = container.querySelector('.deck-error-card');
			expect(card?.hasAttribute('hidden')).toBe(true);
			expect(card?.getAttribute('data-message')).toBe('Slide 3 failed to render');
			expect(container.querySelector('.deck-error-marker')?.textContent).toBe('component error');
		} finally {
			window.history.pushState({}, '', '/');
			consoleSpy.mockRestore();
		}
	});

	it('shows the raw slot content when not in dev mode', () => {
		// "Not in dev mode" is a static tap build output, not merely
		// import.meta.env.DEV === false: SlideErrorBoundary uses isDevRuntime
		// (see $lib/utils/runtime), which also checks for the
		// #presentation-data element a static build's index.html embeds (see
		// internal/builder/builder.go). Without it, this render would still
		// count as a live server (tap dev, tap export pdf, tap export images) and show
		// the error card, per the ruling that a broken slide must be visible
		// as broken everywhere except a static build.
		const originalDev = import.meta.env.DEV;
		(import.meta.env as { DEV: boolean }).DEV = false;
		const presentationDataScript = document.createElement('script');
		presentationDataScript.id = 'presentation-data';
		presentationDataScript.type = 'application/json';
		presentationDataScript.textContent = '{}';
		document.body.appendChild(presentationDataScript);
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.mocked(resolveLayout).mockReturnValue({
			component: () => {
				throw new Error('boom');
			},
			slots: ['default']
		});

		const slide = makeSlide({ slots: { default: '<p>raw fallback</p>' }, slotOrder: ['default'] });

		try {
			const { container } = render(
				<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
			);
			expect(container.innerHTML).toContain('raw fallback');
			expect(container.textContent).not.toContain('failed to render');
		} finally {
			consoleSpy.mockRestore();
			(import.meta.env as { DEV: boolean }).DEV = originalDev;
			presentationDataScript.remove();
		}
	});

	it('a settled capture (settleComponents) makes a whole-slide component print-mode too, at the REQUESTED step - not the slide total', async () => {
		const slide = makeSlide({
			layout: 'component',
			steps: 5,
			component: { source: 'slides/Deploy.jsx', url: '/components/Deploy-1.js' }
		});

		render(
			<Slide slide={slide} active printMode={false} settleComponents fragmentIndex={-1} step={2} total={1} />
		);

		// Rich-block processing (see useRichBlocks) can commit an extra,
		// props-identical re-render of the slide after this mock's first
		// call, independent of settleComponents; assert on the latest call
		// rather than an exact count.
		await waitFor(() => expect(deckComponentSpy).toHaveBeenCalled());
		const lastCall = deckComponentSpy.mock.calls[deckComponentSpy.mock.calls.length - 1];
		const props = lastCall[0] as unknown as { step: number; printMode: boolean };
		expect(props.step).toBe(2);
		expect(props.printMode).toBe(true);
	});

	describe('inline deck components', () => {
		it('mounts a portal into each placeholder, matched to slide.components by index', async () => {
			const slide = makeSlide({
				slots: {
					default:
						'<p>before</p><div class="deck-component" data-component-index="0"></div><p>between</p><div class="deck-component" data-component-index="1"></div>'
				},
				components: [
					{ index: 0, source: 'components/Chart.jsx', url: '/components/Chart-1.js', props: {} },
					{ index: 1, source: 'components/Stat.jsx', url: '/components/Stat-1.js', props: { value: 42 } }
				]
			});

			const { container } = render(
				<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />
			);

			await waitFor(() => expect(deckComponentSpy).toHaveBeenCalledTimes(2));

			const placeholders = container.querySelectorAll('.deck-component');
			expect(placeholders[0].textContent).toBe('components/Chart.jsx');
			expect(placeholders[1].textContent).toBe('components/Stat.jsx');

			const secondCallProps = deckComponentSpy.mock.calls.find((call) => call[0].source === 'components/Stat.jsx')?.[0];
			expect(secondCallProps?.props).toEqual({ value: 42 });
			expect(secondCallProps?.slots).toEqual({});
		});

		it('renders nothing for a placeholder index with no matching slide.components entry', () => {
			const slide = makeSlide({
				slots: { default: '<div class="deck-component" data-component-index="0"></div>' },
				components: []
			});

			render(<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={1} />);

			expect(deckComponentSpy).not.toHaveBeenCalled();
		});

		it('mounts in a preview render (active: false, printMode: false from the caller), at the slide total step and printMode: true', async () => {
			const slide = makeSlide({
				steps: 3,
				slots: { default: '<div class="deck-component" data-component-index="0"></div>' },
				components: [{ index: 0, source: 'components/Chart.jsx', url: '/components/Chart-1.js', props: {} }]
			});

			render(
				<Slide
					slide={slide}
					active={false}
					printMode={false}
					preview
					fragmentIndex={-1}
					step={0}
					total={1}
				/>
			);

			await waitFor(() => expect(deckComponentSpy).toHaveBeenCalledTimes(1));
			const props = deckComponentSpy.mock.calls[0][0] as unknown as {
				step: number;
				steps: number;
				active: boolean;
				printMode: boolean;
				preview: boolean;
			};
			expect(props.step).toBe(3);
			expect(props.active).toBe(false);
			expect(props.printMode).toBe(true);
			expect(props.preview).toBe(true);
		});

		it('a settled capture (settleComponents) makes an inline component print-mode too, at the REQUESTED step - not the slide total', async () => {
			// A settled screenshot capture settles a deck component's own
			// animation, but must not also force it to the slide's final
			// step the way real print mode does - it wants the step the
			// capture actually asked for.
			const slide = makeSlide({
				steps: 5,
				slots: { default: '<div class="deck-component" data-component-index="0"></div>' },
				components: [{ index: 0, source: 'components/Chart.jsx', url: '/components/Chart-1.js', props: {} }]
			});

			render(
				<Slide
					slide={slide}
					active
					printMode={false}
					settleComponents
					fragmentIndex={-1}
					step={2}
					total={1}
				/>
			);

			await waitFor(() => expect(deckComponentSpy).toHaveBeenCalledTimes(1));
			const props = deckComponentSpy.mock.calls[0][0] as unknown as { step: number; printMode: boolean };
			expect(props.step).toBe(2);
			expect(props.printMode).toBe(true);
		});
	});

	describe('skipped slides', () => {
		afterEach(() => {
			resetPresentation();
			useConnectionStore.setState({ presentMode: false });
		});

		const slides = [makeSlide({ index: 0 }), makeSlide({ index: 1, skip: true }), makeSlide({ index: 2 })];

		it('numbers a slide among the slides that are not skipped', () => {
			loadPresentation({ config: {}, slides });
			const { container } = render(
				<Slide slide={slides[2]} active printMode={false} fragmentIndex={-1} step={0} total={2} />
			);
			expect(container.querySelector('.slide')?.getAttribute('data-index')).toBe('2');
		});

		it('marks a skipped slide opened directly and hides its number', () => {
			loadPresentation({ config: {}, slides });
			const { container } = render(
				<Slide slide={slides[1]} active printMode={false} fragmentIndex={-1} step={0} total={2} />
			);
			const root = container.querySelector('.slide');
			expect(root?.getAttribute('data-skipped')).toBe('true');
			expect(root?.getAttribute('data-slide-numbers')).toBe('off');
			expect(container.querySelector('.slide-skipped-marker')?.textContent).toBe('Skipped');
		});

		it('leaves the marker out in print mode', () => {
			loadPresentation({ config: {}, slides });
			const { container } = render(<Slide slide={slides[1]} active printMode fragmentIndex={0} step={0} total={2} />);
			expect(container.querySelector('.slide-skipped-marker')).toBeNull();
		});

		it('leaves the marker out during tap present', () => {
			useConnectionStore.setState({ presentMode: true });
			loadPresentation({ config: {}, slides });
			const { container } = render(
				<Slide slide={slides[1]} active printMode={false} fragmentIndex={-1} step={0} total={2} />
			);
			expect(container.querySelector('.slide-skipped-marker')).toBeNull();
		});
	});
});
