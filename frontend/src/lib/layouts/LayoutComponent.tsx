/**
 * Layout "component": renders a whole-slide deck-supplied component
 * instead of a built-in layout. Registered in registry.ts without a fixed
 * slot list, since a component decides for itself which of the slide's
 * slots (if any) it renders through tap's Slot helper.
 */

import type { LayoutProps } from '$lib/types';
import { DeckComponent } from '../components/DeckComponent';
import { LayoutDefault } from './LayoutDefault';

/**
 * Rendered in a production build in place of a whole-slide component that
 * fails to build or render: falls back to showing all the slide's slots
 * with the default layout, so the presentation stays usable on stage
 * instead of showing nothing.
 */
function DefaultLayoutFallback(props: LayoutProps) {
	return <LayoutDefault {...props} />;
}

export function LayoutComponent({ slots, slide, step, active, printMode, preview = false }: LayoutProps) {
	const info = slide.component;
	const steps = slide.steps;

	// Defensive: layout "component" with no component info shouldn't happen
	// (the backend always sets slide.component alongside layout "component"),
	// but fall back to the default layout rather than rendering nothing.
	if (!info) {
		return <LayoutDefault slots={slots} slide={slide} step={step} active={active} printMode={printMode} />;
	}

	const effectiveStep = preview ? steps : step;
	const effectiveActive = preview ? false : active;
	const effectivePrintMode = preview ? true : printMode;

	return (
		<DeckComponent
			source={info.source}
			url={info.url ?? ''}
			css={info.css}
			buildError={info.error}
			props={{}}
			slots={slots}
			slide={slide}
			step={effectiveStep}
			steps={steps}
			active={effectiveActive}
			printMode={effectivePrintMode}
			preview={preview}
			buildFallback={
				<DefaultLayoutFallback
					slots={slots}
					slide={slide}
					step={effectiveStep}
					active={effectiveActive}
					printMode={effectivePrintMode}
				/>
			}
		/>
	);
}
