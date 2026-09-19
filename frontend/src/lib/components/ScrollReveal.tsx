/**
 * Scroll reveal for long-content slides (`scroll: true`). Measures how far
 * the content extends past the visible slide area and translates it into
 * view as the presenter advances, animating the move with a CSS transition.
 * The first position it applies (on mount) never animates, matching how a
 * freshly-entered slide should not scroll on arrival.
 */

import { useLayoutEffect, useRef, type ReactNode } from 'react';
import { isPrintMode, prefersReducedMotion } from '../utils/transitions';

export interface ScrollRevealProps {
	/** Whether this slide has scroll reveal enabled (`slide.scroll === true`). */
	enabled: boolean;
	/** Whether the scroll position should be revealed (scrolled down). */
	revealed: boolean;
	/** Animation duration in milliseconds. */
	speed: number;
	/** Increments each time navigation triggers a scroll reveal. */
	triggerCount: number;
	children: ReactNode;
}

interface ScrollState {
	scrollDistance: number;
	measured: boolean;
	lastApplied: number | null;
}

export function ScrollReveal({ enabled, revealed, speed, triggerCount, children }: ScrollRevealProps) {
	const ref = useRef<HTMLDivElement>(null);
	const stateRef = useRef<ScrollState>({ scrollDistance: 0, measured: false, lastApplied: null });

	function apply(): void {
		const element = ref.current;
		const state = stateRef.current;
		if (!element || !state.measured) return;

		const needsScroll = state.scrollDistance > 0;
		const targetY = revealed && needsScroll ? state.scrollDistance : 0;
		const isFirstApplication = state.lastApplied === null;
		const isPositionChange = !isFirstApplication && targetY !== state.lastApplied;
		const shouldAnimate = isPositionChange && !prefersReducedMotion() && !isPrintMode();

		if (shouldAnimate) {
			element.style.transition = `transform ${speed}ms ease-in-out`;
			// Force a reflow so the browser picks up the transition before the
			// transform below changes, otherwise the move jumps instead of animating.
			void element.offsetHeight;
		} else {
			element.style.transition = 'none';
		}
		element.style.transform = `translateY(-${targetY}px)`;
		state.lastApplied = targetY;
	}

	useLayoutEffect(() => {
		if (!enabled) return;
		const element = ref.current;
		if (!element) return;

		function measure(): void {
			const parent = element?.parentElement;
			if (!element || !parent) return;
			// clientHeight includes the parent's padding, so measure against its
			// content-box height instead: some themes pad `.slide-content` to
			// reserve room for chrome, and subtracting the padded height would
			// leave the fully-scrolled position short by that padding.
			const parentStyle = window.getComputedStyle(parent);
			const verticalPadding = parseFloat(parentStyle.paddingTop) + parseFloat(parentStyle.paddingBottom);
			const parentContentHeight = parent.clientHeight - verticalPadding;
			stateRef.current.scrollDistance = Math.max(0, element.scrollHeight - parentContentHeight);
			stateRef.current.measured = true;
			apply();
		}

		measure();

		const resizeObserver = new ResizeObserver(measure);
		resizeObserver.observe(element);

		return () => resizeObserver.disconnect();
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [enabled]);

	useLayoutEffect(() => {
		if (!enabled) return;
		apply();
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [enabled, revealed, triggerCount, speed]);

	if (!enabled) {
		return <>{children}</>;
	}

	return (
		<div className="scroll-content" ref={ref}>
			{children}
		</div>
	);
}
