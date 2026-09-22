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

	// Skipped slides are left out of both numbers. On the first slide this
	// shows 1/total progress, and on the last one 100%.
	const progressPercent = (position / total) * 100;

	return (
		<div
			className="progress-bar-container"
			role="progressbar"
			aria-valuenow={position}
			aria-valuemin={1}
			aria-valuemax={total}
			aria-label={`Presentation progress: slide ${position} of ${total}`}
		>
			<div className="progress-bar-fill" style={{ width: `${progressPercent}%` }} />
		</div>
	);
}
