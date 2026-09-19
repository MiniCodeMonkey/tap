/**
 * Catches a render failure in one slide's layout so the rest of the
 * presentation keeps working. In a server-backed runtime (`tap dev`, `tap
 * pdf`, `tap screenshot`, or the frontend's own dev server - see
 * isDevRuntime) it shows an error card naming the slide, so a broken slide
 * is visible as broken instead of silently falling back; in a static `tap
 * build` output it falls back to the slide's raw slot content instead of
 * going blank.
 */

import { Component, type ReactNode } from 'react';
import { isDevRuntime, shouldUseSafeErrorForm } from '$lib/utils/runtime';

interface SlideErrorBoundaryProps {
	slideNumber: number;
	fallback: ReactNode;
	children: ReactNode;
}

interface SlideErrorBoundaryState {
	hasError: boolean;
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
				if (shouldUseSafeErrorForm()) {
					return (
						<>
							<div className="slide-error deck-error-card deck-error-card-safe" data-message={message} hidden />
							<div className="deck-error-marker" title={message}>
								component error
							</div>
						</>
					);
				}
				return <div className="slide-error deck-error-card">{message}</div>;
			}
			return this.props.fallback;
		}
		return this.props.children;
	}
}
