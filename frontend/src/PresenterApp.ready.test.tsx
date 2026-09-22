/**
 * tap export pdf --content both captures /presenter?print=true, so the
 * presenter page reports the ready signal too.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/react';
import type { Presentation } from '$lib/types';

const presentation: Presentation = {
	config: { title: 'Ready Deck' },
	revision: 'r7',
	slides: [
		{
			index: 0,
			layout: 'default',
			html: '<p>One</p>',
			slots: { default: '<p>One</p>' },
			slotOrder: ['default'],
			fragmentCount: 0,
			steps: 1,
			hash: 'a'
		}
	]
};

describe('PresenterApp ready signal in print mode', () => {
	beforeEach(() => {
		vi.resetModules();
		window.history.pushState({}, '', '/presenter?print=true#1');
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
		vi.stubGlobal('navigator', { wakeLock: undefined });
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
		window.history.pushState({}, '', '/');
		delete (window as unknown as { __tapReady?: unknown }).__tapReady;
	});

	it('reports the revision, the slide and the final step', async () => {
		const { resetPresentation } = await import('$lib/stores/presentation');
		resetPresentation();
		const { default: PresenterApp } = await import('./PresenterApp');
		render(<PresenterApp />);

		await waitFor(() =>
			expect((window as unknown as { __tapReady?: unknown }).__tapReady).toEqual({
				revision: 'r7',
				slide: 1,
				step: 1
			})
		);
	});
});
