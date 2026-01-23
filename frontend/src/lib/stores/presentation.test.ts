/**
 * Unit tests for the presentation store.
 * These tests will run once Vitest is configured (US-075).
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import {
	presentation,
	currentSlideIndex,
	currentFragmentIndex,
	currentSlide,
	totalSlides,
	totalFragments,
	nextSlide,
	prevSlide,
	goToSlide,
	nextFragment,
	prevFragment,
	resetPresentation,
	loadPresentation,
	initializeFromURL,
	setupHashChangeListener
} from './presentation';
import type { Presentation } from '$lib/types';

// Mock window object for URL hash tests
const mockWindow = {
	location: { hash: '' },
	history: {
		replaceState: vi.fn()
	},
	addEventListener: vi.fn(),
	removeEventListener: vi.fn()
};

// Helper to create a test presentation
function createTestPresentation(slideCount: number, fragmentsPerSlide: number[] = []): Presentation {
	return {
		config: {
			title: 'Test Presentation',
			theme: 'minimal',
			transition: 'fade'
		},
		slides: Array.from({ length: slideCount }, (_, i) => ({
			index: i,
			layout: 'default' as const,
			html: `<p>Slide ${i + 1}</p>`,
			fragments:
				fragmentsPerSlide[i] !== undefined
					? Array.from({ length: fragmentsPerSlide[i] }, (_, j) => ({
							index: j,
							content: `Fragment ${j + 1}`
						}))
					: undefined
		}))
	};
}

describe('presentation store', () => {
	beforeEach(() => {
		// Reset stores before each test
		resetPresentation();

		// Reset window mock
		mockWindow.location.hash = '';
		mockWindow.history.replaceState.mockClear();
		mockWindow.addEventListener.mockClear();
		mockWindow.removeEventListener.mockClear();

		// Mock global window
		vi.stubGlobal('window', mockWindow);
	});

	describe('writable stores', () => {
		it('should have null presentation initially', () => {
			expect(get(presentation)).toBeNull();
		});

		it('should have slide index 0 initially', () => {
			expect(get(currentSlideIndex)).toBe(0);
		});

		it('should have fragment index -1 initially', () => {
			expect(get(currentFragmentIndex)).toBe(-1);
		});

		it('should update presentation store', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);
			expect(get(presentation)).toEqual(testPresentation);
		});
	});

	describe('derived stores', () => {
		it('currentSlide should return null when no presentation', () => {
			expect(get(currentSlide)).toBeNull();
		});

		it('currentSlide should return first slide initially', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);
			expect(get(currentSlide)).toEqual(testPresentation.slides[0]);
		});

		it('currentSlide should update when slideIndex changes', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);
			currentSlideIndex.set(2);
			expect(get(currentSlide)).toEqual(testPresentation.slides[2]);
		});

		it('totalSlides should return 0 when no presentation', () => {
			expect(get(totalSlides)).toBe(0);
		});

		it('totalSlides should return slide count', () => {
			const testPresentation = createTestPresentation(5);
			presentation.set(testPresentation);
			expect(get(totalSlides)).toBe(5);
		});

		it('totalFragments should return 0 when no fragments', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);
			expect(get(totalFragments)).toBe(0);
		});

		it('totalFragments should return fragment count for current slide', () => {
			const testPresentation = createTestPresentation(3, [2, 4, 1]);
			presentation.set(testPresentation);
			expect(get(totalFragments)).toBe(2);

			currentSlideIndex.set(1);
			expect(get(totalFragments)).toBe(4);
		});
	});

	describe('nextSlide', () => {
		it('should advance to next slide', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);

			const result = nextSlide();
			expect(result).toBe(true);
			expect(get(currentSlideIndex)).toBe(1);
		});

		it('should not advance past last slide', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);
			currentSlideIndex.set(2);

			const result = nextSlide();
			expect(result).toBe(false);
			expect(get(currentSlideIndex)).toBe(2);
		});

		it('should reveal fragment before advancing slide', () => {
			const testPresentation = createTestPresentation(3, [2, 0, 0]);
			presentation.set(testPresentation);

			// First call reveals first fragment
			nextSlide();
			expect(get(currentSlideIndex)).toBe(0);
			expect(get(currentFragmentIndex)).toBe(0);

			// Second call reveals second fragment
			nextSlide();
			expect(get(currentSlideIndex)).toBe(0);
			expect(get(currentFragmentIndex)).toBe(1);

			// Third call advances to next slide
			nextSlide();
			expect(get(currentSlideIndex)).toBe(1);
			expect(get(currentFragmentIndex)).toBe(-1);
		});

		it('should update URL hash when advancing', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);

			nextSlide();
			expect(mockWindow.history.replaceState).toHaveBeenCalledWith(null, '', '#2');
		});
	});

	describe('prevSlide', () => {
		it('should go to previous slide', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);
			currentSlideIndex.set(2);

			const result = prevSlide();
			expect(result).toBe(true);
			expect(get(currentSlideIndex)).toBe(1);
		});

		it('should not go before first slide', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);

			const result = prevSlide();
			expect(result).toBe(false);
			expect(get(currentSlideIndex)).toBe(0);
		});

		it('should hide fragment before going to previous slide', () => {
			const testPresentation = createTestPresentation(3, [2, 0, 0]);
			presentation.set(testPresentation);
			currentFragmentIndex.set(1);

			// First call hides last fragment
			prevSlide();
			expect(get(currentSlideIndex)).toBe(0);
			expect(get(currentFragmentIndex)).toBe(0);

			// Second call hides first fragment
			prevSlide();
			expect(get(currentSlideIndex)).toBe(0);
			expect(get(currentFragmentIndex)).toBe(-1);

			// Third call does nothing (first slide, no fragments visible)
			prevSlide();
			expect(get(currentSlideIndex)).toBe(0);
		});

		it('should show all fragments when going to previous slide', () => {
			const testPresentation = createTestPresentation(3, [0, 3, 0]);
			presentation.set(testPresentation);
			currentSlideIndex.set(2);

			prevSlide();
			expect(get(currentSlideIndex)).toBe(1);
			expect(get(currentFragmentIndex)).toBe(2); // Last fragment visible
		});
	});

	describe('goToSlide', () => {
		it('should navigate to specific slide', () => {
			const testPresentation = createTestPresentation(5);
			presentation.set(testPresentation);

			const result = goToSlide(3);
			expect(result).toBe(true);
			expect(get(currentSlideIndex)).toBe(3);
		});

		it('should reset fragment index', () => {
			const testPresentation = createTestPresentation(5, [3, 3, 3, 3, 3]);
			presentation.set(testPresentation);
			currentFragmentIndex.set(2);

			goToSlide(3);
			expect(get(currentFragmentIndex)).toBe(-1);
		});

		it('should not navigate to invalid slide index', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);

			expect(goToSlide(-1)).toBe(false);
			expect(goToSlide(3)).toBe(false);
			expect(goToSlide(100)).toBe(false);
		});

		it('should update URL hash', () => {
			const testPresentation = createTestPresentation(5);
			presentation.set(testPresentation);

			goToSlide(3);
			expect(mockWindow.history.replaceState).toHaveBeenCalledWith(null, '', '#4');
		});
	});

	describe('nextFragment', () => {
		it('should reveal next fragment', () => {
			const testPresentation = createTestPresentation(3, [3, 0, 0]);
			presentation.set(testPresentation);

			const result = nextFragment();
			expect(result).toBe(true);
			expect(get(currentFragmentIndex)).toBe(0);
		});

		it('should not reveal past last fragment', () => {
			const testPresentation = createTestPresentation(3, [2, 0, 0]);
			presentation.set(testPresentation);
			currentFragmentIndex.set(1);

			const result = nextFragment();
			expect(result).toBe(false);
			expect(get(currentFragmentIndex)).toBe(1);
		});

		it('should return false when no fragments', () => {
			const testPresentation = createTestPresentation(3);
			presentation.set(testPresentation);

			const result = nextFragment();
			expect(result).toBe(false);
		});
	});

	describe('prevFragment', () => {
		it('should hide last visible fragment', () => {
			const testPresentation = createTestPresentation(3, [3, 0, 0]);
			presentation.set(testPresentation);
			currentFragmentIndex.set(2);

			const result = prevFragment();
			expect(result).toBe(true);
			expect(get(currentFragmentIndex)).toBe(1);
		});

		it('should not hide when no fragments visible', () => {
			const testPresentation = createTestPresentation(3, [3, 0, 0]);
			presentation.set(testPresentation);

			const result = prevFragment();
			expect(result).toBe(false);
			expect(get(currentFragmentIndex)).toBe(-1);
		});
	});

	describe('URL hash management', () => {
		it('initializeFromURL should set slide from hash', () => {
			mockWindow.location.hash = '#3';
			const testPresentation = createTestPresentation(5);
			presentation.set(testPresentation);

			initializeFromURL();
			expect(get(currentSlideIndex)).toBe(2); // 0-based
		});

		it('initializeFromURL should handle empty hash', () => {
			mockWindow.location.hash = '';
			const testPresentation = createTestPresentation(5);
			presentation.set(testPresentation);

			initializeFromURL();
			expect(get(currentSlideIndex)).toBe(0);
		});

		it('initializeFromURL should clamp to valid range', () => {
			mockWindow.location.hash = '#100';
			const testPresentation = createTestPresentation(5);
			presentation.set(testPresentation);

			initializeFromURL();
			expect(get(currentSlideIndex)).toBe(4); // Last slide
		});

		it('initializeFromURL should handle invalid hash', () => {
			mockWindow.location.hash = '#invalid';
			const testPresentation = createTestPresentation(5);
			presentation.set(testPresentation);

			initializeFromURL();
			expect(get(currentSlideIndex)).toBe(0);
		});

		it('setupHashChangeListener should add event listener', () => {
			setupHashChangeListener();
			expect(mockWindow.addEventListener).toHaveBeenCalledWith('hashchange', expect.any(Function));
		});

		it('setupHashChangeListener should return unsubscribe function', () => {
			const unsubscribe = setupHashChangeListener();
			unsubscribe();
			expect(mockWindow.removeEventListener).toHaveBeenCalledWith(
				'hashchange',
				expect.any(Function)
			);
		});
	});

	describe('loadPresentation', () => {
		it('should set presentation and initialize from URL', () => {
			mockWindow.location.hash = '#2';
			const testPresentation = createTestPresentation(5);

			loadPresentation(testPresentation);

			expect(get(presentation)).toEqual(testPresentation);
			expect(get(currentSlideIndex)).toBe(1); // 0-based from #2
		});
	});

	describe('resetPresentation', () => {
		it('should reset all stores to initial values', () => {
			const testPresentation = createTestPresentation(5, [3, 3, 3, 3, 3]);
			presentation.set(testPresentation);
			currentSlideIndex.set(3);
			currentFragmentIndex.set(2);

			resetPresentation();

			expect(get(presentation)).toBeNull();
			expect(get(currentSlideIndex)).toBe(0);
			expect(get(currentFragmentIndex)).toBe(-1);
		});
	});
});
