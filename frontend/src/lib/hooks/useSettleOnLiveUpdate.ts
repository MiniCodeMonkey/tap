/**
 * Keeps a live update from replaying a slide's entrance animations.
 *
 * Slot renders its HTML with dangerouslySetInnerHTML, so an in-place update
 * to the slide on screen (an edit in `tap dev` or Tap Desktop, see
 * updatePresentationInPlace) swaps in brand new elements, and every theme
 * entrance animation on them (`.slot > * { animation: ... }`) starts over.
 * Entrance animations belong to arriving at a slide, so while the slide on
 * screen shows content that arrived by a live update, this hook jumps every
 * CSS animation on newly added elements straight to its end. That covers
 * both the slot swap itself and the rich blocks that replace a code block
 * or diagram a moment later.
 *
 * Arriving at a slide is left alone: navigation changes the slide's index
 * (and usually remounts the Slide under SlideTransition's key), which ends
 * the live update state. A fragment or step change ends it too, so anything
 * a reveal adds afterwards animates as it always has. Infinite animations
 * (a blinking cursor, a pulse) keep running, since they are not entrances.
 */

import { useLayoutEffect, useRef, type RefObject } from 'react';
import type { Slide } from '$lib/types';

interface SettleState {
	slide: Slide;
	fragmentIndex: number;
	step: number;
}

/** Jumps every finite CSS animation on `element` and its descendants to its end state. */
export function finishCssAnimations(element: Element): void {
	if (typeof element.getAnimations !== 'function') {
		return;
	}
	for (const animation of element.getAnimations({ subtree: true })) {
		// Only CSS animations (a theme's keyframes), never a deck component's
		// Motion or Web Animations, which run under that component's control.
		if (!('animationName' in animation)) {
			continue;
		}
		if (animation.effect?.getComputedTiming().endTime === Infinity) {
			continue;
		}
		try {
			animation.finish();
		} catch {
			// Already gone from the document, or otherwise not finishable; leave it.
		}
	}
}

export function useSettleOnLiveUpdate(
	rootRef: RefObject<HTMLElement | null>,
	slide: Slide,
	fragmentIndex: number,
	step: number
): void {
	const previousRef = useRef<SettleState | null>(null);
	const liveUpdatedRef = useRef(false);

	useLayoutEffect(() => {
		const previous = previousRef.current;
		previousRef.current = { slide, fragmentIndex, step };
		if (previous === null) {
			return;
		}
		if (previous.slide !== slide) {
			liveUpdatedRef.current = previous.slide.index === slide.index;
		} else if (previous.fragmentIndex !== fragmentIndex || previous.step !== step) {
			liveUpdatedRef.current = false;
		}
	}, [slide, fragmentIndex, step]);

	useLayoutEffect(() => {
		const root = rootRef.current;
		if (!root || typeof MutationObserver === 'undefined') {
			return undefined;
		}
		// A MutationObserver callback runs as a microtask right after the
		// commit that swapped the slot's HTML, after every layout effect
		// (so liveUpdatedRef is already current) and before the next paint.
		const observer = new MutationObserver((records) => {
			if (!liveUpdatedRef.current) {
				return;
			}
			for (const record of records) {
				record.addedNodes.forEach((node) => {
					if (node instanceof Element && node.isConnected) {
						finishCssAnimations(node);
					}
				});
			}
		});
		observer.observe(root, { childList: true, subtree: true });
		return () => observer.disconnect();
	}, [rootRef]);
}
