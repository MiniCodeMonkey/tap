/**
 * Post-processes a slide's rendered HTML for rich content: syntax-highlighted
 * code, mermaid diagrams, and asciinema terminal recordings.
 *
 * Slot renders slide HTML via dangerouslySetInnerHTML, so these blocks start
 * out as plain <pre><code class="language-*"> markup. This hook walks the
 * mounted DOM after each render and replaces them with rendered output.
 */

import { useEffect, useRef, type RefObject } from 'react';
import type { CodeBlock, Slide } from '$lib/types';
import { highlightCodeBlocksInElement } from '../utils/highlighting';
import { renderMermaidBlocksInElement, type MermaidThemeOverrides } from '../utils/mermaid';
import { renderAsciinemaBlocksInElement, type AsciinemaPlayerInstance } from '../utils/asciinema';
import { parseMapConfig } from '../utils/map';

/** Disposes every player in the list, tolerating a player that errors on dispose (already gone). */
function disposeAsciinemaPlayers(players: AsciinemaPlayerInstance[]): void {
	for (const player of players) {
		try {
			player.dispose();
		} catch {
			// Already disposed, or its DOM is already gone; nothing more to do.
		}
	}
}

/** A live code block found in the DOM, ready to be portaled a `<LiveCodeBlock>` into. */
export interface LiveCodeBlockPortal {
	/** Stable key for the portal, derived from the code block's position in the slide. */
	key: string;
	/** The code block data to render. */
	codeBlock: CodeBlock;
	/** The wrapper element inserted in place of the code block's `<pre>`. */
	container: HTMLElement;
}

/** An inline deck component placeholder found in the DOM, ready to be portaled a `<DeckComponent>` into. */
export interface DeckComponentPortal {
	/** Stable key for the portal, derived from the component's index within the slide. */
	key: string;
	/** The component's index into slide.components. */
	index: number;
	/** The placeholder element itself (`<div class="deck-component" data-component-index="N">`), mounted into directly. */
	container: HTMLElement;
}

export interface UseRichBlocksOptions {
	/** The slide whose slot content should be post-processed. */
	slide: Slide;
	/** Whether this slide is the one currently shown to the viewer. */
	active: boolean;
	/** True when rendering for PDF export; processes even when not active. */
	printMode: boolean;
	/** Called with the live code block portals found on this pass, if any. */
	onLiveCodeBlocksChange?: (portals: LiveCodeBlockPortal[]) => void;
	/** Called with the inline deck component portals found on this pass, if any. */
	onDeckComponentsChange?: (portals: DeckComponentPortal[]) => void;
	/** The active theme's mermaid settings, from its theme.json. */
	mermaidOverrides?: MermaidThemeOverrides;
}

/**
 * Replace each driver code block's `<pre>` with a wrapper element so the
 * caller can portal a `<LiveCodeBlock>` into it. Also removes the `<pre>`
 * for a valid `map` code block, since it renders as a `<MapSlide>` instead.
 * Both run before highlighting so neither block is highlighted a second
 * time by `highlightCodeBlocksInElement`.
 *
 * Each entry in `codeBlocks` is paired with its `<pre>` by the
 * `data-code-block-index` attribute the backend writes onto the block's
 * `<code>` element (the block's position in this array, assigned in source
 * document order - see internal/parser/codeblocks.go), not by the block's
 * position in the DOM. A layout can render its slots - and therefore their
 * `<pre>` elements - in a different order than the source, so pairing by
 * DOM position pairs the wrong block with the wrong `<pre>` whenever that
 * happens. The search is scoped to `.slot` descendants so DOM a
 * deck-supplied component owns outside of a slot is never touched.
 */
function mountRichCodeBlocks(element: HTMLElement, codeBlocks: CodeBlock[]): LiveCodeBlockPortal[] {
	if (codeBlocks.length === 0) {
		return [];
	}

	const portals: LiveCodeBlockPortal[] = [];

	codeBlocks.forEach((codeBlock, index) => {
		const code = element.querySelector<HTMLElement>(
			`:is(.slot pre) > code[data-code-block-index="${index}"]`
		);
		const pre = code?.parentElement;
		if (!pre) return;

		if (codeBlock.language === 'map') {
			if (parseMapConfig(codeBlock.code) !== null) {
				pre.remove();
			}
			return;
		}

		if (!codeBlock.driver) return;

		const wrapper = document.createElement('div');
		wrapper.className = 'live-code-block-portal';
		pre.replaceWith(wrapper);
		portals.push({ key: `live-code-${index}`, codeBlock, container: wrapper });
	});

	return portals;
}

/**
 * Finds each inline deck component's placeholder
 * (`<div class="deck-component" data-component-index="N">`, written by the
 * parser at the ```component fence's position) within `.slot` descendants,
 * for the caller to portal a `<DeckComponent>` into directly - unlike a
 * live code block's `<pre>`, the placeholder is already a plain `<div>`,
 * so no wrapper element is needed.
 */
function findDeckComponentPlaceholders(element: HTMLElement): DeckComponentPortal[] {
	const nodes = element.querySelectorAll<HTMLElement>('.slot .deck-component[data-component-index]');
	return Array.from(nodes).map((node) => {
		const index = Number(node.getAttribute('data-component-index'));
		return { key: `deck-component-${index}`, index, container: node };
	});
}

/**
 * Build a signature for a slide's slot content, used to detect whether the
 * mounted DOM already reflects the current slide. A repeat effect run over
 * the same signature (React StrictMode runs effects twice in development)
 * is skipped instead of reprocessing the same nodes a second time.
 */
function slotContentSignature(slide: Slide): string {
	return `${slide.index}:${slide.slotOrder.map((name) => slide.slots[name] ?? '').join('\u0000')}`;
}

/**
 * Highlight code blocks and render mermaid diagrams and asciinema players
 * within `elementRef`'s current DOM node, whenever the slide's slot content
 * changes and the slide is active (or the presentation is in print mode).
 */
export function useRichBlocks(
	elementRef: RefObject<HTMLElement | null>,
	{
		slide,
		active,
		printMode,
		onLiveCodeBlocksChange,
		onDeckComponentsChange,
		mermaidOverrides
	}: UseRichBlocksOptions
): void {
	// The asciinema players currently mounted for this hook's content, so
	// they can be disposed either when the content they belong to is
	// replaced or when the host unmounts. A ref, not state: disposal is an
	// imperative side effect on these instances, not something a render
	// needs to react to.
	const asciinemaPlayersRef = useRef<AsciinemaPlayerInstance[]>([]);

	// Finds inline deck component placeholders and reports them, independent
	// of the active/printMode gate below: a preview render (the overview
	// grid, the presenter's next-slide panel) always passes active: false and
	// printMode: false, but the spec still requires inline components to
	// mount there (unless a component opts out with `export const preview =
	// false`). Unlike live code, mermaid, and asciinema, a placeholder is a
	// synchronous, non-mutating DOM read, so it needs no isNewContent guard
	// of its own - finding the same nodes twice (a StrictMode double-invoke)
	// just reports the same portals twice, which is harmless.
	useEffect(() => {
		const element = elementRef.current;
		if (!element || !element.isConnected) {
			return;
		}
		onDeckComponentsChange?.(findDeckComponentPlaceholders(element));
	}, [elementRef, slide, onDeckComponentsChange]);

	useEffect(() => {
		if (!active && !printMode) {
			return;
		}

		const element = elementRef.current;
		if (!element) {
			return;
		}

		// Claim this content synchronously so a same-tick repeat of this effect
		// doesn't launch a second, overlapping pass over the same nodes before
		// the first pass has had a chance to mutate the DOM. React StrictMode
		// runs this effect, its cleanup, then the effect again in the same
		// tick; `isNewContent` is computed once per run from the DOM marker, so
		// only the first of those two runs mounts portals and starts
		// highlighting. The async chain below is deliberately left running to
		// completion rather than aborted on cleanup: aborting it here would
		// leave StrictMode content permanently unhighlighted, because the
		// second run sees the marker already claimed and skips doing the
		// work itself. A theme change (mermaidOverrides changing with the
		// same slide content) still needs mermaid re-rendered, but must not
		// re-mount live code block portals or redo work that already happened
		// for this content: Shiki's highlighted output follows a theme change
		// on its own, through the `--shiki-*` CSS variables it was highlighted
		// with.
		const signature = slotContentSignature(slide);
		const isNewContent = element.dataset.richProcessed !== signature;
		if (isNewContent) {
			element.dataset.richProcessed = signature;
		}

		void (async () => {
			try {
				if (isNewContent) {
					const portals = mountRichCodeBlocks(element, slide.codeBlocks ?? []);
					onLiveCodeBlocksChange?.(portals);
				}
				// The chain above is deliberately left running to completion
				// rather than aborted (see the comment above), which matters for
				// StrictMode's synthetic cleanup, where the element stays
				// connected. It does not matter for a slide actually unmounted by
				// rapid navigation: there is nothing to show this work for, so
				// skip each remaining step once the element is no longer in the
				// document instead of doing free-running mermaid/asciinema/Shiki
				// work for it.
				if (!element.isConnected) return;
				await renderMermaidBlocksInElement(element, mermaidOverrides);
				if (isNewContent) {
					if (!element.isConnected) return;
					// Dispose the previous content's asciinema players before
					// mounting this content's own: this only runs when the
					// slide's slot content actually changed (isNewContent),
					// never merely because some other dependency (e.g.
					// mermaidOverrides, on a theme switch) re-ran this effect
					// for the same content, which must not interrupt a
					// player that's still showing the same recording.
					disposeAsciinemaPlayers(asciinemaPlayersRef.current);
					asciinemaPlayersRef.current = await renderAsciinemaBlocksInElement(element);
					if (!element.isConnected) return;
					await highlightCodeBlocksInElement(element);
				}
			} catch (err) {
				console.error('Error processing slide rich content:', err);
			}
		})();
	}, [elementRef, slide, active, printMode, onLiveCodeBlocksChange, onDeckComponentsChange, mermaidOverrides]);

	// Disposes whatever asciinema players remain mounted when this hook's
	// host unmounts (rapid navigation away from the slide, or the
	// component going away entirely). A separate effect with no
	// dependencies, so its cleanup fires only on unmount, not on every
	// dependency change the main effect above reruns for (see the comment
	// there about a theme switch). In React StrictMode this mounts,
	// cleans up, and mounts again synchronously in the same tick, before
	// any async work above has created a player, so that synthetic
	// cleanup finds nothing to dispose - the same reasoning that already
	// applies to Shiki highlighting in the effect above.
	useEffect(() => {
		return () => {
			disposeAsciinemaPlayers(asciinemaPlayersRef.current);
			asciinemaPlayersRef.current = [];
		};
	}, []);
}
