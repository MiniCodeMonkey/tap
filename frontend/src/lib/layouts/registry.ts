/**
 * Maps a layout name to its React component and declared slot names.
 * The slot lists come from internal/layouts/layouts.json, the single file
 * that both Go and the frontend read, so the two sides never drift apart.
 */

import type { LayoutDefinition } from '$lib/types';
import layoutSlots from '../../../../internal/layouts/layouts.json';
import { LayoutBigStat } from './LayoutBigStat';
import { LayoutBlank } from './LayoutBlank';
import { LayoutCodeFocus } from './LayoutCodeFocus';
import { LayoutComponent } from './LayoutComponent';
import { LayoutCover } from './LayoutCover';
import { LayoutDefault } from './LayoutDefault';
import { LayoutQuote } from './LayoutQuote';
import { LayoutSection } from './LayoutSection';
import { LayoutSidebar } from './LayoutSidebar';
import { LayoutSplitMedia } from './LayoutSplitMedia';
import { LayoutThreeColumn } from './LayoutThreeColumn';
import { LayoutTitle } from './LayoutTitle';
import { LayoutTwoColumn } from './LayoutTwoColumn';

const components: Record<string, LayoutDefinition['component']> = {
	title: LayoutTitle,
	section: LayoutSection,
	default: LayoutDefault,
	'two-column': LayoutTwoColumn,
	'three-column': LayoutThreeColumn,
	'code-focus': LayoutCodeFocus,
	'big-stat': LayoutBigStat,
	quote: LayoutQuote,
	cover: LayoutCover,
	sidebar: LayoutSidebar,
	'split-media': LayoutSplitMedia,
	blank: LayoutBlank
};

export const layoutRegistry = new Map<string, LayoutDefinition>(
	Object.entries(layoutSlots).map(([name, slots]) => [
		name,
		{ component: components[name] ?? LayoutDefault, slots }
	])
);

// The "component" layout has no fixed slot list in layouts.json: a
// whole-slide deck component decides for itself which slots it renders
// (through tap's Slot helper) or ignores. Registered directly rather than
// from layoutSlots, with an empty slot list, so Slide.tsx's "extra slot"
// handling never double-renders a slot the component already placed itself.
layoutRegistry.set('component', { component: LayoutComponent, slots: [] });

/**
 * Resolve a layout name to its definition. An unknown name falls back to
 * the default layout, which shows all content, so a presentation never
 * goes blank on stage.
 */
export function resolveLayout(name: string): LayoutDefinition {
	const definition = layoutRegistry.get(name);
	if (definition) {
		return definition;
	}
	// The default layout is always present in layouts.json.
	return layoutRegistry.get('default') as LayoutDefinition;
}
