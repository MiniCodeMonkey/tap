import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { cleanup, render, waitFor } from '@testing-library/react';
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
});
