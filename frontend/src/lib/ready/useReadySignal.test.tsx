import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { resetBlockersForTests } from './blockers';
import { clearReady, type ReadyPayload } from './readySignal';
import { useReadySignal } from './useReadySignal';

// Probes whose rounds never settle: every wait ends at once, and the page
// always reports something still loading.
vi.mock('./probes', () => ({
	createDomProbes: () => ({
		fonts: () => Promise.resolve(),
		images: () => Promise.resolve(),
		animations: () => Promise.resolve(),
		paint: () => Promise.resolve(),
		settledNow: () => false
	})
}));

function readyValue(): ReadyPayload | null | undefined {
	return (window as unknown as { __tapReady?: ReadyPayload | null }).__tapReady;
}

function Page({ requirePaint }: { requirePaint: boolean }) {
	useReadySignal({
		enabled: true,
		revision: 'r1',
		slide: 1,
		step: 0,
		fragment: 0,
		theme: 'base',
		includeInfiniteAnimations: requirePaint,
		requirePaint
	});
	return null;
}

beforeEach(() => {
	resetBlockersForTests();
	clearReady();
});

afterEach(() => {
	cleanup();
});

describe('useReadySignal when the rounds never settle', () => {
	it('reports a live page ready with settled false', async () => {
		render(<Page requirePaint={false} />);

		await vi.waitFor(() => expect(readyValue()).toEqual({ revision: 'r1', slide: 1, step: 0, settled: false }));
	});

	it('never reports a print or capture page ready', async () => {
		render(<Page requirePaint={true} />);

		await vi.waitFor(() =>
			expect((window as unknown as { __tapReadyState: { phase: string } }).__tapReadyState.phase).toBe('gave-up')
		);
		expect(readyValue()).toBeNull();
	});
});
