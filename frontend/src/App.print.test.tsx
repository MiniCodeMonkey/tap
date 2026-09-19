/**
 * Print mode (`?print=true`) is a static PDF-export snapshot of one slide:
 * it must never open the websocket or have the hub's live state applied out
 * from under the screenshot (see internal/pdf/exporter.go, which loads
 * `<server>?print=true#<n>` for every slide). App.tsx reads its PRINT_MODE
 * flag from `window.location.search` once, at module load, so this needs
 * the URL set and the module imported fresh, in a file of its own.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/react';
import type { Presentation } from '$lib/types';

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

describe('App in print mode', () => {
	beforeEach(() => {
		vi.resetModules();
		window.history.pushState({}, '', '/?print=true#1');
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
		window.history.pushState({}, '', '/');
	});

	it('never connects the websocket', async () => {
		const websocketModule = await import('$lib/stores/websocket');
		const connectSpy = vi.spyOn(websocketModule, 'connectWebSocket');
		const { resetPresentation } = await import('$lib/stores/presentation');
		resetPresentation();

		const { default: App } = await import('./App');
		render(<App />);

		await waitFor(() => {
			expect(document.querySelector('.slide-content')).toBeInTheDocument();
		});

		expect(connectSpy).not.toHaveBeenCalled();
	});
});
