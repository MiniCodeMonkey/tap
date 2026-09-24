/**
 * Post-processes a slide's rendered HTML for rich content: syntax-highlighted
 * code, mermaid diagrams, and asciinema terminal recordings.
 *
 * Slot renders slide HTML via dangerouslySetInnerHTML, so these blocks start
 * out as plain <pre><code class="language-*"> markup. This hook walks the
 * mounted DOM after each render and replaces them with rendered output.
 */

import { useEffect, useLayoutEffect, useRef, type RefObject } from 'react';
import type { CodeBlock, Slide } from '$lib/types';
import { highlightCodeBlocksInElement } from '../utils/highlighting';
import { renderMermaidBlocksInElement, type MermaidThemeOverrides } from '../utils/mermaid';
import { renderAsciinemaBlocksInElement, type AsciinemaPlayerInstance } from '../utils/asciinema';
import { parseMapConfig } from '../utils/map';
import { holdReady } from '$lib/ready/blockers';

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
 *
 * `reusable` holds the wrappers of the slide's previous content, by block
 * identity (see liveCodeBlockIdentity), when this is a live update to the
 * same slide. A block with the same identity gets its old, detached wrapper
 * back instead of a new one, so the LiveCodeBlock portaled into it stays
 * mounted and keeps its output, rather than mounting again empty.
 */
function mountRichCodeBlocks(
	element: HTMLElement,
	codeBlocks: CodeBlock[],
	reusable: Map<string, HTMLElement> | null
): { portals: LiveCodeBlockPortal[]; wrappers: Map<string, HTMLElement> } {
	const portals: LiveCodeBlockPortal[] = [];
	const wrappers = new Map<string, HTMLElement>();
	if (codeBlocks.length === 0) {
		return { portals, wrappers };
	}

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

		const identity = liveCodeBlockIdentity(codeBlock, index);
		const mounted = reusable?.get(identity);
		let wrapper: HTMLElement;
		if (mounted && !mounted.isConnected) {
			wrapper = mounted;
		} else {
			wrapper = document.createElement('div');
			wrapper.className = 'live-code-block-portal';
		}
		pre.replaceWith(wrapper);
		wrappers.set(identity, wrapper);
		portals.push({ key: `live-code-${index}`, codeBlock, container: wrapper });
	});

	return { portals, wrappers };
}

/**
 * What makes a live code block the same block across a live update: its
 * position among the slide's code blocks and its own source (language,
 * driver, connection and code). The same block keeps its mounted
 * LiveCodeBlock, and with it the output of its last run. Any change to the
 * block's own source is a different block: its old output would describe
 * code that is no longer on the slide, so it mounts fresh, with no output.
 */
function liveCodeBlockIdentity(codeBlock: CodeBlock, index: number): string {
	return [index, codeBlock.language, codeBlock.driver ?? '', codeBlock.connection ?? '', codeBlock.code].join('\u0000');
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
 * What makes an inline deck component the same component across a live
 * update: its position among the slide's components and its source file.
 * A props change keeps the identity; a different file at that position,
 * or a new position, is a different component and mounts fresh.
 */
function deckComponentIdentity(slide: Slide, index: number): string {
	const source = slide.components?.find((component) => component.index === index)?.source ?? '';
	return `${index}\u0000${source}`;
}

/**
 * A short, non-cryptographic hash of `value` (djb2 XOR variant), base-36
 * encoded. Used to keep `data-rich-processed` a short marker instead of a
 * copy of the slide's whole slot HTML - the attribute only needs to detect
 * whether the content changed, never to be read back as content itself.
 */
function hashString(value: string): string {
	let hash = 5381;
	for (let i = 0; i < value.length; i++) {
		hash = (hash * 33) ^ value.charCodeAt(i);
	}
	return (hash >>> 0).toString(36);
}

/**
 * Build a signature for a slide's slot content, used to detect whether the
 * mounted DOM already reflects the current slide. A repeat effect run over
 * the same signature (React StrictMode runs effects twice in development)
 * is skipped instead of reprocessing the same nodes a second time.
 */
function slotContentSignature(slide: Slide): string {
	const content = `${slide.index}:${slide.slotOrder.map((name) => slide.slots[name] ?? '').join('\u0000')}`;
	return hashString(content);
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

	// The placeholder each inline deck component is mounted into, by its
	// identity on the slide on screen (see deckComponentIdentity), so a live
	// update can hand the same element back to the same component.
	const placeholdersRef = useRef<{ slideIndex: number; containers: Map<string, HTMLElement> } | null>(null);

	// Finds inline deck component placeholders and reports them, independent
	// of the active/printMode gate below: a preview render (the overview
	// grid, the presenter's next-slide panel) always passes active: false and
	// printMode: false, but the spec still requires inline components to
	// mount there (unless a component opts out with `export const preview =
	// false`).
	//
	// A live update to the slide on screen swaps the slot's HTML, which
	// replaces every placeholder with a new, empty element. Portaling into
	// that new element would unmount the component and mount it again,
	// replaying its entrance on every edit. Instead, each new placeholder
	// is swapped back for the detached one the same component (same slide,
	// same index, same source file) is already mounted in, so the component
	// keeps its instance and only receives new props. Navigating to another
	// slide starts with no placeholders to reuse, so its components mount
	// fresh and animate in. A layout effect, so the swap lands before the
	// browser paints the empty placeholder.
	useLayoutEffect(() => {
		const element = elementRef.current;
		if (!element || !element.isConnected) {
			return;
		}
		const previous = placeholdersRef.current;
		const reusable = previous && previous.slideIndex === slide.index ? previous.containers : null;
		const containers = new Map<string, HTMLElement>();
		const portals = findDeckComponentPlaceholders(element).map((portal) => {
			const identity = deckComponentIdentity(slide, portal.index);
			const mounted = reusable?.get(identity);
			let container = portal.container;
			if (mounted && mounted !== container && !mounted.isConnected) {
				container.replaceWith(mounted);
				container = mounted;
			}
			containers.set(identity, container);
			return { ...portal, container };
		});
		placeholdersRef.current = { slideIndex: slide.index, containers };
		onDeckComponentsChange?.(portals);
	}, [elementRef, slide, onDeckComponentsChange]);

	// The wrapper each live code block is mounted in, by its identity on the
	// slide on screen (see liveCodeBlockIdentity), for a live update to hand
	// back to the same block.
	const liveCodeWrappersRef = useRef<{ slideIndex: number; wrappers: Map<string, HTMLElement> } | null>(null);

	// Swaps each live code block's `<pre>` for the wrapper its LiveCodeBlock
	// is portaled into, and removes a map block's `<pre>`, once per slot
	// content. A layout effect, before the highlighting pass below and before
	// the browser paints: a live update to the slide on screen replaces the
	// slot's HTML, and each unchanged block gets its old wrapper back in the
	// same frame, so it keeps its mounted LiveCodeBlock and its output instead
	// of flashing the raw code and mounting again empty. Navigating to another
	// slide starts with no wrappers to reuse, so its blocks mount fresh. The
	// marker skips a repeat run over the same content (React StrictMode runs
	// layout effects twice), which would find no `<pre>` left to replace.
	useLayoutEffect(() => {
		if (!active && !printMode) {
			return;
		}
		const element = elementRef.current;
		if (!element) {
			return;
		}
		const signature = slotContentSignature(slide);
		if (element.dataset.liveCodeProcessed === signature) {
			return;
		}
		element.dataset.liveCodeProcessed = signature;
		const previous = liveCodeWrappersRef.current;
		const reusable = previous && previous.slideIndex === slide.index ? previous.wrappers : null;
		const { portals, wrappers } = mountRichCodeBlocks(element, slide.codeBlocks ?? [], reusable);
		liveCodeWrappersRef.current = { slideIndex: slide.index, wrappers };
		onLiveCodeBlocksChange?.(portals);
	}, [elementRef, slide, active, printMode, onLiveCodeBlocksChange]);

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
		// only the first of those two runs starts highlighting. The async
		// chain below is deliberately left running to completion rather than
		// aborted on cleanup: aborting it here would leave StrictMode content
		// permanently unhighlighted, because the second run sees the marker
		// already claimed and skips doing the work itself. A theme change
		// (mermaidOverrides changing with the same slide content) still needs
		// mermaid re-rendered, but must not redo work that already happened
		// for this content: Shiki's highlighted output follows a theme change
		// on its own, through the `--shiki-*` CSS variables it was highlighted
		// with.
		const signature = slotContentSignature(slide);
		const isNewContent = element.dataset.richProcessed !== signature;
		if (isNewContent) {
			element.dataset.richProcessed = signature;
		}

		// The slide holds a "component" blocker while this chain runs, so
		// the ready signal waits for mermaid, asciinema and Shiki output.
		const releaseReady = holdReady('component');

		void (async () => {
			try {
				// The chain below is deliberately left running to completion
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
			} finally {
				releaseReady();
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
