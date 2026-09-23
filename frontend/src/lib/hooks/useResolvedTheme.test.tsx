import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import { heldBlockers, resetBlockersForTests } from '$lib/ready/blockers';
import { loadTheme, type ThemeDefinition } from '$lib/themes/loader';
import { resetPresentation } from '$lib/stores/presentation';
import { useResolvedTheme } from './useResolvedTheme';

vi.mock('$lib/themes/loader', () => ({ loadTheme: vi.fn(), listThemes: vi.fn(() => []) }));

beforeEach(() => {
	resetBlockersForTests();
	resetPresentation();
});
afterEach(() => cleanup());

describe('useResolvedTheme and the ready signal', () => {
	it('holds a fonts blocker until the requested theme has loaded', async () => {
		let finish!: (definition: ThemeDefinition) => void;
		vi.mocked(loadTheme).mockReturnValue(
			new Promise<ThemeDefinition>((resolve) => {
				finish = resolve;
			})
		);
		const { result } = renderHook(() => useResolvedTheme());
		expect(heldBlockers()).toEqual(['fonts']);

		await act(async () => {
			finish({ slug: 'base' } as unknown as ThemeDefinition);
		});

		expect(heldBlockers()).toEqual([]);
		expect(result.current?.slug).toBe('base');
	});
});
