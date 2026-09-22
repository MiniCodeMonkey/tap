/**
 * Animates between slides using Motion. Wraps the active slide in a frame
 * that enters and exits according to the resolved transition type and the
 * navigation direction. Renders instantly, with no animation, in print mode
 * or when the user prefers reduced motion. An animated change holds the
 * ready signal until the entering slide has finished animating.
 */

import { AnimatePresence, motion } from 'motion/react';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { Transition } from '$lib/types';
import { useReadyHold } from '$lib/ready/blockers';
import {
	TRANSITION_DEFAULTS,
	getEffectiveDuration,
	getTransitionVariants,
	prefersReducedMotion,
	type TransitionDirection
} from '../utils/transitions';

/**
 * Extra time, past the exit and enter animations, after which the ready
 * signal stops waiting for a transition that never reported completion.
 */
export const TRANSITION_READY_MARGIN_MS = 1000;

export interface SlideTransitionProps {
	/** Key identifying the current slide; changing it triggers the transition. */
	slideKey: number | string;
	/** Transition type to animate with. */
	transition: Transition;
	/** Navigation direction, used to mirror directional transitions. */
	direction: TransitionDirection;
	/** Whether the page is in print/PDF export mode. */
	printMode?: boolean;
	children: ReactNode;
}

export function SlideTransition({
	slideKey,
	transition,
	direction,
	printMode = false,
	children
}: SlideTransitionProps) {
	const duration = getEffectiveDuration(TRANSITION_DEFAULTS.defaultDuration);
	const skipAnimation = printMode || prefersReducedMotion() || transition === 'none' || duration === 0;

	// The ready signal waits from a slide change until the entering slide's
	// animation completes. The exiting slide reports completion with its
	// own, older key, which is ignored.
	const [settledKey, setSettledKey] = useState(slideKey);
	const latestKeyRef = useRef(slideKey);
	latestKeyRef.current = slideKey;
	const transitioning = !skipAnimation && settledKey !== slideKey;
	useReadyHold('animations', transitioning);
	const markSettled = (completedKey: number | string): void => {
		if (completedKey === latestKeyRef.current) {
			setSettledKey(completedKey);
		}
	};

	useEffect(() => {
		if (!transitioning) {
			return undefined;
		}
		const timer = setTimeout(() => setSettledKey(slideKey), duration * 2 + TRANSITION_READY_MARGIN_MS);
		return () => clearTimeout(timer);
	}, [transitioning, slideKey, duration]);

	if (skipAnimation) {
		return <div className="slide-transition-frame">{children}</div>;
	}

	const variants = getTransitionVariants(transition, direction);
	// "sync" mounts the entering slide immediately so it can overlap the
	// leaving one; the other transitions wait for the exit to finish first.
	const mode = transition === 'push' || transition === 'slide' ? 'sync' : 'wait';

	return (
		<AnimatePresence mode={mode} initial={false}>
			<motion.div
				key={slideKey}
				className="slide-transition-frame"
				initial={variants.initial}
				animate={variants.animate}
				exit={variants.exit}
				transition={{ duration: duration / 1000, ease: variants.ease }}
				onAnimationComplete={() => markSettled(slideKey)}
			>
				{children}
			</motion.div>
		</AnimatePresence>
	);
}
