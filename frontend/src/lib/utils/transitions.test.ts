import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	TRANSITION_DEFAULTS,
	TRANSITION_TYPES,
	getEffectiveDuration,
	getTransitionDescription,
	getTransitionVariants,
	isValidTransition,
	prefersReducedMotion,
	resolveTransition
} from './transitions';

function mockMatchMedia(matches: boolean): void {
	vi.mocked(window.matchMedia).mockImplementation((query: string) => ({
		matches,
		media: query,
		onchange: null,
		addListener: vi.fn(),
		removeListener: vi.fn(),
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		dispatchEvent: vi.fn()
	}));
}

afterEach(() => {
	vi.restoreAllMocks();
	window.history.replaceState(null, '', '/');
});

describe('isValidTransition', () => {
	it('accepts every known transition type', () => {
		for (const type of TRANSITION_TYPES) {
			expect(isValidTransition(type)).toBe(true);
		}
	});

	it('rejects an unknown value', () => {
		expect(isValidTransition('spin')).toBe(false);
	});
});

describe('resolveTransition', () => {
	it('prefers the per-slide transition over the global one', () => {
		expect(resolveTransition('zoom', 'slide')).toBe('zoom');
	});

	it('falls back to the global transition when the slide has none', () => {
		expect(resolveTransition(undefined, 'push')).toBe('push');
	});

	it('falls back to the default transition when neither is set', () => {
		expect(resolveTransition(undefined, undefined)).toBe(TRANSITION_DEFAULTS.defaultTransition);
	});

	it('ignores an invalid slide transition and falls back to global', () => {
		expect(resolveTransition('bogus' as never, 'zoom')).toBe('zoom');
	});
});

describe('getEffectiveDuration', () => {
	it('returns the given duration by default', () => {
		mockMatchMedia(false);
		expect(getEffectiveDuration(400)).toBe(400);
	});

	it('returns 0 when the user prefers reduced motion', () => {
		mockMatchMedia(true);
		expect(getEffectiveDuration(400)).toBe(0);
	});

	it('returns 0 in print mode', () => {
		mockMatchMedia(false);
		window.history.replaceState(null, '', '/?print=true');
		expect(getEffectiveDuration(400)).toBe(0);
	});
});

describe('prefersReducedMotion', () => {
	it('reflects the media query result', () => {
		mockMatchMedia(true);
		expect(prefersReducedMotion()).toBe(true);

		mockMatchMedia(false);
		expect(prefersReducedMotion()).toBe(false);
	});
});

describe('getTransitionDescription', () => {
	it('describes every known transition type', () => {
		for (const type of TRANSITION_TYPES) {
			expect(getTransitionDescription(type)).not.toBe('Unknown transition');
		}
	});
});

describe('getTransitionVariants', () => {
	const CUBIC_OUT = [0.33, 1, 0.68, 1];

	it('returns empty variants for none', () => {
		expect(getTransitionVariants('none', 'forward')).toEqual({
			initial: {},
			animate: {},
			exit: {},
			ease: 'linear'
		});
	});

	it('fades between 0 and 1 opacity regardless of direction', () => {
		const forward = getTransitionVariants('fade', 'forward');
		const backward = getTransitionVariants('fade', 'backward');

		expect(forward).toEqual({
			initial: { opacity: 0 },
			animate: { opacity: 1 },
			exit: { opacity: 0 },
			ease: 'linear'
		});
		expect(backward).toEqual(forward);
	});

	it('mirrors the slide x offset for backward', () => {
		const forward = getTransitionVariants('slide', 'forward');
		const backward = getTransitionVariants('slide', 'backward');

		expect(forward).toEqual({ initial: { x: 100 }, animate: { x: 0 }, exit: { x: 100 }, ease: CUBIC_OUT });
		expect(backward).toEqual({ initial: { x: -100 }, animate: { x: 0 }, exit: { x: -100 }, ease: CUBIC_OUT });
	});

	it('mirrors the push x offset for backward and dips opacity', () => {
		const forward = getTransitionVariants('push', 'forward');
		const backward = getTransitionVariants('push', 'backward');

		expect(forward).toEqual({
			initial: { x: 50, opacity: 0.5 },
			animate: { x: 0, opacity: 1 },
			exit: { x: 50, opacity: 0.5 },
			ease: CUBIC_OUT
		});
		expect(backward).toEqual({
			initial: { x: -50, opacity: 0.5 },
			animate: { x: 0, opacity: 1 },
			exit: { x: -50, opacity: 0.5 },
			ease: CUBIC_OUT
		});
	});

	it('mirrors the zoom start scale for backward', () => {
		const forward = getTransitionVariants('zoom', 'forward');
		const backward = getTransitionVariants('zoom', 'backward');

		expect(forward).toEqual({ initial: { scale: 0.8 }, animate: { scale: 1 }, exit: { scale: 0.8 }, ease: CUBIC_OUT });
		expect(backward).toEqual({ initial: { scale: 1.2 }, animate: { scale: 1 }, exit: { scale: 1.2 }, ease: CUBIC_OUT });
	});

	it('uses linear easing for fade and none, cubic-out for the directional transitions', () => {
		expect(getTransitionVariants('fade', 'forward').ease).toBe('linear');
		expect(getTransitionVariants('none', 'forward').ease).toBe('linear');
		expect(getTransitionVariants('slide', 'forward').ease).toEqual(CUBIC_OUT);
		expect(getTransitionVariants('push', 'forward').ease).toEqual(CUBIC_OUT);
		expect(getTransitionVariants('zoom', 'forward').ease).toEqual(CUBIC_OUT);
	});
});
