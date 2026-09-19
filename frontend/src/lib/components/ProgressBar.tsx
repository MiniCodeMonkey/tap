/**
 * Thin accent-colored bar along the bottom of the viewer showing how far
 * through the presentation the current slide is.
 */

import { usePresentationStore, selectTotalSlides } from '$lib/stores/presentation';

export interface ProgressBarProps {
	/** Whether to show the progress bar (can be disabled via config). */
	show?: boolean;
}

export function ProgressBar({ show = true }: ProgressBarProps) {
	const currentIndex = usePresentationStore((state) => state.currentSlideIndex);
	const total = usePresentationStore(selectTotalSlides);

	if (!show || total <= 0) {
		return null;
	}

	// On the first slide (index 0) this shows 1/total progress; on the last
	// slide (index total - 1) it shows 100%.
	const progressPercent = ((currentIndex + 1) / total) * 100;

	return (
		<div
			className="progress-bar-container"
			role="progressbar"
			aria-valuenow={currentIndex + 1}
			aria-valuemin={1}
			aria-valuemax={total}
			aria-label={`Presentation progress: slide ${currentIndex + 1} of ${total}`}
		>
			<div className="progress-bar-fill" style={{ width: `${progressPercent}%` }} />
		</div>
	);
}
