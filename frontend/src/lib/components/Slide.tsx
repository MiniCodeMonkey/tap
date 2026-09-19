/**
 * Renders one slide: picks its layout, provides fragment context to every
 * slot, wraps scroll-reveal slides in ScrollReveal, and isolates layout
 * render failures behind an error boundary.
 */

import { useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import { createPortal } from 'react-dom';
import type { BackgroundConfig, Slide as SlideData } from '$lib/types';
import { resolveLayout } from '../layouts/registry';
import { usePresentationStore } from '../stores/presentation';
import { useRichBlocks, type DeckComponentPortal, type LiveCodeBlockPortal } from '../hooks/useRichBlocks';
import type { MermaidThemeOverrides } from '../utils/mermaid';
import { parseMapConfig } from '../utils/map';
import { DeckComponent } from './DeckComponent';
import { LiveCodeBlock } from './LiveCodeBlock';
import { MapSlide } from './MapSlide';
import { ScrollReveal } from './ScrollReveal';
import { Slot } from './Slot';
import { SlideContext } from './SlideContext';
import { SlideErrorBoundary } from './SlideErrorBoundary';

export interface SlideProps {
	slide: SlideData;
	active: boolean;
	printMode: boolean;
	fragmentIndex: number;
	step: number;
	/** Total slide count, shown as data-total on the slide root. */
	total: number;
	/** Whether this slide's scroll reveal is currently scrolled into view. */
	scrollRevealed?: boolean;
	/** Increments each time navigation triggers this slide's scroll reveal. */
	scrollTriggerCount?: number;
	/**
	 * True for a static, non-interactive rendering of the slide: the overview
	 * grid and the presenter view's current/next panels. A preview never
	 * mounts the map (a WebGL context per thumbnail is too expensive to run
	 * for every slide at once) and skips mermaid, Shiki and live code block
	 * processing, since that output is illegible at thumbnail scale and would
	 * otherwise run once per thumbnail on every open of the overview. Since
	 * skipping rich blocks also skips the step that normally removes a map
	 * code block's raw `<pre>`, a preview strips it directly instead.
	 * Fragment and step visibility is unaffected, since that comes from
	 * `printMode` through `SlideContext`, not from this flag.
	 */
	preview?: boolean;
	/** The active theme's mermaid settings, from its theme.json. */
	mermaidOverrides?: MermaidThemeOverrides;
	/**
	 * True for a settled screenshot capture (`tap screenshot`, without
	 * `--wait`): a deck component must render without animating, the same as
	 * `printMode`, but - unlike `printMode` - at the REQUESTED step and
	 * fragment, not forced to the slide's final state. Kept separate from
	 * `printMode` for exactly that reason: `printMode` also feeds
	 * `SlideContext` (every markdown fragment shows, via Slot) and forces the
	 * `fragmentIndex`/`step` props above to their final values, both of
	 * which a settled capture must NOT do - it wants the fragment and step
	 * the capture actually asked for, just without any animation running
	 * while it asked for them.
	 */
	settleComponents?: boolean;
}

const DEFAULT_SCROLL_SPEED = 2000;

function getBackgroundStyle(background: BackgroundConfig | undefined): CSSProperties {
	if (!background) {
		return {};
	}
	switch (background.type) {
		case 'image':
			return {
				backgroundImage: `url('${background.value}')`,
				backgroundSize: 'cover',
				backgroundPosition: 'center'
			};
		case 'gradient':
			return { background: background.value };
		case 'color':
		default:
			return { backgroundColor: background.value };
	}
}

/** The slide's slot HTML, concatenated in declared order, for the build-mode error fallback. */
function rawSlotContent(slide: SlideData): string {
	return slide.slotOrder.map((name) => slide.slots[name] ?? '').join('');
}

export function Slide({
	slide,
	active,
	printMode,
	fragmentIndex,
	step,
	total,
	scrollRevealed = false,
	scrollTriggerCount = 0,
	preview = false,
	mermaidOverrides,
	settleComponents = false
}: SlideProps) {
	// Only what a deck component sees (through LayoutComponent for the
	// whole-slide form, and the inline portal props below) - see
	// settleComponents's doc comment above for why this must not also
	// reach SlideContext/Slot's fragment visibility or the fragmentIndex/
	// step props, which stay driven by `printMode` alone.
	const deckComponentPrintMode = printMode || settleComponents;
	const layout = resolveLayout(slide.layout);
	const LayoutComponent = layout.component;
	const declaredSlots = new Set(layout.slots);
	// A whole-slide component owns all of the slide's slot content itself
	// (it renders each slot it wants through tap's Slot helper, or ignores
	// the rest by choice - see the "component" layout's empty slot list in
	// registry.ts): never fall back to rendering "extra" slots after it, or
	// slot content the component intentionally ignored would show anyway.
	const extraSlotNames =
		slide.layout === 'component'
			? []
			: slide.slotOrder.filter((name) => !declaredSlots.has(name) && slide.slots[name] !== undefined);
	const scrollEnabled = slide.scroll === true;
	const contentRef = useRef<HTMLDivElement>(null);
	const [livePortals, setLivePortals] = useState<LiveCodeBlockPortal[]>([]);
	const [deckComponentPortals, setDeckComponentPortals] = useState<DeckComponentPortal[]>([]);

	useRichBlocks(contentRef, {
		slide,
		active: preview ? false : active,
		printMode: preview ? false : printMode,
		onLiveCodeBlocksChange: setLivePortals,
		onDeckComponentsChange: setDeckComponentPortals,
		mermaidOverrides
	});

	// A preview skips useRichBlocks, so the map code fence is never swapped
	// for a <MapSlide>. Strip it directly instead of running the full rich
	// blocks pass, so a thumbnail never shows the raw map directive as text.
	useLayoutEffect(() => {
		if (!preview) return;
		const element = contentRef.current;
		if (!element) return;
		element.querySelectorAll('pre > code.language-map').forEach((code) => {
			code.parentElement?.remove();
		});
	}, [preview, slide]);

	// Themes draw the slide number from data-index and data-total. With
	// `slideNumbers: false` in the frontmatter, data-slide-numbers="off"
	// tells each theme to leave it out.
	const slideNumbersOff = usePresentationStore((state) => state.presentation?.config?.slideNumbers === false);
	// The deck title, for themes that print it on every slide (the terminal
	// theme's tmux window tab). Absent when the deck has no title.
	const deckTitle = usePresentationStore((state) => state.presentation?.config?.title?.trim() || undefined);

	const mapBlock = slide.codeBlocks?.find((block) => block.language === 'map');
	const mapConfig = useMemo(() => (mapBlock ? parseMapConfig(mapBlock.code) : null), [mapBlock]);

	return (
		<SlideContext.Provider value={{ fragmentIndex, printMode }}>
			<div
				className={`slide${mapConfig ? ' has-map' : ''}`}
				data-layout={slide.layout}
				data-index={slide.index + 1}
				data-total={total}
				data-slide-numbers={slideNumbersOff ? 'off' : undefined}
				data-deck-title={deckTitle}
				style={getBackgroundStyle(slide.background)}
			>
				{slide.tag ? <div className="slide-tag">{slide.tag}</div> : null}
				{slide.badge ? <div className="slide-badge">{slide.badge}</div> : null}
				{mapConfig && !preview ? (
					<MapSlide config={mapConfig} step={step} active={active} printMode={printMode} />
				) : null}
				<div className={`slide-content${mapConfig ? ' map-content-overlay' : ''}`} ref={contentRef}>
					<ScrollReveal
						enabled={scrollEnabled}
						revealed={printMode || scrollRevealed}
						speed={slide.scrollSpeed ?? DEFAULT_SCROLL_SPEED}
						triggerCount={scrollTriggerCount}
					>
						<SlideErrorBoundary
							slideNumber={slide.index + 1}
							fallback={<div dangerouslySetInnerHTML={{ __html: rawSlotContent(slide) }} />}
						>
							<LayoutComponent
								slots={slide.slots}
								slide={slide}
								step={step}
								active={active}
								printMode={deckComponentPrintMode}
								preview={preview}
							/>
							{extraSlotNames.map((name) => (
								<Slot key={name} html={slide.slots[name]} className={`slot-${name}`} />
							))}
						</SlideErrorBoundary>
					</ScrollReveal>
					{livePortals.map((portal) =>
						createPortal(<LiveCodeBlock codeBlock={portal.codeBlock} />, portal.container, portal.key)
					)}
					{deckComponentPortals.map((portal) => {
						const info = slide.components?.find((component) => component.index === portal.index);
						if (!info) return null;
						// A preview (the overview grid, the presenter's next-slide
						// panel) always shows an inline component at its final
						// state, the same as the whole-slide form in
						// LayoutComponent.tsx: step at the slide total, no
						// animation, not "the one currently shown to the viewer".
						const componentStep = preview ? slide.steps : step;
						const componentActive = preview ? false : active;
						const componentPrintMode = preview ? true : deckComponentPrintMode;
						return createPortal(
							<DeckComponent
								source={info.source}
								url={info.url ?? ''}
								css={info.css}
								buildError={info.error}
								props={info.props ?? {}}
								slots={{}}
								slide={slide}
								step={componentStep}
								steps={slide.steps}
								active={componentActive}
								printMode={componentPrintMode}
								preview={preview}
								buildFallback={null}
							/>,
							portal.container,
							portal.key
						);
					})}
				</div>
			</div>
		</SlideContext.Provider>
	);
}
