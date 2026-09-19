import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/react';
import App from './App';
import { resetPresentation } from '$lib/stores/presentation';
import type { Presentation } from '$lib/types';

vi.mock('$lib/stores/websocket', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/stores/websocket')>();
	return {
		...actual,
		connectWebSocket: vi.fn(),
		detectStaticMode: vi.fn(async () => false),
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
			html: '<p>Hello</p>',
			slots: { default: '<p>Hello</p>' },
			slotOrder: ['default'],
			fragmentCount: 0,
			steps: 0
		}
	]
};

describe('App', () => {
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

	it('renders inside a slide-renderer root', async () => {
		const { container } = render(<App />);
		expect(container.querySelector('.slide-renderer')).toBeInTheDocument();

		// Let the presentation fetch and its state update settle before the test ends.
		await waitFor(() => {
			expect(document.querySelector('.slide-content')).toBeInTheDocument();
		});
	});

	it('loads the presentation and renders the current slide', async () => {
		render(<App />);

		await waitFor(() => {
			expect(document.querySelector('.slide-content')).toBeInTheDocument();
		});

		expect(document.querySelector('.slot-default')?.innerHTML).toContain('Hello');
	});

	it('applies the deck theme to data-theme once its CSS has loaded', async () => {
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

		render(<App />);

		await waitFor(() => {
			expect(document.querySelector('[data-theme="terminal"]')).toBeInTheDocument();
		});
	});
});
