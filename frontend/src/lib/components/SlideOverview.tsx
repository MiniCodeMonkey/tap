/**
 * Full-screen grid of every slide, opened with the O key or a two-finger
 * tap. Each thumbnail is a real Slide rendered as a static preview inside a
 * scaled SlideCanvas, so it always matches what the audience view would
 * show. Arrow keys move a focus ring around the grid; Enter or a click
 * jumps to that slide.
 *
 * Only the thumbnails near the viewport render their slide. A long deck of
 * component-driven slides would otherwise mount every map, chart and
 * highlighted code block at once, which is enough to take a phone down.
 */

import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import type { Slide as SlideData, Theme } from '$lib/types';
import { usePresentationStore, goToSlide } from '$lib/stores/presentation';
import { broadcastPresentationState } from '$lib/stores/websocket';
import { SlideCanvas } from './SlideCanvas';
import { Slide } from './Slide';

export interface SlideOverviewProps {
	/** Every slide in the presentation, shown as thumbnails. */
	slides: SlideData[];
	/** Current theme name. */
	theme?: Theme;
	/** Aspect ratio shared with the main viewer, so thumbnails match its shape. */
	aspectRatio?: string;
	/** Whether the overview is currently visible. */
	isOpen?: boolean;
	/** Called after a slide is selected, before the overview closes. */
	onSelect?: (index: number) => void;
	/** Called when the overview should close. */
	onClose?: () => void;
}

/** Number of columns in the grid at its widest, matching the CSS default. */
const GRID_COLUMNS = 5;

/**
 * How far outside the viewport a thumbnail starts rendering its slide, so a
 * scroll lands on a drawn thumbnail rather than an empty box.
 */
const RENDER_MARGIN = '300px';

/**
 * A thumbnail that mounts its children only while it is near the viewport,
 * and drops them again once it is well clear of it. Without an
 * IntersectionObserver (jsdom, older browsers) it renders everything, which
 * is the behaviour this replaced.
 */
function LazyThumbnail({ children }: { children: ReactNode }) {
	const ref = useRef<HTMLDivElement>(null);
	const [visible, setVisible] = useState(typeof IntersectionObserver === 'undefined');

	useEffect(() => {
		if (typeof IntersectionObserver === 'undefined') {
			return;
		}

		const element = ref.current;
		if (!element) {
			return;
		}

		const observer = new IntersectionObserver(
			(entries) => {
				const entry = entries[0];
				if (entry) {
					setVisible(entry.isIntersecting);
				}
			},
			{ rootMargin: RENDER_MARGIN }
		);

		observer.observe(element);
		return () => observer.disconnect();
	}, []);

	return (
		<div className="thumbnail-aspect" ref={ref}>
			{visible ? children : null}
		</div>
	);
}

export function SlideOverview({
	slides,
	theme = 'base' as Theme,
	aspectRatio = '16:9',
	isOpen = false,
	onSelect,
	onClose
}: SlideOverviewProps) {
	const currentIndex = usePresentationStore((state) => state.currentSlideIndex);
	const [focusedIndex, setFocusedIndex] = useState(currentIndex);
	const containerRef = useRef<HTMLDivElement>(null);

	// Reset the focus ring to the current slide each time the overview opens.
	useEffect(() => {
		if (isOpen) {
			setFocusedIndex(currentIndex);
			containerRef.current?.focus();
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [isOpen]);

	if (!isOpen) {
		return null;
	}

	function selectSlide(index: number): void {
		goToSlide(index);
		// Mirror keyboard navigation, which broadcasts after every navigation
		// action - without this, jumping to a slide from the overview never
		// reached other connected clients.
		broadcastPresentationState();
		onSelect?.(index);
		onClose?.();
	}

	function handleKeyDown(event: KeyboardEvent<HTMLDivElement>): void {
		switch (event.key) {
			case 'ArrowRight':
				event.preventDefault();
				setFocusedIndex((index) => Math.min(index + 1, slides.length - 1));
				break;
			case 'ArrowLeft':
				event.preventDefault();
				setFocusedIndex((index) => Math.max(index - 1, 0));
				break;
			case 'ArrowDown':
				event.preventDefault();
				setFocusedIndex((index) => Math.min(index + GRID_COLUMNS, slides.length - 1));
				break;
			case 'ArrowUp':
				event.preventDefault();
				setFocusedIndex((index) => Math.max(index - GRID_COLUMNS, 0));
				break;
			case 'Enter':
			case ' ':
				event.preventDefault();
				selectSlide(focusedIndex);
				break;
			case 'Escape':
				event.preventDefault();
				onClose?.();
				break;
			case 'Home':
				event.preventDefault();
				setFocusedIndex(0);
				break;
			case 'End':
				event.preventDefault();
				setFocusedIndex(slides.length - 1);
				break;
		}
	}

	return (
		<div
			className="slide-overview"
			role="dialog"
			aria-label="Slide overview"
			aria-modal="true"
			tabIndex={0}
			ref={containerRef}
			onKeyDown={handleKeyDown}
		>
			<div className="overview-backdrop" role="presentation" onClick={() => onClose?.()} />

			<div className="overview-content">
				<div
					className="thumbnail-grid"
					style={{ ['--grid-columns' as string]: GRID_COLUMNS }}
					role="listbox"
					aria-label="Select a slide"
				>
					{slides.map((slide, index) => (
						<button
							key={slide.index}
							className={`thumbnail${index === currentIndex ? ' current' : ''}${index === focusedIndex ? ' focused' : ''}`}
							onClick={() => selectSlide(index)}
							role="option"
							aria-selected={index === currentIndex}
							aria-label={`Slide ${index + 1}`}
						>
							<LazyThumbnail>
								<SlideCanvas aspectRatio={aspectRatio} theme={theme}>
									<Slide
										slide={slide}
										active={false}
										printMode
										preview
										fragmentIndex={slide.fragmentCount}
										step={slide.steps}
										total={slides.length}
									/>
								</SlideCanvas>
							</LazyThumbnail>
							<div className="thumbnail-number">{index + 1}</div>
						</button>
					))}
				</div>
			</div>
		</div>
	);
}
