/**
 * Runs the ready signal for the slide a page shows (see readySignal.ts):
 * a new cycle starts whenever the revision, slide, step, fragment or theme
 * changes, and the signal is cleared while the deck has not loaded.
 */

import { useEffect } from 'react';
import { createDomProbes } from './probes';
import { clearReady, startReadyCycle } from './readySignal';

export interface ReadySignalState {
	/** False while the deck is loading or failed to load; the page then reports nothing. */
	enabled: boolean;
	/** The deck's revision, or "" when it has none (a static build). */
	revision: string;
	/** The 1-based slide on screen. */
	slide: number;
	/** The step rendered, from 0 to the slide's step count. */
	step: number;
	/** The fragment rendered. A change restarts the signal; it is not part of the payload. */
	fragment: number;
	/** The theme applied. A change restarts the signal; it is not part of the payload. */
	theme: string;
	/** See DomProbeOptions.includeInfiniteAnimations. */
	includeInfiniteAnimations: boolean;
	/** See DomProbeOptions.requirePaint. */
	requirePaint: boolean;
}

export function useReadySignal({
	enabled,
	revision,
	slide,
	step,
	fragment,
	theme,
	includeInfiniteAnimations,
	requirePaint
}: ReadySignalState): void {
	useEffect(() => {
		if (!enabled) {
			clearReady();
			return undefined;
		}
		return startReadyCycle({ revision, slide, step }, createDomProbes({ includeInfiniteAnimations, requirePaint }));
		// fragment and theme are not read here, but a change to either is a
		// new rendering that has to settle again.
	}, [enabled, revision, slide, step, fragment, theme, includeInfiniteAnimations, requirePaint]);
}
