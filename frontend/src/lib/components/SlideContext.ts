/**
 * Per-slide render context.
 * Slide provides it once; Slot reads it to decide fragment visibility.
 * Layouts never receive or pass these values by hand.
 */

import { createContext } from 'react';

export interface SlideContextValue {
	/** Index of the last visible fragment, -1 when none are visible. */
	fragmentIndex: number;
	/** True when rendering for PDF export; all fragments show at once. */
	printMode: boolean;
}

export const SlideContext = createContext<SlideContextValue>({
	fragmentIndex: -1,
	printMode: false
});
