import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/react';
import PresenterApp from './PresenterApp';
import { resetPresentation, usePresentationStore } from '$lib/stores/presentation';
import type { Presentation } from '$lib/types';

vi.mock('$lib/stores/websocket', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/stores/websocket')>();
	return {
		...actual,
		connectWebSocket: vi.fn(),
		disconnectWebSocket: vi.fn(),
		getWebSocketClient: vi.fn(() => ({ send: vi.fn() }))
	};
});

const samplePresentation: Presentation = {
	config: { title: 'Test Deck' },
	slides: [
		{
			index: 0,
			layout: 'default',
			html: '<p>Slide one</p>',
			notes: '<p>Remember to breathe</p>',
			slots: { default: '<p>Slide one</p>' },
			slotOrder: ['default'],
			fragmentCount: 0,
			steps: 0
		},
		{
			index: 1,
			layout: 'default',
			html: '<p>Slide two</p>',
			slots: { default: '<p>Slide two</p>' },
			slotOrder: ['default'],
			fragmentCount: 0,
			steps: 0
		}
	]
};

describe('PresenterApp', () => {
	beforeEach(() => {
		resetPresentation();
		window.localStorage.clear();
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					statusText: 'OK',
					json: () => Promise.resolve(samplePresentation)
				} as Response)
			)
		);
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renders inside a slide-renderer root and shows the presenter view', async () => {
		const { container } = render(<PresenterApp />);
		expect(container.querySelector('.slide-renderer')).toBeInTheDocument();

		await waitFor(() => {
			expect(container.querySelector('.presenter-view')).toBeInTheDocument();
		});
	});

	it('sets data-print="true" on the next-slide panel\'s canvas, not just the current slide\'s, under ?print=true', async () => {
		// PRINT_MODE is a module-level constant read from window.location at
		// import time, so re-import the module fresh with the query string
		// already set, the way /presenter?print=true actually loads.
		window.history.replaceState(null, '', '/presenter?print=true');
		vi.resetModules();
		const presentationModule = await import('$lib/stores/presentation');
		const { default: PrintPresenterApp } = await import('./PresenterApp');
		presentationModule.resetPresentation();

		const { container } = render(<PrintPresenterApp />);

		await waitFor(() => {
			expect(container.querySelector('.presenter-next-slide-panel .slot-default')).toBeInTheDocument();
		});

		const currentCanvas = container.querySelector('.presenter-current-slide-panel .slide-container .slide');
		const nextCanvas = container.querySelector('.presenter-next-slide-panel .slide-container .slide');
		expect(currentCanvas).toHaveAttribute('data-print', 'true');
		expect(nextCanvas).toHaveAttribute('data-print', 'true');

		window.history.replaceState(null, '', '/');
	});

	it('renders both the current slide and the next slide preview', async () => {
		const { container } = render(<PresenterApp />);

		await waitFor(() => {
			expect(container.querySelector('.presenter-current-slide-panel .slot-default')).toBeInTheDocument();
		});

		expect(container.querySelector('.presenter-current-slide-panel')?.textContent).toContain('Slide one');
		expect(container.querySelector('.presenter-next-slide-panel')?.textContent).toContain('Slide two');
	});

	it('renders the current slide speaker notes', async () => {
		const { container } = render(<PresenterApp />);

		await waitFor(() => {
			expect(container.querySelector('.presenter-notes-content')).toHaveTextContent('Remember to breathe');
		});
	});

	it('keeps line breaks and blank lines between paragraphs in speaker notes', async () => {
		// Notes are plain text with line breaks preserved by the parser (see
		// internal/parser/notes.go), sent to the client as-is - literal "\n"
		// characters, not <br> or <p> tags. presenter-view.css's
		// .presenter-notes-content rule (white-space: pre-wrap) is what turns
		// those newlines into visible line breaks; jsdom cannot measure
		// layout, so this only asserts the text content keeps the newline
		// characters and the element carries the class that carries the rule.
		const multilineNotes: Presentation = {
			config: {},
			slides: [
				{
					index: 0,
					layout: 'default',
					html: '<p>Slide one</p>',
					notes: 'First line\nSecond line\n\nNext paragraph',
					slots: { default: '<p>Slide one</p>' },
					slotOrder: ['default'],
					fragmentCount: 0,
					steps: 0
				}
			]
		};
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					statusText: 'OK',
					json: () => Promise.resolve(multilineNotes)
				} as Response)
			)
		);

		const { container } = render(<PresenterApp />);

		let notesEl: Element | null = null;
		await waitFor(() => {
			notesEl = container.querySelector('.presenter-notes-content');
			expect(notesEl).toBeInTheDocument();
		});

		expect(notesEl!.classList.contains('presenter-notes-content')).toBe(true);
		expect(notesEl!.textContent).toBe('First line\nSecond line\n\nNext paragraph');
	});

	it('shows a placeholder for notes when the slide has none', async () => {
		const noNotes: Presentation = {
			config: {},
			slides: [samplePresentation.slides[1]]
		};
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					statusText: 'OK',
					json: () => Promise.resolve(noNotes)
				} as Response)
			)
		);

		const { container } = render(<PresenterApp />);

		await waitFor(() => {
			expect(container.querySelector('.presenter-no-notes')).toBeInTheDocument();
		});
	});

	it('shows the current slide panel in sync with the live fragment index', async () => {
		const withFragment: Presentation = {
			config: {},
			slides: [
				{
					index: 0,
					layout: 'default',
					html: '<p>Intro</p><p data-fragment-index="0">Revealed later</p>',
					slots: { default: '<p>Intro</p><p data-fragment-index="0">Revealed later</p>' },
					slotOrder: ['default'],
					fragmentCount: 1,
					steps: 0
				}
			]
		};
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					statusText: 'OK',
					json: () => Promise.resolve(withFragment)
				} as Response)
			)
		);

		const { container } = render(<PresenterApp />);

		await waitFor(() => {
			expect(container.querySelector('.presenter-current-slide-panel .slot-default')).toBeInTheDocument();
		});

		const fragment = () =>
			container
				.querySelector('.presenter-current-slide-panel')
				?.querySelector('[data-fragment-index="0"]');

		expect(fragment()).toHaveClass('fragment-hidden');
		expect(fragment()).not.toHaveClass('fragment-visible');

		act(() => {
			usePresentationStore.setState({ currentFragmentIndex: 0 });
		});

		expect(fragment()).toHaveClass('fragment-visible');
		expect(fragment()).not.toHaveClass('fragment-hidden');
	});

	it('applies the deck theme to both preview panels once its CSS has loaded', async () => {
		// Regression test: PresenterApp resolves the theme through
		// useResolvedTheme (which calls loadTheme) instead of setting
		// data-theme straight from themeOverride/config.theme, so a
		// non-base theme's CSS is always fetched before both panels apply
		// it.
		const themedPresentation: Presentation = {
			...samplePresentation,
			config: { ...samplePresentation.config, theme: 'terminal' }
		};
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					statusText: 'OK',
					json: () => Promise.resolve(themedPresentation)
				} as Response)
			)
		);

		const { container } = render(<PresenterApp />);

		await waitFor(() => {
			const currentCanvas = container.querySelector('.presenter-current-slide-panel .slide-container .slide');
			const nextCanvas = container.querySelector('.presenter-next-slide-panel .slide-container .slide');
			expect(currentCanvas).toHaveAttribute('data-theme', 'terminal');
			expect(nextCanvas).toHaveAttribute('data-theme', 'terminal');
		});
	});
	it('opens the shortcut overlay on ? and closes it on ? or Escape', async () => {
		const { container } = render(<PresenterApp />);
		await waitFor(() => expect(container.querySelector('.presenter-view')).toBeInTheDocument());

		fireEvent.keyDown(window, { key: '?' });
		const overlay = container.querySelector('.shortcut-help');
		expect(overlay).toBeInTheDocument();
		expect(overlay).toHaveTextContent('Reset the timer');
		expect(overlay).not.toHaveTextContent('Toggle the slide overview');

		fireEvent.keyDown(window, { key: '?' });
		expect(container.querySelector('.shortcut-help')).not.toBeInTheDocument();

		fireEvent.keyDown(window, { key: '?' });
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(container.querySelector('.shortcut-help')).not.toBeInTheDocument();
	});

	it('closes the shortcut overlay on a backdrop click and ignores navigation while open', async () => {
		const { container } = render(<PresenterApp />);
		await waitFor(() => expect(container.querySelector('.presenter-view')).toBeInTheDocument());

		fireEvent.keyDown(window, { key: '?' });
		fireEvent.keyDown(window, { key: 'ArrowRight' });
		expect(usePresentationStore.getState().currentSlideIndex).toBe(0);

		fireEvent.click(container.querySelector('.shortcut-help-backdrop')!);
		expect(container.querySelector('.shortcut-help')).not.toBeInTheDocument();
	});

	describe('speaker notes font size', () => {
		const storageKey = 'tap-presenter-notes-font-size';

		beforeEach(() => {
			window.localStorage.clear();
		});

		function notesFontSize(container: HTMLElement): string {
			return (container.querySelector('.presenter-notes-content') as HTMLElement).style.fontSize;
		}

		it('changes with the - and = keys and the A- / A+ buttons, and persists the size', async () => {
			const { container, getByLabelText } = render(<PresenterApp />);
			await waitFor(() => expect(container.querySelector('.presenter-notes-content')).toBeInTheDocument());
			expect(notesFontSize(container)).toBe('1.5rem');

			fireEvent.keyDown(window, { key: '=' });
			expect(notesFontSize(container)).toBe('1.625rem');
			expect(window.localStorage.getItem(storageKey)).toBe('1.625');

			fireEvent.keyDown(window, { key: '-' });
			fireEvent.keyDown(window, { key: '-' });
			expect(notesFontSize(container)).toBe('1.375rem');

			fireEvent.click(getByLabelText('Larger speaker notes'));
			expect(notesFontSize(container)).toBe('1.5rem');
			fireEvent.click(getByLabelText('Smaller speaker notes'));
			expect(notesFontSize(container)).toBe('1.375rem');
			expect(window.localStorage.getItem(storageKey)).toBe('1.375');
		});

		it('restores the stored size on load and clamps it to the allowed range', async () => {
			window.localStorage.setItem(storageKey, '2.25');
			const first = render(<PresenterApp />);
			await waitFor(() => expect(first.container.querySelector('.presenter-notes-content')).toBeInTheDocument());
			expect(notesFontSize(first.container)).toBe('2.25rem');
			first.unmount();

			window.localStorage.setItem(storageKey, '99');
			const second = render(<PresenterApp />);
			await waitFor(() => expect(second.container.querySelector('.presenter-notes-content')).toBeInTheDocument());
			expect(notesFontSize(second.container)).toBe('3rem');
			expect(second.getByLabelText('Larger speaker notes')).toBeDisabled();
		});

		it('falls back to the default size when storage throws', async () => {
			vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
				throw new Error('blocked');
			});
			vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
				throw new Error('blocked');
			});

			const { container } = render(<PresenterApp />);
			await waitFor(() => expect(container.querySelector('.presenter-notes-content')).toBeInTheDocument());
			expect(notesFontSize(container)).toBe('1.5rem');

			fireEvent.keyDown(window, { key: '=' });
			expect(notesFontSize(container)).toBe('1.625rem');
			vi.restoreAllMocks();
		});
	});
	describe('layouts', () => {
		it('opens in the standard layout and shows every panel', async () => {
			const { container } = render(<PresenterApp />);
			await waitFor(() => expect(container.querySelector('.presenter-view')).toBeInTheDocument());
			expect(container.querySelector('.presenter-view')?.getAttribute('data-presenter-layout')).toBe(
				'standard'
			);
			expect(container.querySelector('.presenter-current-slide-panel')).toBeInTheDocument();
			expect(container.querySelector('.presenter-next-slide-panel')).toBeInTheDocument();
			expect(container.querySelector('.presenter-notes-panel')).toBeInTheDocument();
		});

		it('cycles the layout with the V key', async () => {
			const { container } = render(<PresenterApp />);
			await waitFor(() => expect(container.querySelector('.presenter-view')).toBeInTheDocument());

			fireEvent.keyDown(window, { key: 'v' });

			await waitFor(() =>
				expect(
					container.querySelector('.presenter-view')?.getAttribute('data-presenter-layout')
				).toBe('notes-first')
			);
		});

		it('ignores digits while the layout menu is closed', async () => {
			const { container } = render(<PresenterApp />);
			await waitFor(() => expect(container.querySelector('.presenter-view')).toBeInTheDocument());

			fireEvent.keyDown(window, { key: '5' });

			expect(container.querySelector('.presenter-view')?.getAttribute('data-presenter-layout')).toBe(
				'standard'
			);
		});

		it('drops the notes and next panels in the slide-only layout', async () => {
			window.localStorage.setItem('tap-presenter-layout', 'slide-only');
			const { container } = render(<PresenterApp />);
			await waitFor(() =>
				expect(
					container.querySelector('.presenter-view')?.getAttribute('data-presenter-layout')
				).toBe('slide-only')
			);
			expect(container.querySelector('.presenter-current-slide-panel')).toBeInTheDocument();
			expect(container.querySelector('.presenter-next-slide-panel')).toBeNull();
			expect(container.querySelector('.presenter-notes-panel')).toBeNull();
		});

		it('drops the slide panels in the notes-only layout', async () => {
			window.localStorage.setItem('tap-presenter-layout', 'notes-only');
			const { container } = render(<PresenterApp />);
			await waitFor(() =>
				expect(
					container.querySelector('.presenter-view')?.getAttribute('data-presenter-layout')
				).toBe('notes-only')
			);
			expect(container.querySelector('.presenter-current-slide-panel')).toBeNull();
			expect(container.querySelector('.presenter-next-slide-panel')).toBeNull();
			expect(container.querySelector('.presenter-notes-panel')).toBeInTheDocument();
		});
	});
	describe('fitting the speaker notes', () => {
		function notesFontSize(container: HTMLElement): string {
			const notes = container.querySelector('.presenter-notes-content') as HTMLElement;
			return notes.style.fontSize;
		}

		it('lets fitting grow past the manual reading size', async () => {
			// jsdom lays nothing out, so the panel measures as zero height and
			// the hook falls back to its ceiling. That makes the ceiling
			// observable: it must be larger than the 3rem manual maximum.
			window.localStorage.setItem('tap-presenter-notes-size-mode', 'fit');
			const { container } = render(<PresenterApp />);
			await waitFor(() => expect(container.querySelector('.presenter-notes-content')).toBeInTheDocument());
			expect(notesFontSize(container)).toBe('6rem');
		});

		it('scales the fitted size with - and =, leaving the manual size alone', async () => {
			window.localStorage.setItem('tap-presenter-notes-size-mode', 'fit');
			const { container } = render(<PresenterApp />);
			await waitFor(() => expect(container.querySelector('.presenter-notes-content')).toBeInTheDocument());

			fireEvent.keyDown(window, { key: '-' });

			expect(notesFontSize(container)).toBe('5.4rem');
			expect(window.localStorage.getItem('tap-presenter-notes-fit-scale')).toBe('0.9');
			expect(window.localStorage.getItem('tap-presenter-notes-font-size')).toBeNull();
		});

		it('never scales above the fitted size, and stops at half', async () => {
			window.localStorage.setItem('tap-presenter-notes-size-mode', 'fit');
			const { container } = render(<PresenterApp />);
			await waitFor(() => expect(container.querySelector('.presenter-notes-content')).toBeInTheDocument());

			fireEvent.keyDown(window, { key: '=' });
			expect(notesFontSize(container)).toBe('6rem');

			for (let press = 0; press < 10; press++) {
				fireEvent.keyDown(window, { key: '-' });
			}
			expect(window.localStorage.getItem('tap-presenter-notes-fit-scale')).toBe('0.5');
			expect(notesFontSize(container)).toBe('3rem');
		});

		it('keeps using the manual size when fitting is off', async () => {
			const { container } = render(<PresenterApp />);
			await waitFor(() => expect(container.querySelector('.presenter-notes-content')).toBeInTheDocument());
			expect(notesFontSize(container)).toBe('1.5rem');
		});
	});

	it('previews the next presented slide and counts only presented slides', async () => {
		const withSkip: Presentation = {
			config: { title: 'Test Deck' },
			slides: ['one', 'two', 'three'].map((word, index) => ({
				index,
				layout: 'default',
				html: `<p>Slide ${word}</p>`,
				slots: { default: `<p>Slide ${word}</p>` },
				slotOrder: ['default'],
				fragmentCount: 0,
				steps: 0,
				skip: index === 1
			}))
		};
		// An earlier test's navigation can leave a hash in the URL, which
		// would open a skipped slide directly.
		window.history.replaceState(null, '', '/presenter');
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({ ok: true, statusText: 'OK', json: () => Promise.resolve(withSkip) } as Response)
			)
		);

		const { container } = render(<PresenterApp />);

		await waitFor(() => {
			expect(container.querySelector('.presenter-next-slide-panel')?.textContent).toContain('Slide three');
		});
		expect(container.querySelector('.presenter-slide-counter .total')?.textContent).toBe('2');
		expect(container.querySelector('.presenter-slide-counter .current')?.textContent).toBe('1');

		act(() => {
			usePresentationStore.setState({ currentSlideIndex: 1 });
		});
		expect(container.querySelector('.presenter-slide-counter .current')?.textContent).toBe('Skipped');
	});
});
