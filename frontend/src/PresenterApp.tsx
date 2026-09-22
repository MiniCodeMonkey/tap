/**
 * Root component for the presenter view, served at /presenter. Shows a
 * compact timer and slide counter, a preview of the current and next
 * slides, and the current slide's speaker notes, with touch-friendly
 * controls for advancing the presentation.
 *
 * Named PresenterApp rather than Presenter because this project's default
 * filesystem is case-insensitive, so a file named Presenter.tsx would
 * collide with the sibling entry point presenter.tsx.
 *
 * The current slide panel mirrors exactly what the audience sees: the same
 * live fragment index and step, with the map, live code and rich content
 * (syntax highlighting, mermaid) all running, so the speaker can read code
 * and diagrams as clearly as the audience does. It never animates a slide
 * transition. The next slide panel shows that slide's initial state (no
 * fragments revealed yet) as a static, non-interactive preview, the same
 * cheap path the overview thumbnails use: no map, no live code, no rich
 * content processing, since it is only a look-ahead.
 *
 * Layout: the current slide takes the wider left column; the next slide
 * and the speaker notes share the right column, with the notes filling
 * whatever height the next slide leaves. The notes font size is adjustable
 * (the - and = keys, or the A- / A+ buttons) and persists in localStorage.
 */

import { useEffect, useRef, useState, type ReactNode } from 'react';
import {
	usePresentationStore,
	selectCurrentSlide,
	selectTotalSlides,
	loadPresentation,
	setupHashChangeListener,
	nextSlide,
	prevSlide,
	goToSlide,
	slideKey
} from '$lib/stores/presentation';
import { useResolvedTheme } from '$lib/hooks/useResolvedTheme';
import {
	broadcastPresentationState,
	connectWebSocket,
	detectStaticMode,
	disconnectWebSocket,
	useConnectionStore
} from '$lib/stores/websocket';
import { fetchPresentation } from '$lib/utils/fetchPresentation';
import { setupWakeLock } from '$lib/utils/wakeLock';
import { SlideCanvas } from '$lib/components/SlideCanvas';
import { Slide } from '$lib/components/Slide';
import { ShortcutHelp } from '$lib/components/ShortcutHelp';
import { DiskIndicator } from '$lib/components/DiskIndicator';
import { PresenterLayoutMenu } from '$lib/components/PresenterLayoutMenu';
import { usePresenterLayout } from '$lib/hooks/usePresenterLayout';
import { useFitText } from '$lib/hooks/useFitText';
import {
	NOTES_FIT_MAX_SIZE,
	NOTES_FIT_MIN_SIZE,
	NOTES_FIT_SCALE_MAX,
	NOTES_FIT_SCALE_MIN,
	NOTES_FIT_SCALE_STEP,
	clampFitScale,
	readStoredFitScale,
	writeStoredFitScale
} from '$lib/utils/fitText';
import { HELP_KEY, PRESENTER_SHORTCUTS } from '$lib/utils/shortcuts';

/** True when exported to PDF via the "both" content option (`/presenter?print=true`). */
const PRINT_MODE =
	typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('print') === 'true';

/** Whether the currently focused element is one the presenter is typing into. */
function isInputFocused(): boolean {
	const target = document.activeElement;
	if (!target) return false;
	const tagName = target.tagName;
	return (
		tagName === 'INPUT' ||
		tagName === 'TEXTAREA' ||
		tagName === 'SELECT' ||
		target.getAttribute('contenteditable') === 'true'
	);
}

const NOTES_FONT_SIZE_STORAGE_KEY = 'tap-presenter-notes-font-size';
/** Speaker notes font size bounds and step, in rem. */
const NOTES_FONT_SIZE_DEFAULT = 1.5;
const NOTES_FONT_SIZE_MIN = 1;
const NOTES_FONT_SIZE_MAX = 3;
const NOTES_FONT_SIZE_STEP = 0.125;

function clampNotesFontSize(size: number): number {
	return Math.min(NOTES_FONT_SIZE_MAX, Math.max(NOTES_FONT_SIZE_MIN, size));
}

function readNotesFontSize(): number {
	try {
		const stored = Number.parseFloat(window.localStorage.getItem(NOTES_FONT_SIZE_STORAGE_KEY) ?? '');
		return Number.isFinite(stored) ? clampNotesFontSize(stored) : NOTES_FONT_SIZE_DEFAULT;
	} catch {
		return NOTES_FONT_SIZE_DEFAULT;
	}
}

function writeNotesFontSize(size: number): void {
	try {
		window.localStorage.setItem(NOTES_FONT_SIZE_STORAGE_KEY, String(size));
	} catch {
		// Storage can be unavailable (private window, blocked site data); the
		// size then lasts only until reload.
	}
}

/** "16:9" to the CSS aspect-ratio value "16 / 9". */
function cssAspectRatio(aspectRatio: string): string {
	const [width, height] = aspectRatio.split(':');
	return width && height ? `${width} / ${height}` : '16 / 9';
}

function formatTime(seconds: number): string {
	const hours = Math.floor(seconds / 3600);
	const minutes = Math.floor((seconds % 3600) / 60);
	const secs = seconds % 60;
	if (hours > 0) {
		return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
	}
	return `${minutes}:${secs.toString().padStart(2, '0')}`;
}

export default function PresenterApp() {
	const [isLoading, setIsLoading] = useState(true);
	const [loadError, setLoadError] = useState<string | null>(null);
	const [elapsedSeconds, setElapsedSeconds] = useState(0);
	const [helpOpen, setHelpOpen] = useState(false);
	const helpOpenRef = useRef(helpOpen);
	helpOpenRef.current = helpOpen;
	const [notesFontSize, setNotesFontSize] = useState(readNotesFontSize);
	// In fit mode the size comes from the panel, and this scales that result:
	// A- and A+ ask for a notch smaller rather than an absolute size.
	const [notesFitScale, setNotesFitScale] = useState(readStoredFitScale);
	const [layoutMenuOpen, setLayoutMenuOpen] = useState(false);
	const layoutMenuOpenRef = useRef(layoutMenuOpen);
	layoutMenuOpenRef.current = layoutMenuOpen;
	const notesContentRef = useRef<HTMLDivElement | null>(null);

	const presentation = usePresentationStore((state) => state.presentation);
	const currentSlide = usePresentationStore(selectCurrentSlide);
	const currentSlideIndex = usePresentationStore((state) => state.currentSlideIndex);
	const totalSlides = usePresentationStore(selectTotalSlides);
	const currentFragmentIndex = usePresentationStore((state) => state.currentFragmentIndex);
	const currentStep = usePresentationStore((state) => state.currentStep);
	const scrollRevealed = usePresentationStore((state) => state.scrollRevealed);
	const scrollTriggerCount = usePresentationStore((state) => state.scrollTriggerCount);
	const connected = useConnectionStore((state) => state.connected);

	const presenterLayout = usePresenterLayout(presentation?.config?.presenterLayout);
	const { layout, notesSizeMode } = presenterLayout;

	// The keyboard handler is installed once, so it reaches the current layout
	// state through a ref rather than through its closure.
	const presenterLayoutRef = useRef(presenterLayout);
	presenterLayoutRef.current = presenterLayout;
	const notesSizeModeRef = useRef(notesSizeMode);
	notesSizeModeRef.current = notesSizeMode;

	const showCurrentSlide = layout !== 'notes-only';
	const showNextSlide = layout !== 'slide-only' && layout !== 'notes-only';
	const showNotes = layout !== 'slide-only';

	const resolvedTheme = useResolvedTheme();
	const theme = resolvedTheme?.slug ?? 'base';
	const aspectRatio = presentation?.config?.aspectRatio ?? '16:9';
	const customTheme = presentation?.config?.customTheme;
	const nextSlideData =
		presentation && currentSlideIndex < presentation.slides.length - 1
			? presentation.slides[currentSlideIndex + 1]
			: null;
	const fragmentCount = currentSlide?.fragmentCount ?? 0;

	const fittedNotesFontSize = useFitText({
		enabled: notesSizeMode === 'fit',
		elementRef: notesContentRef,
		capSize: NOTES_FIT_MAX_SIZE,
		minSize: NOTES_FIT_MIN_SIZE,
		step: NOTES_FONT_SIZE_STEP,
		contentKey: `${currentSlide?.index ?? -1}:${layout}`
	});

	const fitting = notesSizeMode === 'fit';
	const activeNotesSize = fitting ? notesFitScale : notesFontSize;
	const activeNotesSizeMin = fitting ? NOTES_FIT_SCALE_MIN : NOTES_FONT_SIZE_MIN;
	const activeNotesSizeMax = fitting ? NOTES_FIT_SCALE_MAX : NOTES_FONT_SIZE_MAX;
	const appliedNotesFontSize = fitting
		? Math.max(NOTES_FIT_MIN_SIZE, fittedNotesFontSize * notesFitScale)
		: notesFontSize;

	const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

	function resetTimer(): void {
		setElapsedSeconds(0);
	}

	function handleNextSlide(): void {
		nextSlide();
		broadcastPresentationState();
	}

	function handlePrevSlide(): void {
		prevSlide();
		broadcastPresentationState();
	}

	// A- and A+ move whichever size is in play: the size read at in manual
	// mode, the ceiling fitting may grow to in fit mode.
	function changeNotesFontSize(delta: number): void {
		if (notesSizeModeRef.current === 'fit') {
			const step = delta < 0 ? -NOTES_FIT_SCALE_STEP : NOTES_FIT_SCALE_STEP;
			setNotesFitScale((scale) => {
				const next = clampFitScale(scale + step);
				writeStoredFitScale(next);
				return next;
			});
			return;
		}
		setNotesFontSize((size) => {
			const next = clampNotesFontSize(size + delta);
			writeNotesFontSize(next);
			return next;
		});
	}

	useEffect(() => {
		let cancelled = false;

		async function load(): Promise<void> {
			try {
				const data = await fetchPresentation();
				if (cancelled) return;
				loadPresentation(data);
				setIsLoading(false);
			} catch (error) {
				if (cancelled) return;
				setLoadError(error instanceof Error ? error.message : 'Failed to load presentation');
				setIsLoading(false);
			}
		}

		void load();

		const hashCleanup = setupHashChangeListener();

		// A print pass (PDF export, ?print=true) is a static snapshot of one
		// slide: it never connects the websocket, so it can never have the
		// hub's live state applied out from under the screenshot. A static
		// build has no server either, so the websocket must never be opened
		// (and never retried) once static mode is confirmed.
		if (!PRINT_MODE) {
			void detectStaticMode().then((isStatic) => {
				if (!cancelled && !isStatic) {
					connectWebSocket();
				}
			});
		}

		timerRef.current = setInterval(() => {
			setElapsedSeconds((seconds) => seconds + 1);
		}, 1000);

		function handleKeyDown(event: KeyboardEvent): void {
			if (isInputFocused()) return;

			// Escape closes the layout menu before anything else looks at the key.
			if (layoutMenuOpenRef.current && event.key === 'Escape') {
				event.preventDefault();
				setLayoutMenuOpen(false);
				return;
			}

			// While the shortcut overlay is open, only ? and Escape reach it
			// (to close it); every other key is ignored.
			if (helpOpenRef.current) {
				if (event.key === HELP_KEY || event.key === 'Escape') {
					event.preventDefault();
					setHelpOpen(false);
				}
				return;
			}

			switch (event.key) {
				case HELP_KEY:
					event.preventDefault();
					setHelpOpen(true);
					break;
				case 'ArrowRight':
				case 'ArrowDown':
				case ' ':
				case 'Enter':
				case 'PageDown':
					event.preventDefault();
					handleNextSlide();
					break;
				case 'ArrowLeft':
				case 'ArrowUp':
				case 'Backspace':
				case 'PageUp':
					event.preventDefault();
					handlePrevSlide();
					break;
				case 'Home':
					event.preventDefault();
					goToSlide(0);
					broadcastPresentationState();
					break;
				case 'End': {
					event.preventDefault();
					const total = selectTotalSlides(usePresentationStore.getState());
					if (total > 0) {
						goToSlide(total - 1);
					}
					broadcastPresentationState();
					break;
				}
				case 'r':
				case 'R':
					event.preventDefault();
					resetTimer();
					break;
				case '-':
				case '_':
					event.preventDefault();
					changeNotesFontSize(-NOTES_FONT_SIZE_STEP);
					break;
				case '=':
				case '+':
					event.preventDefault();
					changeNotesFontSize(NOTES_FONT_SIZE_STEP);
					break;
				case 'v':
				case 'V':
					event.preventDefault();
					presenterLayoutRef.current.cycleLayout();
					break;
				case '1':
				case '2':
				case '3':
				case '4':
				case '5': {
					// Digits pick a layout only while the menu is open, leaving them
					// free for jumping to a slide by number.
					if (!layoutMenuOpenRef.current) break;
					event.preventDefault();
					const chosen = presenterLayoutRef.current.availableLayouts[Number(event.key) - 1];
					if (chosen) {
						presenterLayoutRef.current.setLayout(chosen.id);
						setLayoutMenuOpen(false);
					}
					break;
				}
			}
		}

		// Presenting from a phone or a propped-up laptop: keep the screen on.
		const wakeLock = setupWakeLock();
		window.addEventListener('keydown', handleKeyDown);

		return () => {
			cancelled = true;
			hashCleanup();
			disconnectWebSocket();
			if (timerRef.current) {
				clearInterval(timerRef.current);
				timerRef.current = null;
			}
			wakeLock.release();
			window.removeEventListener('keydown', handleKeyDown);
		};
	}, []);

	useEffect(() => {
		document.title = presentation?.config?.title ? `${presentation.config.title} - Presenter` : 'Tap Presenter';
	}, [presentation?.config?.title]);

	useEffect(() => {
		if (!customTheme) {
			return;
		}

		const link = document.createElement('link');
		link.rel = 'stylesheet';
		link.type = 'text/css';
		link.href = `/api/custom-theme.css?t=${Date.now()}`;
		link.id = 'custom-theme-css';
		link.onerror = () => {
			console.warn('[tap] Custom theme CSS failed to load. Using default theme.');
		};
		document.head.appendChild(link);

		return () => {
			link.remove();
		};
	}, [customTheme]);

	let content: ReactNode;

	if (isLoading) {
		content = (
			<div className="loading-container">
				<div className="loading-spinner" />
				<p>Loading presentation...</p>
			</div>
		);
	} else if (loadError) {
		content = (
			<div className="error-container">
				<h1>Error</h1>
				<p>{loadError}</p>
				<button onClick={() => window.location.reload()}>Reload</button>
			</div>
		);
	} else if (!presentation || !currentSlide) {
		content = (
			<div className="empty-container">
				<h1>No Presentation</h1>
				<p>No presentation data available.</p>
			</div>
		);
	} else {
		content = (
			<div className="presenter-view" data-presenter-layout={layout} data-notes-size={notesSizeMode}>
				<header className="presenter-header">
					<div className="presenter-slide-counter">
						<span className="current">{currentSlideIndex + 1}</span>
						<span className="separator">/</span>
						<span className="total">{totalSlides}</span>
						{fragmentCount > 0 ? (
							<span className="fragment-counter">
								({currentFragmentIndex + 1}/{fragmentCount})
							</span>
						) : null}
					</div>

					<button
						className="presenter-timer"
						onClick={resetTimer}
						title="Click to reset timer"
						aria-label={`Elapsed time: ${formatTime(elapsedSeconds)}. Click to reset.`}
					>
						{formatTime(elapsedSeconds)}
					</button>

					<PresenterLayoutMenu
						layout={presenterLayout.layout}
						availableLayouts={presenterLayout.availableLayouts}
						onSelectLayout={presenterLayout.setLayout}
						notesSizeMode={presenterLayout.notesSizeMode}
						onSelectNotesSizeMode={presenterLayout.setNotesSizeMode}
						isNarrow={presenterLayout.isNarrow}
						isOpen={layoutMenuOpen}
						onOpenChange={setLayoutMenuOpen}
					/>

					{/*
					 * Scanning the dev server's QR code lands here, on the
					 * notes and the controls. This is the way back out to the
					 * deck itself, on the same host, so it works over a
					 * tunnel as well as over localhost.
					 */}
					<a className="presenter-slides-link" href="/" title="Open the slides in this window">
						Slides
					</a>

					<div className={`presenter-connection-status${connected ? ' connected' : ''}`}>
						{connected ? 'Connected' : 'Disconnected'}
					</div>
				</header>

				<main
					className="presenter-main"
					style={{ ['--presenter-aspect-ratio' as string]: cssAspectRatio(aspectRatio) }}
				>
					{showCurrentSlide ? (
						<section className="presenter-current-slide-panel">
							<h2 className="presenter-panel-title">Current Slide{currentSlide.scroll ? ' (Scroll)' : ''}</h2>
							<div className="presenter-slide-preview current">
								<SlideCanvas aspectRatio={aspectRatio} theme={theme} printMode={PRINT_MODE}>
									<Slide
										key={currentSlide.index}
										slide={currentSlide}
										active
										printMode={PRINT_MODE}
										fragmentIndex={PRINT_MODE ? currentSlide.fragmentCount : currentFragmentIndex}
										step={PRINT_MODE ? currentSlide.steps : currentStep}
										total={totalSlides}
										scrollRevealed={scrollRevealed}
										scrollTriggerCount={scrollTriggerCount}
										mermaidOverrides={resolvedTheme?.mermaid}
									/>
								</SlideCanvas>
							</div>
						</section>
					) : null}

					{showNextSlide ? (
						<section className="presenter-next-slide-panel">
							<h2 className="presenter-panel-title">Next Slide</h2>
							<div className="presenter-slide-preview next">
								{nextSlideData ? (
									<SlideCanvas aspectRatio={aspectRatio} theme={theme} printMode={PRINT_MODE}>
										<Slide
											key={slideKey(nextSlideData)}
											slide={nextSlideData}
											active={false}
											printMode={false}
											preview
											fragmentIndex={-1}
											step={0}
											total={totalSlides}
										/>
									</SlideCanvas>
								) : (
									<div className="presenter-end-placeholder">End of Presentation</div>
								)}
							</div>
						</section>
					) : null}

					{showNotes ? (
						<section className={`presenter-notes-panel${currentSlide.notes ? ' has-notes' : ''}`}>
							<div className="presenter-notes-header">
								<h2 className="presenter-panel-title">Speaker Notes</h2>
								<div className="presenter-notes-font-controls">
									<button
										type="button"
										className="presenter-notes-font-button"
										onClick={() => changeNotesFontSize(-NOTES_FONT_SIZE_STEP)}
										disabled={activeNotesSize <= activeNotesSizeMin}
										aria-label="Smaller speaker notes"
										title="Smaller notes (-)"
									>
										A-
									</button>
									<button
										type="button"
										className="presenter-notes-font-button"
										onClick={() => changeNotesFontSize(NOTES_FONT_SIZE_STEP)}
										disabled={activeNotesSize >= activeNotesSizeMax}
										aria-label="Larger speaker notes"
										title="Larger notes (=)"
									>
										A+
									</button>
								</div>
							</div>
							<div
								className="presenter-notes-content"
								ref={notesContentRef}
								style={{ fontSize: `${appliedNotesFontSize}rem` }}
							>
								{currentSlide.notes ? (
									<div dangerouslySetInnerHTML={{ __html: currentSlide.notes }} />
								) : (
									<p className="presenter-no-notes">No speaker notes for this slide.</p>
								)}
							</div>
						</section>
					) : null}
				</main>

				<footer className="presenter-controls">
					<button
						className="presenter-control-button prev"
						onClick={handlePrevSlide}
						disabled={currentSlideIndex === 0 && currentFragmentIndex < 0}
						aria-label="Previous slide"
					>
						<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
							<polyline points="15 18 9 12 15 6" />
						</svg>
						<span>Previous</span>
					</button>

					<div className="presenter-control-info">
						<span className="presenter-keyboard-hint">Use arrow keys or space to navigate</span>
						<span className="presenter-keyboard-hint">Press R to reset timer, ? for all shortcuts</span>
					</div>

					<button
						className="presenter-control-button next"
						onClick={handleNextSlide}
						disabled={currentSlideIndex === totalSlides - 1 && currentFragmentIndex >= fragmentCount - 1}
						aria-label="Next slide"
					>
						<span>Next</span>
						<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
							<polyline points="9 18 15 12 9 6" />
						</svg>
					</button>
				</footer>

				<DiskIndicator />
				<ShortcutHelp groups={PRESENTER_SHORTCUTS} theme={theme} isOpen={helpOpen} onClose={() => setHelpOpen(false)} />
			</div>
		);
	}

	return <div className="slide-renderer">{content}</div>;
}
