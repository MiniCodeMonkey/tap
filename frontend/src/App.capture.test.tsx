/**
 * Capture mode (`?capture=true`) is a stepped or fragment screenshot (tap
 * export images --step/--fragment): it renders the live, non-print viewer at an
 * exact presenter state, against a temporary server that never serves the
 * websocket route (see internal/cli/export_images.go and buildSlideURL in
 * internal/pdf/capture.go). Like print mode, it must never open the
 * websocket or show the connection badge - otherwise a "Reconnecting..."
 * badge gets baked into the PNG. App.tsx reads its CAPTURE_MODE flag from
 * `window.location.search` once, at module load, so this needs the URL set
 * and the module imported fresh, in a file of its own.
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

describe('App in capture mode', () => {
	beforeEach(() => {
		vi.resetModules();
		window.history.pushState({}, '', '/?step=2&capture=true#1');
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

	it('never renders the connection indicator badge', async () => {
		const { resetPresentation } = await import('$lib/stores/presentation');
		resetPresentation();

		const { default: App } = await import('./App');
		render(<App />);

		await waitFor(() => {
			expect(document.querySelector('.slide-content')).toBeInTheDocument();
		});

		expect(document.querySelector('.connection-indicator')).not.toBeInTheDocument();
	});
});

describe('App settling a capture: a stepped capture must not land mid-animation', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
		window.history.pushState({}, '', '/');
	});

	async function renderAppAt(url: string): Promise<void> {
		vi.resetModules();
		window.history.pushState({}, '', url);
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
		const { resetPresentation } = await import('$lib/stores/presentation');
		resetPresentation();
		const { default: App } = await import('./App');
		render(<App />);
		await waitFor(() => {
			expect(document.querySelector('.slide-content')).toBeInTheDocument();
		});
	}

	it('sets data-print on the canvas frame for a capture without --wait, so theme CSS animations settle too', async () => {
		await renderAppAt('/?step=2&capture=true#1');

		const frame = document.querySelector('.slide-container > .slide');
		expect(frame?.getAttribute('data-print')).toBe('true');
	});

	it('does not set data-print for a live capture (--wait, ?live=true): the point is to catch it still running', async () => {
		await renderAppAt('/?step=2&capture=true&live=true#1');

		const frame = document.querySelector('.slide-container > .slide');
		expect(frame?.getAttribute('data-print')).toBeNull();
	});
});
