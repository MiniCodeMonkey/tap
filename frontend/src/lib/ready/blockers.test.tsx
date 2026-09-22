import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import {
	heldBlockers,
	holdReady,
	resetBlockersForTests,
	subscribeToBlockers,
	useReadyHold,
	whenNoBlockers,
	type ReadyBlockerKind
} from './blockers';

afterEach(() => {
	cleanup();
	resetBlockersForTests();
});

describe('blockers', () => {
	it('lists a held blocker until it is released', () => {
		const release = holdReady('map');
		expect(heldBlockers()).toEqual(['map']);
		release();
		expect(heldBlockers()).toEqual([]);
	});

	it('releases once, however often the release function runs', () => {
		const releaseMap = holdReady('map');
		holdReady('component');
		releaseMap();
		releaseMap();
		expect(heldBlockers()).toEqual(['component']);
	});

	it('tells subscribers about every hold and release', () => {
		const seen: ReadyBlockerKind[][] = [];
		const unsubscribe = subscribeToBlockers(() => seen.push(heldBlockers()));
		const release = holdReady('fonts');
		release();
		unsubscribe();
		holdReady('images');
		expect(seen).toEqual([['fonts'], []]);
	});

	it('resolves whenNoBlockers once the last blocker is released', async () => {
		const releaseFirst = holdReady('component');
		const releaseSecond = holdReady('map');
		let resolved = false;
		const waiting = whenNoBlockers().then(() => {
			resolved = true;
		});

		releaseFirst();
		await Promise.resolve();
		expect(resolved).toBe(false);

		releaseSecond();
		await waiting;
		expect(resolved).toBe(true);
	});

	it('holds from a component while holding is true', () => {
		function Holder({ holding }: { holding: boolean }) {
			useReadyHold('animations', holding);
			return null;
		}
		const { rerender, unmount } = render(<Holder holding />);
		expect(heldBlockers()).toEqual(['animations']);

		rerender(<Holder holding={false} />);
		expect(heldBlockers()).toEqual([]);

		rerender(<Holder holding />);
		unmount();
		expect(heldBlockers()).toEqual([]);
	});
});
