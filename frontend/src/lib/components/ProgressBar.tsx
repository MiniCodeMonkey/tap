/**
 * Thin accent-colored bar along the bottom of the viewer showing how far
 * through the presentation the current slide is.
 */

import { usePresentationStore, selectPresentedSlideCount } from '$lib/stores/presentation';
import { presentedSlidesThrough } from '$lib/utils/skip';

export interface ProgressBarProps {
	/** Whether to show the progress bar (can be disabled via config). */
	show?: boolean;
}

export function ProgressBar({ show = true }: ProgressBarProps) {
	const position = usePresentationStore((state) =>
		presentedSlidesThrough(state.presentation?.slides ?? [], state.currentSlideIndex)
	);
	const total = usePresentationStore(selectPresentedSlideCount);

	if (!show || total <= 0) {
		return null;
	}

	// Skipped slides are left out of both numbers. On the first presented
	// slide this shows 1/total progress, and on the last one 100%. Opening a
	// skipped slide directly - tap dev only, ahead of any presented slide -
	// leaves position at 0: an empty bar, honestly reported rather than
	// floored to 1, which would claim progress that has not happened.
	const progressPercent = (position / total) * 100;

	// The minimum stays fixed at 0, the value the fill calculation above
	// already treats as empty, so a screen reader's own percentage
	// ((value - min) / (max - min)) always agrees with the bar it draws.
	return (
		<div
			className="progress-bar-container"
			role="progressbar"
			aria-valuenow={position}
			aria-valuemin={0}
			aria-valuemax={total}
			aria-label={`Presentation progress: slide ${position} of ${total}`}
		>
			<div className="progress-bar-fill" style={{ width: `${progressPercent}%` }} />
		</div>
	);
}
