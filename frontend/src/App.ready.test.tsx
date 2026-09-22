/**
 * The audience page reports {revision, slide, step} once the slide on
 * screen has settled, and resets when the slide or the deck changes. Print
 * mode is read once at module load, so the URL is set and App imported
 * fresh for each test.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render, waitFor } from '@testing-library/react';
import type { Presentation } from '$lib/types';

const presentation: Presentation = {
	config: { title: 'Ready Deck' },
	revision: 'r1',
	slides: [
		{
			index: 0,
			layout: 'default',
			html: '<p>One</p>',
			slots: { default: '<p>One</p>' },
			slotOrder: ['default'],
			fragmentCount: 0,
			steps: 2,
			hash: 'a'
		},
		{
			index: 1,
			layout: 'default',
			html: '<p>Two</p>',
			slots: { default: '<p>Two</p>' },
			slotOrder: ['default'],
			fragmentCount: 0,
			steps: 0,
			hash: 'b'
		}
	]
};

interface ReadyWindow {
	__tapReady?: unknown;
	webkit?: unknown;
}

function readyValue(): unknown {
	return (window as unknown as ReadyWindow).__tapReady;
}

async function renderApp(): Promise<typeof import('$lib/stores/presentation')> {
	const store = await import('$lib/stores/presentation');
	store.resetPresentation();
	const { default: App } = await import('./App');
	render(<App />);
	return store;
}

describe('App ready signal in print mode', () => {
	beforeEach(() => {
		vi.resetModules();
		window.history.pushState({}, '', '/?print=true#1');
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					statusText: 'OK',
					json: () => Promise.resolve(presentation)
				} as Response)
			)
		);
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
		window.history.pushState({}, '', '/');
		delete (window as unknown as ReadyWindow).__tapReady;
		delete (window as unknown as ReadyWindow).webkit;
	});

	it('reports the revision, the 1-based slide and the final step', async () => {
		await renderApp();
		await waitFor(() => expect(readyValue()).toEqual({ revision: 'r1', slide: 1, step: 2 }));
	});

	it('posts the same payload to the tapReady message handler', async () => {
		const postMessage = vi.fn();
		(window as unknown as ReadyWindow).webkit = { messageHandlers: { tapReady: { postMessage } } };
		await renderApp();
		await waitFor(() => expect(postMessage).toHaveBeenCalledWith({ revision: 'r1', slide: 1, step: 2 }));
	});

	it('resets on navigation and reports the new slide', async () => {
		await renderApp();
		await waitFor(() => expect(readyValue()).toEqual({ revision: 'r1', slide: 1, step: 2 }));

		act(() => {
			window.history.pushState({}, '', '/?print=true#2');
			window.dispatchEvent(new HashChangeEvent('hashchange'));
		});
		expect(readyValue()).toBeNull();

		await waitFor(() => expect(readyValue()).toEqual({ revision: 'r1', slide: 2, step: 0 }));
	});

	it('resets after an in-place update and reports the new revision', async () => {
		const store = await renderApp();
		await waitFor(() => expect(readyValue()).toEqual({ revision: 'r1', slide: 1, step: 2 }));

		act(() => {
			store.updatePresentationInPlace({ ...presentation, revision: 'r2' });
		});
		expect(readyValue()).toBeNull();

		await waitFor(() => expect(readyValue()).toEqual({ revision: 'r2', slide: 1, step: 2 }));
	});
});
