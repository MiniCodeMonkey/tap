/**
 * Holds the presenter view's chosen layout and notes size mode.
 *
 * The layout is re-resolved whenever the screen crosses the narrow
 * breakpoint, so a window dragged narrow drops out of a two-slide layout and
 * a window dragged wide again returns to it. Choosing a layout writes it to
 * localStorage and retires the URL parameter, which is a one-off override.
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
	availableLayouts,
	nextLayout,
	NARROW_QUERY,
	readStoredLayout,
	readStoredNotesSizeMode,
	resolveLayout,
	resolveNotesSizeMode,
	writeStoredLayout,
	writeStoredNotesSizeMode,
	type NotesSizeMode,
	type PresenterLayout,
	type PresenterLayoutInfo
} from '$lib/utils/presenterLayout';

export interface PresenterLayoutState {
	layout: PresenterLayout;
	isNarrow: boolean;
	availableLayouts: PresenterLayoutInfo[];
	setLayout: (layout: PresenterLayout) => void;
	cycleLayout: () => void;
	notesSizeMode: NotesSizeMode;
	setNotesSizeMode: (mode: NotesSizeMode) => void;
}

/** Reads one search parameter, tolerating a server-side render. */
function readSearchParam(name: string): string | null {
	if (typeof window === 'undefined') return null;
	return new URLSearchParams(window.location.search).get(name);
}

/** jsdom and older browsers may not have matchMedia; treat those as wide. */
function matchesNarrow(): boolean {
	if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false;
	return window.matchMedia(NARROW_QUERY).matches;
}

export function usePresenterLayout(configuredLayout?: string): PresenterLayoutState {
	const [urlLayout, setUrlLayout] = useState(() => readSearchParam('layout'));
	const [storedLayout, setStoredLayout] = useState(readStoredLayout);
	const [isNarrow, setIsNarrow] = useState(matchesNarrow);

	const [urlNotesSize, setUrlNotesSize] = useState(() => readSearchParam('notesSize'));
	const [storedNotesSize, setStoredNotesSize] = useState(readStoredNotesSizeMode);

	useEffect(() => {
		if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
		const query = window.matchMedia(NARROW_QUERY);
		const handleChange = (event: MediaQueryListEvent): void => setIsNarrow(event.matches);
		query.addEventListener('change', handleChange);
		setIsNarrow(query.matches);
		return () => query.removeEventListener('change', handleChange);
	}, []);

	const layout = resolveLayout({
		fromUrl: urlLayout,
		fromStorage: storedLayout,
		fromConfig: configuredLayout,
		isNarrow
	});

	const notesSizeMode = resolveNotesSizeMode({
		fromUrl: urlNotesSize,
		fromStorage: storedNotesSize
	});

	const offered = useMemo(() => availableLayouts(isNarrow), [isNarrow]);

	const setLayout = useCallback((next: PresenterLayout): void => {
		writeStoredLayout(next);
		setStoredLayout(next);
		setUrlLayout(null);
	}, []);

	const cycleLayout = useCallback((): void => {
		setLayout(nextLayout(layout, isNarrow));
	}, [layout, isNarrow, setLayout]);

	const setNotesSizeMode = useCallback((next: NotesSizeMode): void => {
		writeStoredNotesSizeMode(next);
		setStoredNotesSize(next);
		setUrlNotesSize(null);
	}, []);

	return {
		layout,
		isNarrow,
		availableLayouts: offered,
		setLayout,
		cycleLayout,
		notesSizeMode,
		setNotesSizeMode
	};
}
