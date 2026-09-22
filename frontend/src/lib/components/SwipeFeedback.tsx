/**
 * A short confirmation that a swipe landed.
 *
 * On a phone there is no click, no arrow key travel and often no slide
 * transition, so a swipe on a deck with `transition: none` changes the
 * screen with nothing to say that the gesture was the cause. This shows the
 * direction and the new position for a moment, and says so when the swipe
 * hit the first or last slide and nothing moved.
 */

import { useEffect, useState } from 'react';
import { usePresentationStore, selectPresentedSlideCount, selectPresentedSlideNumber } from '$lib/stores/presentation';

export interface SwipeFeedbackProps {
	/** Which way the last swipe went, or null before the first one. */
	direction: 'next' | 'prev' | null;
	/** Whether that swipe actually moved the deck. */
	moved: boolean;
	/**
	 * Changes on every swipe, including a repeat in the same direction, so
	 * the same gesture twice shows twice.
	 */
	nonce: number;
}

/** How long the pill stays up, in milliseconds. */
const VISIBLE_MS = 900;

export function SwipeFeedback({ direction, moved, nonce }: SwipeFeedbackProps) {
	const currentIndex = usePresentationStore((state) => state.currentSlideIndex);
	const presentedNumber = usePresentationStore(selectPresentedSlideNumber);
	const total = usePresentationStore(selectPresentedSlideCount);
	const [visible, setVisible] = useState(false);

	useEffect(() => {
		if (!direction) {
			return;
		}

		setVisible(true);
		const timer = setTimeout(() => setVisible(false), VISIBLE_MS);
		return () => clearTimeout(timer);
	}, [direction, nonce]);

	if (!direction || !visible) {
		return null;
	}

	return (
		<div
			className={`swipe-feedback${moved ? '' : ' blocked'}`}
			role="status"
			aria-live="polite"
			key={nonce}
		>
			{direction === 'prev' ? (
				<span className="swipe-feedback-arrow" aria-hidden="true">
					&#8249;
				</span>
			) : null}
			<span className="swipe-feedback-position">
				{moved ? `${presentedNumber ?? currentIndex + 1} / ${total}` : direction === 'next' ? 'Last slide' : 'First slide'}
			</span>
			{direction === 'next' ? (
				<span className="swipe-feedback-arrow" aria-hidden="true">
					&#8250;
				</span>
			) : null}
		</div>
	);
}
