/**
 * Root component for the main slide viewer.
 * Loads the presentation, wires up keyboard navigation and hot reload, and
 * renders the current slide inside the scaled canvas, along with the
 * progress bar, the connection indicator, and the slide overview.
 */

import { useEffect, useRef, useState, type ReactNode } from 'react';
import {
	usePresentationStore,
	selectCurrentSlide,
	selectTotalSlides,
	loadPresentation,
	setupHashChangeListener
} from '$lib/stores/presentation';
import { useResolvedTheme } from '$lib/hooks/useResolvedTheme';
import {
	broadcastPresentationState,
	connectWebSocket,
	detectStaticMode,
	disconnectWebSocket
} from '$lib/stores/websocket';
import { setupKeyboardNavigation } from '$lib/utils/keyboard';
import { setupTouchNavigation } from '$lib/utils/touch';
import { fetchPresentation } from '$lib/utils/fetchPresentation';
import { SlideCanvas } from '$lib/components/SlideCanvas';
import { Slide } from '$lib/components/Slide';
import { SlideTransition } from '$lib/components/SlideTransition';
import { ProgressBar } from '$lib/components/ProgressBar';
import { ConnectionIndicator } from '$lib/components/ConnectionIndicator';
import { SlideOverview } from '$lib/components/SlideOverview';
import { ShortcutHelp } from '$lib/components/ShortcutHelp';
import { AUDIENCE_SHORTCUTS } from '$lib/utils/shortcuts';
import { resolveTransition, type TransitionDirection } from '$lib/utils/transitions';

const PRINT_MODE =
	typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('print') === 'true';

// A stepped or fragment screenshot (tap screenshot --step/--fragment) opens
// this live, non-print viewer so it can render an exact presenter state -
// print mode always shows the final step and fragment, which a stepped
// capture must not. It still runs against a temporary server that never
// serves the websocket route (see internal/cli/screenshot.go), so it must
// behave like print mode for the websocket: never connect, and never show
// the connection badge, or a capture bakes "Reconnecting..." into the PNG.
const CAPTURE_MODE =
	typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('capture') === 'true';

// `tap screenshot --wait <ms>` (see internal/pdf/capture.go) keeps a capture
// genuinely live - a running animation is exactly what --wait is for - so it
// skips the settled treatment below. Without --wait, the default, a capture
// is a still: `?live=true` is absent and the capture settles.
const CAPTURE_LIVE =
	typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('live') === 'true';

// Whether a deck component (and CSS-driven theme animations, via
// SlideCanvas's data-print attribute) should render its settled state
// instead of animating: true print (?print=true, which also forces the
// final step/fragment - see the step/fragmentIndex props below) and a
// non-live capture both want this; a live capture (--wait) does not, since
// the whole point of --wait is to catch a running animation partway
// through. Unlike print mode, a settled capture keeps the REQUESTED step
// and fragment - see SETTLE's callers below, and Slide.tsx's
// deckComponentPrintMode, which is this same OR applied at the slide level
// for the same reason.
const SETTLE = PRINT_MODE || (CAPTURE_MODE && !CAPTURE_LIVE);

/**
 * Tracks the direction of the most recent slide change by comparing the
 * current index to the previous render's. Read during render, updated in
 * the same render pass, so it is always current for the transition below.
 */
function useSlideDirection(currentIndex: number): TransitionDirection {
	const previousIndexRef = useRef(currentIndex);
	const directionRef = useRef<TransitionDirection>('forward');

	if (previousIndexRef.current !== currentIndex) {
		directionRef.current = currentIndex >= previousIndexRef.current ? 'forward' : 'backward';
		previousIndexRef.current = currentIndex;
	}

	return directionRef.current;
}

export default function App() {
	const [isLoading, setIsLoading] = useState(true);
	const [loadError, setLoadError] = useState<string | null>(null);
	const [overviewOpen, setOverviewOpen] = useState(false);
	const overviewOpenRef = useRef(overviewOpen);
	overviewOpenRef.current = overviewOpen;
	const [helpOpen, setHelpOpen] = useState(false);
	const helpOpenRef = useRef(helpOpen);
	helpOpenRef.current = helpOpen;

	const presentation = usePresentationStore((state) => state.presentation);
	const currentSlide = usePresentationStore(selectCurrentSlide);
	const currentSlideIndex = usePresentationStore((state) => state.currentSlideIndex);
	const totalSlides = usePresentationStore(selectTotalSlides);
	const currentFragmentIndex = usePresentationStore((state) => state.currentFragmentIndex);
	const currentStep = usePresentationStore((state) => state.currentStep);
	const scrollRevealed = usePresentationStore((state) => state.scrollRevealed);
	const scrollTriggerCount = usePresentationStore((state) => state.scrollTriggerCount);
	const direction = useSlideDirection(currentSlideIndex);

	const resolvedTheme = useResolvedTheme();
	const theme = resolvedTheme?.slug ?? 'base';
	const aspectRatio = presentation?.config?.aspectRatio ?? '16:9';
	const themeColors = presentation?.config?.themeColors;
	const customTheme = presentation?.config?.customTheme;
	const showProgressBar = presentation?.config?.showProgressBar !== false;
	const transition = resolveTransition(currentSlide?.transition, presentation?.config?.transition);

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
		const keyboardCleanup = setupKeyboardNavigation({
			onNavigate: broadcastPresentationState,
			onToggleOverview: () => setOverviewOpen((open) => !open),
			isOverviewOpen: () => overviewOpenRef.current,
			onToggleHelp: () => setHelpOpen((open) => !open),
			isHelpOpen: () => helpOpenRef.current
		});
		// A phone or tablet has no keyboard, so a horizontal swipe is the only
		// way to move between slides there.
		const touchCleanup = setupTouchNavigation({
			onNavigate: broadcastPresentationState,
			onToggleOverview: () => setOverviewOpen((open) => !open),
			isOverviewOpen: () => overviewOpenRef.current,
			isHelpOpen: () => helpOpenRef.current
		});

		// A print pass (PDF export, ?print=true) is a static snapshot of one
		// slide: it never connects the websocket, so it can never have the
		// hub's live state applied out from under the screenshot. A static
		// build has no server either, so the websocket must never be opened
		// (and never retried) once static mode is confirmed. A stepped or
		// fragment capture (?capture=true) is not a static snapshot, but it
		// must never connect either - see CAPTURE_MODE above.
		if (!PRINT_MODE && !CAPTURE_MODE) {
			void detectStaticMode().then((isStatic) => {
				if (!cancelled && !isStatic) {
					connectWebSocket();
				}
			});
		}

		return () => {
			cancelled = true;
			hashCleanup();
			keyboardCleanup();
			touchCleanup();
			disconnectWebSocket();
		};
	}, []);

	useEffect(() => {
		document.title = presentation?.config?.title ?? 'Tap Presentation';
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
			<>
				<SlideCanvas
					aspectRatio={aspectRatio}
					theme={theme}
					themeColors={themeColors}
					printMode={SETTLE}
				>
					<SlideTransition
						slideKey={currentSlideIndex}
						transition={transition}
						direction={direction}
						printMode={SETTLE}
					>
						<Slide
							slide={currentSlide}
							active
							printMode={PRINT_MODE}
							settleComponents={SETTLE}
							fragmentIndex={PRINT_MODE ? currentSlide.fragmentCount : currentFragmentIndex}
							step={PRINT_MODE ? currentSlide.steps : currentStep}
							total={totalSlides}
							scrollRevealed={scrollRevealed}
							scrollTriggerCount={scrollTriggerCount}
							mermaidOverrides={resolvedTheme?.mermaid}
						/>
					</SlideTransition>
				</SlideCanvas>

				<ProgressBar show={showProgressBar} />

				{!PRINT_MODE && !CAPTURE_MODE ? <ConnectionIndicator /> : null}

				<SlideOverview
					slides={presentation.slides}
					theme={theme}
					aspectRatio={aspectRatio}
					isOpen={overviewOpen}
					onClose={() => setOverviewOpen(false)}
				/>

				<ShortcutHelp groups={AUDIENCE_SHORTCUTS} theme={theme} isOpen={helpOpen} onClose={() => setHelpOpen(false)} />
			</>
		);
	}

	return <div className="slide-renderer">{content}</div>;
}
