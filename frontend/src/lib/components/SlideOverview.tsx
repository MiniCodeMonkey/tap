/**
 * Full-screen grid of every slide, opened with the O key or a two-finger
 * tap. Each thumbnail is a real Slide rendered as a static preview inside a
 * scaled SlideCanvas, so it always matches what the audience view would
 * show. It opens scrolled to the current slide, which stays highlighted.
 * Arrow keys move a focus ring around the grid, scrolling to keep it in
 * view; Enter or a click jumps to that slide.
 *
 * Only the thumbnails near the viewport render their slide. A long deck of
 * component-driven slides would otherwise mount every map, chart and
 * highlighted code block at once, which is enough to take a phone down.
 */

import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import type { Slide as SlideData, Theme } from '$lib/types';
import { usePresentationStore, goToSlide, slideKey } from '$lib/stores/presentation';
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

/**
 * Columns to move by on ArrowUp/ArrowDown before the grid has been measured,
 * and wherever getComputedStyle cannot report the tracks.
 */
const FALLBACK_COLUMNS = 5;

/**
 * Count the grid's columns as the browser actually laid them out.
 * The column count belongs to the stylesheet, which varies it by viewport
 * width and by whether the pointer is a finger, so it cannot be a constant
 * here: an inline `--grid-columns` would override every one of those rules.
 */
function countColumns(grid: HTMLElement | null): number {
	if (!grid || typeof window === 'undefined') {
		return FALLBACK_COLUMNS;
	}

	const tracks = window.getComputedStyle(grid).gridTemplateColumns;
	if (!tracks || tracks === 'none') {
		return FALLBACK_COLUMNS;
	}

	return tracks.split(' ').filter(Boolean).length || FALLBACK_COLUMNS;
}

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
	const [wasOpen, setWasOpen] = useState(isOpen);
	const containerRef = useRef<HTMLDivElement>(null);
	const gridRef = useRef<HTMLDivElement>(null);
	const [columns, setColumns] = useState(FALLBACK_COLUMNS);

	// Keep the arrow keys moving by whatever the stylesheet laid out, which
	// changes with the window width and on a rotated phone.
	useEffect(() => {
		if (!isOpen) {
			return;
		}

		const measure = (): void => setColumns(countColumns(gridRef.current));
		measure();

		window.addEventListener('resize', measure);
		window.addEventListener('orientationchange', measure);
		return () => {
			window.removeEventListener('resize', measure);
			window.removeEventListener('orientationchange', measure);
		};
	}, [isOpen]);

	// Reset the focus ring to the current slide each time the overview opens.
	// This happens during render, not in an effect, so the first committed
	// frame already has it there; an effect would leave the scroll effect
	// below one render holding the previous opening's focus position.
	if (isOpen !== wasOpen) {
		setWasOpen(isOpen);
		if (isOpen) {
			setFocusedIndex(currentIndex);
		}
	}

	useEffect(() => {
		if (isOpen) {
			containerRef.current?.focus();
		}
	}, [isOpen]);

	// The grid unmounts while the overview is closed, so each opening starts
	// scrolled to the top. Center the current slide before the first paint,
	// so the overview never shows slide 1 and then jumps.
	useLayoutEffect(() => {
		if (!isOpen) {
			return;
		}
		const current = gridRef.current?.children[currentIndex];
		current?.scrollIntoView?.({ block: 'center', inline: 'nearest' });
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [isOpen]);

	// Keep the focus ring on screen as the arrow keys move it past the edge.
	useEffect(() => {
		if (!isOpen) {
			return;
		}
		const focused = gridRef.current?.children[focusedIndex];
		focused?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' });
	}, [isOpen, focusedIndex]);

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
				setFocusedIndex((index) => Math.min(index + columns, slides.length - 1));
				break;
			case 'ArrowUp':
				event.preventDefault();
				setFocusedIndex((index) => Math.max(index - columns, 0));
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
				<div className="thumbnail-grid" ref={gridRef} role="listbox" aria-label="Select a slide">
					{slides.map((slide, index) => (
						<button
							key={slideKey(slide)}
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
