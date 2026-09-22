/**
 * Presentation state store for slide navigation and step/fragment management.
 * Built on Zustand; navigation is driven by plain functions that read and
 * write the store outside of React render.
 */

import { create } from 'zustand';
import type { Presentation, Slide, Theme } from '$lib/types';
import { listThemes } from '$lib/themes/loader';

// ============================================================================
// Store State
// ============================================================================

export interface PresentationState {
	presentation: Presentation | null;
	currentSlideIndex: number;
	/** Presenter step within the current slide, from 0 to slide.steps. */
	currentStep: number;
	/** Fragment index within the current slide, from -1 to slide.fragmentCount - 1. */
	currentFragmentIndex: number;
	/** Whether the scroll reveal animation has completed for the current slide. */
	scrollRevealed: boolean;
	/** Counter incremented each time scroll is triggered by user navigation. */
	scrollTriggerCount: number;
	/** Theme override from WebSocket, overriding the presentation config theme. */
	themeOverride: Theme | null;
}

const initialState: PresentationState = {
	presentation: null,
	currentSlideIndex: 0,
	currentStep: 0,
	currentFragmentIndex: -1,
	scrollRevealed: false,
	scrollTriggerCount: 0,
	themeOverride: null
};

export const usePresentationStore = create<PresentationState>(() => ({ ...initialState }));

// ============================================================================
// Selectors
// ============================================================================

/**
 * The current slide object, or null if no presentation is loaded.
 */
export const selectCurrentSlide = (state: PresentationState): Slide | null => {
	if (!state.presentation || state.presentation.slides.length === 0) {
		return null;
	}
	return state.presentation.slides[state.currentSlideIndex] ?? null;
};

/**
 * Total number of slides in the presentation.
 */
export const selectTotalSlides = (state: PresentationState): number => {
	return state.presentation?.slides.length ?? 0;
};

/**
 * The URL's `?theme=` query parameter, or null if absent or not running in a
 * browser. Sits between the deck's own theme and the store's live override
 * in precedence.
 */
function queryThemeOverride(): Theme | null {
	if (typeof window === 'undefined') {
		return null;
	}
	return new URLSearchParams(window.location.search).get('theme');
}

/**
 * The theme currently in effect, in priority order: the store's live
 * override (set by the `t` key or a websocket `theme` message), then the
 * `?theme=` query parameter, then the deck's own `theme:` frontmatter,
 * falling back to `base`.
 */
export const selectCurrentThemeSlug = (state: PresentationState): Theme => {
	return state.themeOverride ?? queryThemeOverride() ?? state.presentation?.config?.theme ?? 'base';
};

// ============================================================================
// Load-time query parameter cleanup
// ============================================================================

/**
 * Whether stripLoadTimeQueryParams has already run for this page load. Set
 * once, since the URL never carries `?step=`/`?fragment=` again after the
 * first strip - checking first avoids rewriting the URL (and touching
 * history) on every navigation action for nothing.
 */
let hasStrippedLoadTimeQueryParams = false;

/**
 * Remove `?step=` and `?fragment=` from the URL, keeping every other query
 * parameter (`theme`, `capture`, `live`, `print`, ...). These two are
 * load-time only: they place the initial load on a specific presenter step
 * and fragment (see resolveInitialStepAndFragment), but replaceState's
 * hash-only updates otherwise carry them forward untouched, so a reload
 * after any navigation would land back on whatever step/fragment they
 * named instead of where the clicker actually is. Called once, from every
 * navigation entry point below, so the first navigation away from the
 * initial state - a step or fragment press included, not only a slide
 * change - is what clears them.
 */
function stripLoadTimeQueryParams(): void {
	if (typeof window === 'undefined' || hasStrippedLoadTimeQueryParams) {
		return;
	}
	hasStrippedLoadTimeQueryParams = true;

	const search = window.location.search;
	if (!search) {
		return;
	}
	const params = new URLSearchParams(search);
	if (!params.has('step') && !params.has('fragment')) {
		return;
	}
	params.delete('step');
	params.delete('fragment');
	const newSearch = params.toString();
	window.history.replaceState(
		null,
		'',
		`${window.location.pathname}${newSearch ? `?${newSearch}` : ''}${window.location.hash}`
	);
}

/**
 * Reset the once-per-load query param strip (for testing).
 */
export function resetStrippedLoadTimeQueryParams(): void {
	hasStrippedLoadTimeQueryParams = false;
}

// ============================================================================
// Navigation Actions
// ============================================================================

/**
 * Navigate forward. Order of operations:
 * 1. Steps - advance the presenter step within the slide.
 * 2. Scroll - reveal the slide's scroll content.
 * 3. Fragments - reveal fragments one by one.
 * 4. Next slide - advance to the next slide, resetting step/scroll/fragment state.
 * Returns true if navigation occurred.
 */
export function nextSlide(): boolean {
	stripLoadTimeQueryParams();
	const state = usePresentationStore.getState();
	const slide = selectCurrentSlide(state);
	if (!slide) return false;

	if (state.currentStep < slide.steps) {
		usePresentationStore.setState({ currentStep: state.currentStep + 1 });
		return true;
	}

	if (slide.scroll === true && !state.scrollRevealed) {
		usePresentationStore.setState({
			scrollRevealed: true,
			scrollTriggerCount: state.scrollTriggerCount + 1
		});
		return true;
	}

	const fragmentCount = slide.fragmentCount;
	if (fragmentCount > 0 && state.currentFragmentIndex < fragmentCount - 1) {
		usePresentationStore.setState({ currentFragmentIndex: state.currentFragmentIndex + 1 });
		return true;
	}

	const total = state.presentation?.slides.length ?? 0;
	if (state.currentSlideIndex < total - 1) {
		const newSlideIndex = state.currentSlideIndex + 1;
		usePresentationStore.setState({
			currentSlideIndex: newSlideIndex,
			currentStep: 0,
			currentFragmentIndex: -1,
			scrollRevealed: false
		});
		updateURLHash(newSlideIndex);
		return true;
	}

	return false;
}

/**
 * Navigate backward. Order of operations (reverse of nextSlide):
 * 1. Fragments - hide fragments in reverse order.
 * 2. Scroll - reset the slide's scroll content to the top.
 * 3. Steps - retreat the presenter step within the slide.
 * 4. Previous slide - go to the previous slide, landing on its final state
 *    (all steps taken, scrolled to the bottom, all fragments visible).
 * Returns true if navigation occurred.
 */
export function prevSlide(): boolean {
	stripLoadTimeQueryParams();
	const state = usePresentationStore.getState();
	const slide = selectCurrentSlide(state);
	if (!slide) return false;

	const fragmentCount = slide.fragmentCount;
	if (fragmentCount > 0 && state.currentFragmentIndex >= 0) {
		usePresentationStore.setState({ currentFragmentIndex: state.currentFragmentIndex - 1 });
		return true;
	}

	if (slide.scroll === true && state.scrollRevealed) {
		usePresentationStore.setState({ scrollRevealed: false });
		return true;
	}

	if (state.currentStep > 0) {
		usePresentationStore.setState({ currentStep: state.currentStep - 1 });
		return true;
	}

	if (state.currentSlideIndex > 0) {
		const newSlideIndex = state.currentSlideIndex - 1;
		const newSlide = state.presentation?.slides[newSlideIndex] ?? null;
		const newFragmentCount = newSlide?.fragmentCount ?? 0;
		const newHasScroll = newSlide?.scroll === true;
		usePresentationStore.setState({
			currentSlideIndex: newSlideIndex,
			currentStep: newSlide?.steps ?? 0,
			currentFragmentIndex: newFragmentCount > 0 ? newFragmentCount - 1 : -1,
			scrollRevealed: newHasScroll
		});
		updateURLHash(newSlideIndex);
		return true;
	}

	return false;
}

/**
 * Navigate directly to a specific slide.
 * Resets step, fragment and scroll state.
 */
export function goToSlide(index: number): boolean {
	stripLoadTimeQueryParams();
	const state = usePresentationStore.getState();
	const total = state.presentation?.slides.length ?? 0;
	if (index < 0 || index >= total) {
		return false;
	}

	usePresentationStore.setState({
		currentSlideIndex: index,
		currentStep: 0,
		currentFragmentIndex: -1,
		scrollRevealed: false
	});
	updateURLHash(index);
	return true;
}

/** Clamps value to the inclusive [min, max] range. */
function clamp(value: number, min: number, max: number): number {
	return Math.min(Math.max(value, min), max);
}

/**
 * Apply slide, fragment, step and scroll reveal state received from a
 * remote client over the websocket. Unlike goToSlide, this does not reset
 * fragment/step/scroll state to the slide's initial values - it applies the
 * given values exactly, so the receiving client mirrors the sender's state.
 *
 * The hub relays whatever a client sends it without validating it against
 * any particular deck (it doesn't know the deck), so this is where hostile
 * or simply out-of-range values are made safe: a non-integer is rejected
 * outright (there's no sane way to clamp "not a number" into range), the
 * slide index is clamped to the deck instead of dropping the whole message,
 * and fragment/step are clamped against the *target* slide (the one
 * slideIndex clamped to), since a slide's fragment count and step count can
 * differ from the sender's current slide.
 *
 * Returns true if the state was applied.
 */
export function applyRemoteState(state: {
	slideIndex: number;
	fragment: number;
	step: number;
	scrollRevealed: boolean;
}): boolean {
	stripLoadTimeQueryParams();
	const current = usePresentationStore.getState();
	const slides = current.presentation?.slides ?? [];
	const total = slides.length;
	if (total === 0) {
		return false;
	}
	if (
		!Number.isInteger(state.slideIndex) ||
		!Number.isInteger(state.fragment) ||
		!Number.isInteger(state.step)
	) {
		return false;
	}

	const slideIndex = clamp(state.slideIndex, 0, total - 1);
	const targetSlide = slides[slideIndex];
	if (!targetSlide) {
		return false;
	}
	const fragment = clamp(state.fragment, -1, Math.max(targetSlide.fragmentCount - 1, -1));
	const step = clamp(state.step, 0, Math.max(targetSlide.steps, 0));

	// Bump the scroll trigger count only when scroll reveal turns on, mirroring
	// nextSlide's behavior, so the receiving client animates the reveal the
	// same way local navigation would.
	const revealingScroll = state.scrollRevealed && !current.scrollRevealed;

	usePresentationStore.setState({
		currentSlideIndex: slideIndex,
		currentFragmentIndex: fragment,
		currentStep: step,
		scrollRevealed: state.scrollRevealed,
		scrollTriggerCount: revealingScroll ? current.scrollTriggerCount + 1 : current.scrollTriggerCount
	});
	updateURLHash(slideIndex);
	return true;
}

/**
 * Reveal the next fragment without changing slides or steps.
 * Returns true if a fragment was revealed.
 */
export function nextFragment(): boolean {
	stripLoadTimeQueryParams();
	const state = usePresentationStore.getState();
	const slide = selectCurrentSlide(state);
	if (!slide) return false;

	const fragmentCount = slide.fragmentCount;
	if (fragmentCount > 0 && state.currentFragmentIndex < fragmentCount - 1) {
		usePresentationStore.setState({ currentFragmentIndex: state.currentFragmentIndex + 1 });
		return true;
	}

	return false;
}

/**
 * Hide the last visible fragment without changing slides or steps.
 * Returns true if a fragment was hidden.
 */
export function prevFragment(): boolean {
	stripLoadTimeQueryParams();
	const state = usePresentationStore.getState();
	const slide = selectCurrentSlide(state);
	if (!slide) return false;

	const fragmentCount = slide.fragmentCount;
	if (fragmentCount > 0 && state.currentFragmentIndex >= 0) {
		usePresentationStore.setState({ currentFragmentIndex: state.currentFragmentIndex - 1 });
		return true;
	}

	return false;
}

// ============================================================================
// URL Hash Management
// ============================================================================

/**
 * Update the URL hash to reflect the current slide.
 * Uses #1, #2, etc. (1-based for user-friendly URLs).
 */
function updateURLHash(slideIndex: number): void {
	if (typeof window !== 'undefined') {
		const hash = `#${slideIndex + 1}`;
		// Use replaceState to avoid polluting browser history
		window.history.replaceState(null, '', hash);
	}
}

/**
 * Parse the URL hash to get the 0-based slide index it names, or null if
 * the hash is absent, empty, or does not name a valid slide (e.g. "#abc").
 * Distinct from parseURLHash below: this is what lets a caller tell "no
 * hash" apart from "hash names slide 1", which parseURLHash alone cannot
 * (both parse to index 0).
 */
function parseURLHashSlideIndex(): number | null {
	if (typeof window === 'undefined') {
		return null;
	}

	const hash = window.location.hash;
	if (!hash || hash === '#') {
		return null;
	}

	// Parse #1, #2, etc. (1-based) to 0-based index
	const slideNumber = parseInt(hash.slice(1), 10);
	if (isNaN(slideNumber) || slideNumber < 1) {
		return null;
	}

	return slideNumber - 1;
}

/**
 * Parse the URL hash to get the slide index.
 * Returns 0 if the hash is invalid or not present.
 */
function parseURLHash(): number {
	return parseURLHashSlideIndex() ?? 0;
}

/**
 * The 0-based slide index the URL hash named the last time
 * initializeFromURL ran, or null if that load had no hash (or an invalid
 * one). Read by the websocket client to decide whether the hub's
 * late-joiner state or the URL hash wins on a normal page load (see
 * applyHubLateJoinerState in stores/websocket.ts).
 */
let hashSlideIndexAtLoad: number | null = null;

/**
 * Get the slide index the URL hash named at the last initializeFromURL
 * call, or null if there was none.
 */
export function getHashSlideIndexAtLoad(): number | null {
	return hashSlideIndexAtLoad;
}

/**
 * Reset the remembered hash-at-load slide index (for testing).
 */
export function resetHashSlideIndexAtLoad(): void {
	hashSlideIndexAtLoad = null;
}

/**
 * Parse the URL hash and record it as this load's hash-at-load slide index,
 * without touching the store. Split out from initializeFromURL so
 * loadPresentation can record it before setting `presentation` in the
 * store: that setState is what fires the websocket subscription which
 * resolves the hub's buffered late-joiner state against this value (see
 * applyHubLateJoinerState in stores/websocket.ts), so it has to already be
 * current by the time that setState runs, not only by the time
 * initializeFromURL itself runs afterward.
 */
function recordHashSlideIndexAtLoad(): number | null {
	const hashIndex = parseURLHashSlideIndex();
	hashSlideIndexAtLoad = hashIndex;
	return hashIndex;
}

/** Clamps a hash-named slide index (or 0, if there was none) to a valid slide. */
function clampHashSlideIndex(hashIndex: number | null, total: number): number {
	const slideIndex = hashIndex ?? 0;
	return total > 0 ? Math.min(slideIndex, total - 1) : slideIndex;
}

/**
 * Parse `?step=` and `?fragment=` from the URL query string. Each is null
 * when absent or not a valid integer. Used only at load (see
 * resolveInitialStepAndFragment) to land directly on a specific presenter
 * step and/or fragment - `tap export images --step`/`--fragment` navigates
 * here instead of driving the clicker - without disturbing normal
 * clicker-driven navigation, which never touches these params.
 */
function parseURLStepFragment(): { step: number | null; fragment: number | null } {
	if (typeof window === 'undefined') {
		return { step: null, fragment: null };
	}
	const params = new URLSearchParams(window.location.search);
	const parseParam = (name: string): number | null => {
		const raw = params.get(name);
		if (raw === null) return null;
		const value = parseInt(raw, 10);
		return Number.isNaN(value) ? null : value;
	};
	return { step: parseParam('step'), fragment: parseParam('fragment') };
}

/**
 * Resolve the initial step and fragment index for a slide: the
 * `?step=`/`?fragment=` URL query parameters, clamped to the slide's own
 * limits, or the slide's normal initial state (step 0, no fragment
 * revealed) when a parameter is absent.
 */
function resolveInitialStepAndFragment(slide: Slide | null): { step: number; fragment: number } {
	const { step, fragment } = parseURLStepFragment();
	const maxStep = Math.max(slide?.steps ?? 0, 0);
	const maxFragment = Math.max((slide?.fragmentCount ?? 0) - 1, -1);
	return {
		step: step === null ? 0 : clamp(step, 0, maxStep),
		fragment: fragment === null ? -1 : clamp(fragment, -1, maxFragment)
	};
}

/**
 * Initialize the store from the URL hash, and `?step=`/`?fragment=` if
 * present. Call this after loading the presentation data.
 */
export function initializeFromURL(): void {
	const hashIndex = recordHashSlideIndexAtLoad();
	const state = usePresentationStore.getState();
	const total = state.presentation?.slides.length ?? 0;
	if (total > 0) {
		const slideIndex = clampHashSlideIndex(hashIndex, total);
		const slide = state.presentation?.slides[slideIndex] ?? null;
		const { step, fragment } = resolveInitialStepAndFragment(slide);
		usePresentationStore.setState({
			currentSlideIndex: slideIndex,
			currentStep: step,
			currentFragmentIndex: fragment,
			scrollRevealed: false
		});
	}
}

/**
 * Set up a listener for hashchange events.
 * Returns an unsubscribe function.
 */
export function setupHashChangeListener(): () => void {
	if (typeof window === 'undefined') {
		return () => {};
	}

	const handleHashChange = (): void => {
		const slideIndex = parseURLHash();
		const state = usePresentationStore.getState();
		const total = state.presentation?.slides.length ?? 0;
		if (slideIndex >= 0 && slideIndex < total) {
			usePresentationStore.setState({
				currentSlideIndex: slideIndex,
				currentStep: 0,
				currentFragmentIndex: -1,
				scrollRevealed: false
			});
		}
	};

	window.addEventListener('hashchange', handleHashChange);

	return () => {
		window.removeEventListener('hashchange', handleHashChange);
	};
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Reset the presentation state to its initial values.
 */
export function resetPresentation(): void {
	usePresentationStore.setState({ ...initialState });
	resetHashSlideIndexAtLoad();
	resetStrippedLoadTimeQueryParams();
}

/**
 * Load a presentation and initialize from the URL hash.
 * Also exposes the presentation on window.presentation for PDF export.
 *
 * Sets `presentation` and the hash-clamped slide index in a single setState
 * call, rather than setting `presentation` and calling initializeFromURL as
 * two separate steps: that first setState is what triggers the websocket
 * store subscription that resolves the hub's buffered late-joiner state
 * (see stores/websocket.ts), and that subscription's own state changes are
 * made from inside this same setState call (zustand runs listeners
 * synchronously). A second, later setState call for the hash would clobber
 * whatever the subscription just decided - applying the hub's state, or
 * deliberately leaving the hash's slide alone - the moment it ran.
 */
export function loadPresentation(data: Presentation): void {
	const hashIndex = recordHashSlideIndexAtLoad();
	const slideIndex = clampHashSlideIndex(hashIndex, data.slides.length);
	const { step, fragment } = resolveInitialStepAndFragment(data.slides[slideIndex] ?? null);
	usePresentationStore.setState({
		presentation: data,
		currentSlideIndex: slideIndex,
		currentStep: step,
		currentFragmentIndex: fragment,
		scrollRevealed: false
	});

	// Expose on window for PDF exporter to access slide count
	if (typeof window !== 'undefined') {
		(window as unknown as { presentation: Presentation }).presentation = data;
	}
}

/**
 * Set the theme override from a WebSocket message.
 * This temporarily overrides the theme without modifying the markdown file.
 */
export function setThemeOverride(theme: Theme): void {
	usePresentationStore.setState({ themeOverride: theme });
}

/**
 * Clear the theme override, reverting to the presentation config theme.
 */
export function clearThemeOverride(): void {
	usePresentationStore.setState({ themeOverride: null });
}

/**
 * Switch to the next theme, in the order listThemes() returns (themes.json
 * order, base first), wrapping around. Sets the store's theme override, the
 * same mechanism the websocket `theme` message uses.
 */
export function cycleTheme(): void {
	const themes = listThemes();
	if (themes.length === 0) {
		return;
	}

	const current = selectCurrentThemeSlug(usePresentationStore.getState());
	const index = themes.findIndex((theme) => theme.slug === current);
	const nextIndex = index === -1 ? 0 : (index + 1) % themes.length;
	const next = themes[nextIndex];
	if (next) {
		setThemeOverride(next.slug);
	}
}
