/**
 * Scales speaker notes so the whole slide's notes fit their panel.
 *
 * Measurement means setting a candidate size on the element and comparing its
 * scroll height to its client height, so the work happens in a layout effect,
 * before the browser paints. When the notes cannot fit even at the floor the
 * floor is used and the panel scrolls, which is what a manual size does too.
 */

import { useCallback, useLayoutEffect, useState, type RefObject } from 'react';
import { findFittingSize } from '$lib/utils/fitText';

export interface UseFitTextOptions {
	/** False leaves the size at capSize and does no measuring. */
	enabled: boolean;
	elementRef: RefObject<HTMLElement | null>;
	/** Largest size to use, which is the presenter's manual size. */
	capSize: number;
	minSize: number;
	step: number;
	/** Changes whenever the notes text changes, forcing a re-measure. */
	contentKey: string | number;
}

export function useFitText({
	enabled,
	elementRef,
	capSize,
	minSize,
	step,
	contentKey
}: UseFitTextOptions): number {
	const [size, setSize] = useState(capSize);

	const measure = useCallback((): void => {
		const element = elementRef.current;
		if (!enabled || !element) {
			setSize(capSize);
			return;
		}
		// A panel that has not been laid out yet measures as zero and would
		// report that nothing fits.
		if (element.clientHeight === 0) {
			setSize(capSize);
			return;
		}
		const fitted = findFittingSize({
			minSize,
			maxSize: capSize,
			step,
			fits: (candidate) => {
				element.style.fontSize = `${candidate}rem`;
				return element.scrollHeight <= element.clientHeight;
			}
		});
		element.style.fontSize = `${fitted}rem`;
		setSize(fitted);
	}, [enabled, elementRef, capSize, minSize, step]);

	useLayoutEffect(() => {
		measure();
	}, [measure, contentKey]);

	useLayoutEffect(() => {
		const element = elementRef.current;
		if (!enabled || !element || typeof ResizeObserver === 'undefined') return;
		const observer = new ResizeObserver(() => measure());
		observer.observe(element);
		return () => observer.disconnect();
	}, [enabled, elementRef, measure]);

	return enabled ? size : capSize;
}
