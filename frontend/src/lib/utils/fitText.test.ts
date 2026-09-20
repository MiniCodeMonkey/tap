import { describe, expect, it, vi } from 'vitest';
import { findFittingSize } from './fitText';

const BOUNDS = { minSize: 1, maxSize: 3, step: 0.125 };

describe('findFittingSize', () => {
	it('returns the cap when everything fits', () => {
		expect(findFittingSize({ ...BOUNDS, fits: () => true })).toBe(3);
	});

	it('returns the floor when nothing fits', () => {
		expect(findFittingSize({ ...BOUNDS, fits: () => false })).toBe(1);
	});

	it('returns the largest size that fits', () => {
		const fits = (size: number) => size <= 1.5 + 1e-9;
		expect(findFittingSize({ ...BOUNDS, fits })).toBeCloseTo(1.5, 5);
	});

	it('lands on a step boundary', () => {
		const fits = (size: number) => size <= 1.7;
		expect(findFittingSize({ ...BOUNDS, fits })).toBeCloseTo(1.625, 5);
	});

	it('measures a logarithmic number of times, not one per step', () => {
		const fits = vi.fn(() => false);
		findFittingSize({ ...BOUNDS, fits });
		expect(fits.mock.calls.length).toBeLessThanOrEqual(6);
	});

	it('returns the floor when the bounds are inverted or equal', () => {
		expect(findFittingSize({ minSize: 2, maxSize: 2, step: 0.125, fits: () => true })).toBe(2);
		expect(findFittingSize({ minSize: 2, maxSize: 1, step: 0.125, fits: () => true })).toBe(2);
	});
});
