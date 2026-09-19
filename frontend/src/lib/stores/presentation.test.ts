/**
 * Unit tests for the presentation store.
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
	usePresentationStore,
	selectCurrentSlide,
	selectTotalSlides,
	nextSlide,
	prevSlide,
	goToSlide,
	nextFragment,
	prevFragment,
	applyRemoteState,
	resetPresentation,
	loadPresentation,
	initializeFromURL,
	getHashSlideIndexAtLoad,
	setupHashChangeListener,
	setThemeOverride,
	clearThemeOverride,
	selectCurrentThemeSlug,
	cycleTheme
} from './presentation';
import type { Presentation, Slide } from '$lib/types';

// Mock window object for URL hash tests
const mockWindow = {
	location: { hash: '', search: '' },
	history: {
		replaceState: vi.fn()
	},
	addEventListener: vi.fn(),
	removeEventListener: vi.fn()
};

interface SlideOptions {
	fragmentCount?: number;
	steps?: number;
	scroll?: boolean;
}

function makeSlide(index: number, options: SlideOptions = {}): Slide {
	return {
		index,
		layout: 'default',
		html: `<p>Slide ${index + 1}</p>`,
		slots: {},
		slotOrder: [],
		fragmentCount: options.fragmentCount ?? 0,
		steps: options.steps ?? 0,
		scroll: options.scroll
	};
}

// Helper to create a test presentation
function createTestPresentation(slideCount: number, slideOptions: SlideOptions[] = []): Presentation {
	return {
		config: {
			title: 'Test Presentation',
			theme: 'base',
			transition: 'fade'
		},
		slides: Array.from({ length: slideCount }, (_, i) => makeSlide(i, slideOptions[i] ?? {}))
	};
}

describe('presentation store', () => {
	beforeEach(() => {
		resetPresentation();

		mockWindow.location.hash = '';
		mockWindow.location.search = '';
		mockWindow.history.replaceState.mockClear();
		mockWindow.addEventListener.mockClear();
		mockWindow.removeEventListener.mockClear();

		vi.stubGlobal('window', mockWindow);
	});

	describe('initial state', () => {
		it('should have null presentation initially', () => {
			expect(usePresentationStore.getState().presentation).toBeNull();
		});

		it('should have slide index 0 initially', () => {
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('should have fragment index -1 initially', () => {
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
		});

		it('should have step 0 initially', () => {
			expect(usePresentationStore.getState().currentStep).toBe(0);
		});

		it('should update presentation state', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation });
			expect(usePresentationStore.getState().presentation).toEqual(testPresentation);
		});
	});

	describe('selectors', () => {
		it('selectCurrentSlide should return null when no presentation', () => {
			expect(selectCurrentSlide(usePresentationStore.getState())).toBeNull();
		});

		it('selectCurrentSlide should return first slide initially', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation });
			expect(selectCurrentSlide(usePresentationStore.getState())).toEqual(testPresentation.slides[0]);
		});

		it('selectCurrentSlide should update when slide index changes', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation, currentSlideIndex: 2 });
			expect(selectCurrentSlide(usePresentationStore.getState())).toEqual(testPresentation.slides[2]);
		});

		it('selectTotalSlides should return 0 when no presentation', () => {
			expect(selectTotalSlides(usePresentationStore.getState())).toBe(0);
		});

		it('selectTotalSlides should return slide count', () => {
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });
			expect(selectTotalSlides(usePresentationStore.getState())).toBe(5);
		});
	});

	describe('nextSlide', () => {
		it('should advance to next slide', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation });

			const result = nextSlide();
			expect(result).toBe(true);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
		});

		it('should not advance past last slide', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation, currentSlideIndex: 2 });

			const result = nextSlide();
			expect(result).toBe(false);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
		});

		it('should reveal fragment before advancing slide', () => {
			const testPresentation = createTestPresentation(3, [{ fragmentCount: 2 }]);
			usePresentationStore.setState({ presentation: testPresentation });

			nextSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(0);

			nextSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(1);

			nextSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
		});

		it('should update URL hash when advancing', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation });

			nextSlide();
			expect(mockWindow.history.replaceState).toHaveBeenCalledWith(null, '', '#2');
		});

		it('should walk steps, then fragment, then advance the slide', () => {
			const testPresentation = createTestPresentation(2, [{ steps: 2, fragmentCount: 1 }]);
			usePresentationStore.setState({ presentation: testPresentation });

			nextSlide();
			expect(usePresentationStore.getState().currentStep).toBe(1);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);

			nextSlide();
			expect(usePresentationStore.getState().currentStep).toBe(2);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);

			nextSlide();
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(0);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);

			nextSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
			expect(usePresentationStore.getState().currentStep).toBe(0);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
		});

		it('should reveal scroll before fragments', () => {
			const testPresentation = createTestPresentation(2, [{ scroll: true, fragmentCount: 1 }]);
			usePresentationStore.setState({ presentation: testPresentation });

			nextSlide();
			expect(usePresentationStore.getState().scrollRevealed).toBe(true);
			expect(usePresentationStore.getState().scrollTriggerCount).toBe(1);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);

			nextSlide();
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(0);
		});
	});

	describe('prevSlide', () => {
		it('should go to previous slide', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation, currentSlideIndex: 2 });

			const result = prevSlide();
			expect(result).toBe(true);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
		});

		it('should not go before first slide', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation });

			const result = prevSlide();
			expect(result).toBe(false);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('should hide fragment before going to previous slide', () => {
			const testPresentation = createTestPresentation(3, [{ fragmentCount: 2 }]);
			usePresentationStore.setState({ presentation: testPresentation, currentFragmentIndex: 1 });

			prevSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(0);

			prevSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);

			prevSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('should show all fragments when going to previous slide', () => {
			const testPresentation = createTestPresentation(3, [{}, { fragmentCount: 3 }]);
			usePresentationStore.setState({ presentation: testPresentation, currentSlideIndex: 2 });

			prevSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(2);
		});

		it('should undo fragment, then step, then land on the previous slide', () => {
			const testPresentation = createTestPresentation(2, [{ steps: 2, fragmentCount: 1 }]);
			usePresentationStore.setState({
				presentation: testPresentation,
				currentStep: 2,
				currentFragmentIndex: 0
			});

			prevSlide();
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
			expect(usePresentationStore.getState().currentStep).toBe(2);

			prevSlide();
			expect(usePresentationStore.getState().currentStep).toBe(1);

			prevSlide();
			expect(usePresentationStore.getState().currentStep).toBe(0);

			const result = prevSlide();
			expect(result).toBe(false);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('should land on the final step, fragment and scroll state of the previous slide', () => {
			const testPresentation = createTestPresentation(2, [
				{ steps: 2, fragmentCount: 2, scroll: true },
				{}
			]);
			usePresentationStore.setState({ presentation: testPresentation, currentSlideIndex: 1 });

			prevSlide();

			const state = usePresentationStore.getState();
			expect(state.currentSlideIndex).toBe(0);
			expect(state.currentStep).toBe(2);
			expect(state.currentFragmentIndex).toBe(1);
			expect(state.scrollRevealed).toBe(true);
		});
	});

	describe('goToSlide', () => {
		it('should navigate to specific slide', () => {
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });

			const result = goToSlide(3);
			expect(result).toBe(true);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(3);
		});

		it('should reset step and fragment index', () => {
			const testPresentation = createTestPresentation(5, [{}, {}, {}, { steps: 2, fragmentCount: 3 }]);
			usePresentationStore.setState({
				presentation: testPresentation,
				currentFragmentIndex: 2,
				currentStep: 1
			});

			goToSlide(3);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
			expect(usePresentationStore.getState().currentStep).toBe(0);
		});

		it('should not navigate to an invalid slide index', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation });

			expect(goToSlide(-1)).toBe(false);
			expect(goToSlide(3)).toBe(false);
			expect(goToSlide(100)).toBe(false);
		});

		it('should update URL hash', () => {
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });

			goToSlide(3);
			expect(mockWindow.history.replaceState).toHaveBeenCalledWith(null, '', '#4');
		});
	});

	describe('nextFragment', () => {
		it('should reveal next fragment', () => {
			const testPresentation = createTestPresentation(3, [{ fragmentCount: 3 }]);
			usePresentationStore.setState({ presentation: testPresentation });

			const result = nextFragment();
			expect(result).toBe(true);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(0);
		});

		it('should not reveal past last fragment', () => {
			const testPresentation = createTestPresentation(3, [{ fragmentCount: 2 }]);
			usePresentationStore.setState({ presentation: testPresentation, currentFragmentIndex: 1 });

			const result = nextFragment();
			expect(result).toBe(false);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(1);
		});

		it('should return false when the slide has no fragments', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation });

			const result = nextFragment();
			expect(result).toBe(false);
		});
	});

	describe('prevFragment', () => {
		it('should hide last visible fragment', () => {
			const testPresentation = createTestPresentation(3, [{ fragmentCount: 3 }]);
			usePresentationStore.setState({ presentation: testPresentation, currentFragmentIndex: 2 });

			const result = prevFragment();
			expect(result).toBe(true);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(1);
		});

		it('should not hide when no fragments are visible', () => {
			const testPresentation = createTestPresentation(3, [{ fragmentCount: 3 }]);
			usePresentationStore.setState({ presentation: testPresentation });

			const result = prevFragment();
			expect(result).toBe(false);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
		});
	});

	describe('applyRemoteState', () => {
		it('should apply slide, fragment, step and scroll reveal exactly as given', () => {
			const testPresentation = createTestPresentation(5, [{}, {}, {}, { steps: 2, fragmentCount: 3, scroll: true }]);
			usePresentationStore.setState({ presentation: testPresentation });

			const result = applyRemoteState({ slideIndex: 3, fragment: 1, step: 2, scrollRevealed: true });

			expect(result).toBe(true);
			const state = usePresentationStore.getState();
			expect(state.currentSlideIndex).toBe(3);
			expect(state.currentFragmentIndex).toBe(1);
			expect(state.currentStep).toBe(2);
			expect(state.scrollRevealed).toBe(true);
		});

		it('should not reset fragment/step/scroll to the slide defaults, unlike goToSlide', () => {
			const testPresentation = createTestPresentation(3, [{ fragmentCount: 3 }]);
			usePresentationStore.setState({ presentation: testPresentation });

			applyRemoteState({ slideIndex: 0, fragment: 2, step: 0, scrollRevealed: false });

			expect(usePresentationStore.getState().currentFragmentIndex).toBe(2);
		});

		it('should clamp an out-of-range slide index to the deck instead of dropping the message', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation, currentSlideIndex: 0 });

			const result = applyRemoteState({ slideIndex: 100, fragment: -1, step: 0, scrollRevealed: false });

			expect(result).toBe(true);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
		});

		it('should clamp a negative slide index to 0', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation, currentSlideIndex: 1 });

			const result = applyRemoteState({ slideIndex: -5, fragment: -1, step: 0, scrollRevealed: false });

			expect(result).toBe(true);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('should clamp a huge fragment index to the target slide\'s fragment count', () => {
			const testPresentation = createTestPresentation(2, [{ fragmentCount: 3 }]);
			usePresentationStore.setState({ presentation: testPresentation });

			applyRemoteState({ slideIndex: 0, fragment: 999999999, step: 0, scrollRevealed: false });

			expect(usePresentationStore.getState().currentFragmentIndex).toBe(2);
		});

		it('should clamp a fragment index below -1 up to -1', () => {
			const testPresentation = createTestPresentation(2, [{ fragmentCount: 3 }]);
			usePresentationStore.setState({ presentation: testPresentation });

			applyRemoteState({ slideIndex: 0, fragment: -3, step: 0, scrollRevealed: false });

			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
		});

		it('should clamp a negative step up to 0', () => {
			const testPresentation = createTestPresentation(2, [{ steps: 2 }]);
			usePresentationStore.setState({ presentation: testPresentation });

			applyRemoteState({ slideIndex: 0, fragment: -1, step: -3, scrollRevealed: false });

			expect(usePresentationStore.getState().currentStep).toBe(0);
		});

		it('should clamp a huge step to the target slide\'s step count', () => {
			const testPresentation = createTestPresentation(2, [{ steps: 2 }]);
			usePresentationStore.setState({ presentation: testPresentation });

			applyRemoteState({ slideIndex: 0, fragment: -1, step: 999999999, scrollRevealed: false });

			expect(usePresentationStore.getState().currentStep).toBe(2);
		});

		it('should ignore a message with a non-integer slide index', () => {
			const testPresentation = createTestPresentation(3);
			usePresentationStore.setState({ presentation: testPresentation, currentSlideIndex: 0 });

			const result = applyRemoteState({ slideIndex: 1.5, fragment: -1, step: 0, scrollRevealed: false });

			expect(result).toBe(false);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('should ignore a message with a non-integer fragment or step', () => {
			const testPresentation = createTestPresentation(3, [{ fragmentCount: 3, steps: 2 }]);
			usePresentationStore.setState({ presentation: testPresentation, currentFragmentIndex: -1, currentStep: 0 });

			expect(applyRemoteState({ slideIndex: 0, fragment: 1.5, step: 0, scrollRevealed: false })).toBe(false);
			expect(applyRemoteState({ slideIndex: 0, fragment: 0, step: 0.5, scrollRevealed: false })).toBe(false);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
			expect(usePresentationStore.getState().currentStep).toBe(0);
		});

		it('should ignore a message when no presentation is loaded', () => {
			usePresentationStore.setState({ presentation: null });

			const result = applyRemoteState({ slideIndex: 0, fragment: -1, step: 0, scrollRevealed: false });

			expect(result).toBe(false);
		});

		it('should update the URL hash', () => {
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });

			applyRemoteState({ slideIndex: 3, fragment: -1, step: 0, scrollRevealed: false });
			expect(mockWindow.history.replaceState).toHaveBeenCalledWith(null, '', '#4');
		});
	});

	describe('URL hash management', () => {
		it('initializeFromURL should set slide from hash', () => {
			mockWindow.location.hash = '#3';
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });

			initializeFromURL();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
		});

		it('initializeFromURL should handle empty hash', () => {
			mockWindow.location.hash = '';
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });

			initializeFromURL();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('initializeFromURL should clamp to valid range', () => {
			mockWindow.location.hash = '#100';
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });

			initializeFromURL();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(4);
		});

		it('initializeFromURL should handle an invalid hash', () => {
			mockWindow.location.hash = '#invalid';
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });

			initializeFromURL();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('initializeFromURL should apply ?step= and ?fragment= from the query string', () => {
			mockWindow.location.hash = '#2';
			mockWindow.location.search = '?step=1&fragment=2';
			const testPresentation = createTestPresentation(5, [
				{},
				{ steps: 3, fragmentCount: 4 }
			]);
			usePresentationStore.setState({ presentation: testPresentation });

			initializeFromURL();
			expect(usePresentationStore.getState().currentStep).toBe(1);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(2);
		});

		it('initializeFromURL should clamp ?step= and ?fragment= to the slide limits', () => {
			mockWindow.location.hash = '#2';
			mockWindow.location.search = '?step=99&fragment=99';
			const testPresentation = createTestPresentation(5, [
				{},
				{ steps: 3, fragmentCount: 4 }
			]);
			usePresentationStore.setState({ presentation: testPresentation });

			initializeFromURL();
			expect(usePresentationStore.getState().currentStep).toBe(3);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(3);
		});

		it('initializeFromURL should default step and fragment when the query string is absent', () => {
			mockWindow.location.hash = '#2';
			mockWindow.location.search = '';
			const testPresentation = createTestPresentation(5, [
				{},
				{ steps: 3, fragmentCount: 4 }
			]);
			usePresentationStore.setState({ presentation: testPresentation });

			initializeFromURL();
			expect(usePresentationStore.getState().currentStep).toBe(0);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
		});

		it('setupHashChangeListener should add an event listener', () => {
			setupHashChangeListener();
			expect(mockWindow.addEventListener).toHaveBeenCalledWith('hashchange', expect.any(Function));
		});

		it('setupHashChangeListener should return an unsubscribe function', () => {
			const unsubscribe = setupHashChangeListener();
			unsubscribe();
			expect(mockWindow.removeEventListener).toHaveBeenCalledWith(
				'hashchange',
				expect.any(Function)
			);
		});
	});

	describe('getHashSlideIndexAtLoad', () => {
		it('is null before any load has run', () => {
			expect(getHashSlideIndexAtLoad()).toBeNull();
		});

		it('records the 0-based slide index a hash named', () => {
			mockWindow.location.hash = '#5';
			const testPresentation = createTestPresentation(10);
			usePresentationStore.setState({ presentation: testPresentation });

			initializeFromURL();
			expect(getHashSlideIndexAtLoad()).toBe(4);
		});

		it('is null when the load had no hash', () => {
			mockWindow.location.hash = '';
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });

			initializeFromURL();
			expect(getHashSlideIndexAtLoad()).toBeNull();
		});

		it('is null when the load had an invalid hash', () => {
			mockWindow.location.hash = '#invalid';
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });

			initializeFromURL();
			expect(getHashSlideIndexAtLoad()).toBeNull();
		});

		it('is reset by resetPresentation', () => {
			mockWindow.location.hash = '#3';
			const testPresentation = createTestPresentation(5);
			usePresentationStore.setState({ presentation: testPresentation });
			initializeFromURL();
			expect(getHashSlideIndexAtLoad()).toBe(2);

			resetPresentation();
			expect(getHashSlideIndexAtLoad()).toBeNull();
		});
	});

	describe('loadPresentation', () => {
		it('should set presentation and initialize from URL', () => {
			mockWindow.location.hash = '#2';
			const testPresentation = createTestPresentation(5);

			loadPresentation(testPresentation);

			expect(usePresentationStore.getState().presentation).toEqual(testPresentation);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
		});

		it('should apply ?step= and ?fragment= for the hash-named slide', () => {
			mockWindow.location.hash = '#2';
			mockWindow.location.search = '?step=2&fragment=1';
			const testPresentation = createTestPresentation(5, [
				{},
				{ steps: 4, fragmentCount: 3 }
			]);

			loadPresentation(testPresentation);

			expect(usePresentationStore.getState().currentStep).toBe(2);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(1);
		});
	});

	describe('resetPresentation', () => {
		it('should reset all state to initial values', () => {
			const testPresentation = createTestPresentation(5, [{}, {}, {}, { fragmentCount: 3 }]);
			usePresentationStore.setState({
				presentation: testPresentation,
				currentSlideIndex: 3,
				currentFragmentIndex: 2
			});

			resetPresentation();

			expect(usePresentationStore.getState().presentation).toBeNull();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
			expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
		});
	});

	describe('theme override', () => {
		it('should set and clear the theme override', () => {
			setThemeOverride('base');
			expect(usePresentationStore.getState().themeOverride).toBe('base');

			clearThemeOverride();
			expect(usePresentationStore.getState().themeOverride).toBeNull();
		});
	});

	describe('selectCurrentThemeSlug', () => {
		it('falls back to base when nothing is set', () => {
			resetPresentation();
			expect(selectCurrentThemeSlug(usePresentationStore.getState())).toBe('base');
		});

		it('prefers the deck config theme over base', () => {
			resetPresentation();
			const presentation = createTestPresentation(1);
			presentation.config.theme = 'terminal';
			loadPresentation(presentation);
			expect(selectCurrentThemeSlug(usePresentationStore.getState())).toBe('terminal');
		});

		it('prefers the store override over the deck config theme', () => {
			resetPresentation();
			const presentation = createTestPresentation(1);
			presentation.config.theme = 'terminal';
			loadPresentation(presentation);
			setThemeOverride('swiss');
			expect(selectCurrentThemeSlug(usePresentationStore.getState())).toBe('swiss');
		});
	});

	describe('cycleTheme', () => {
		it('moves to the next theme in themes.json order, wrapping around', () => {
			resetPresentation();
			setThemeOverride('base');

			cycleTheme();
			const afterFirst = usePresentationStore.getState().themeOverride;
			expect(afterFirst).not.toBe('base');
			expect(afterFirst).not.toBeNull();
		});

		it('wraps from the last theme back to base', () => {
			resetPresentation();
			setThemeOverride('transit');

			cycleTheme();
			expect(usePresentationStore.getState().themeOverride).toBe('base');
		});
	});
});
