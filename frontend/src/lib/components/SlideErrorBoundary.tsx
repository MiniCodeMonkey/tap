/**
 * Catches a render failure in one slide's layout so the rest of the
 * presentation keeps working. In a server-backed runtime (`tap dev`, `tap
 * export pdf`, `tap export images`, or the frontend's own dev server - see
 * isDevRuntime) it shows an error card naming the slide, so a broken slide
 * is visible as broken instead of silently falling back; in a static `tap
 * build` output it falls back to the slide's raw slot content instead of
 * going blank. The audience-safe form (see shouldUseSafeErrorForm) renders
 * that same fallback next to its marker, so the audience sees the slide's
 * normal content instead of an empty one.
 */

import { Component, type ReactNode } from 'react';
import { isDevRuntime } from '$lib/utils/runtime';
import { useSafeErrorForm } from '$lib/hooks/useSafeErrorForm';

interface SlideErrorBoundaryProps {
	slideNumber: number;
	fallback: ReactNode;
	children: ReactNode;
}

interface SlideErrorBoundaryState {
	hasError: boolean;
}

/**
 * The dev-mode error display, split out from SlideErrorBoundary's render()
 * so it can call useSafeErrorForm: a class component can't use hooks
 * itself, and the safe form must switch live as the viewer enters or
 * leaves fullscreen, not only on the boundary's own next re-render.
 */
function SlideErrorDisplay({ message, fallback }: { message: string; fallback: ReactNode }) {
	const safe = useSafeErrorForm();
	if (safe) {
		return (
			<>
				{fallback}
				<div className="slide-error deck-error-card deck-error-card-safe" data-message={message} hidden />
				<div className="deck-error-marker" title={message}>
					component error
				</div>
			</>
		);
	}
	return (
		<div className="slide-error deck-error-card" data-message={message}>
			{message}
		</div>
	);
}

export class SlideErrorBoundary extends Component<SlideErrorBoundaryProps, SlideErrorBoundaryState> {
	state: SlideErrorBoundaryState = { hasError: false };

	static getDerivedStateFromError(): SlideErrorBoundaryState {
		return { hasError: true };
	}

	componentDidCatch(error: unknown): void {
		console.error(`[tap] Slide ${this.props.slideNumber} failed to render`, error);
	}

	render(): ReactNode {
		if (this.state.hasError) {
			if (isDevRuntime()) {
				const message = `Slide ${this.props.slideNumber} failed to render`;
				return <SlideErrorDisplay message={message} fallback={this.props.fallback} />;
			}
			return this.props.fallback;
		}
		return this.props.children;
	}
}
