/**
 * Shared per-slide checks for the theme suite. Every check runs against a
 * single slide, in print mode (`?print=true#<n>`), against the real
 * `[data-theme]` canvas: no mocking, no snapshotting the DOM outside the
 * browser.
 *
 * Thresholds live in one place (MINIMUM_FONT_SIZES) so a later design
 * decision can retune them without hunting through the check bodies.
 */

import type { Page } from '@playwright/test';

// ============================================================================
// Minimum font sizes
// ============================================================================

/**
 * Minimum computed font sizes, in canvas pixels (the slide's native
 * 1920x1080 coordinate space, not CSS pixels after any viewport scaling).
 * `body` covers `p` and `li` that are direct slot content; `code` covers
 * `pre code`; `table` covers `td` and `th`; `floor` is the hard minimum for
 * everything else measured (labels, captions, folios), including `h1`/`h2`
 * (see `headlineRecommended` below for their separate, non-failing bar).
 */
export const MINIMUM_FONT_SIZES = {
	body: 40,
	code: 36,
	table: 40,
	floor: 24
} as const;

/**
 * Recommended (not required) minimum size for `h1`/`h2`, in canvas pixels.
 * A headline under this still has to clear `MINIMUM_FONT_SIZES.floor` (the
 * hard minimum every measured element is held to), but is otherwise only
 * reported as a warning, not a failure: see `findSmallHeadlines`.
 */
export const HEADLINE_RECOMMENDED_SIZE = 60;

// ============================================================================
// Overflow
// ============================================================================

/** One element that extends outside the slide's 1920x1080 box. */
export interface OverflowViolation {
	selector: string;
	text: string;
	rect: { top: number; right: number; bottom: number; left: number };
}

/**
 * Finds every text or media element that extends outside the slide's
 * 1920x1080 box. A 1px tolerance absorbs sub-pixel rounding from CSS
 * transforms and fractional layout.
 */
export async function findOverflow(page: Page): Promise<OverflowViolation[]> {
	return page.evaluate(() => {
		const TOLERANCE = 1;
		const slide = document.querySelector('.slide[data-layout]');
		if (!slide) return [];

		const slideRect = slide.getBoundingClientRect();
		const selectors = ['h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'p', 'li', 'td', 'th', 'pre', 'blockquote', 'img'];
		const violations: {
			selector: string;
			text: string;
			rect: { top: number; right: number; bottom: number; left: number };
		}[] = [];

		for (const selector of selectors) {
			const elements = slide.querySelectorAll<HTMLElement>(selector);
			for (const element of elements) {
				// The asciinema player's own internal implementation DOM
				// (e.g. a hidden `<pre class="ap-term-text">` accessibility/
				// selection layer, unrelated to the visible terminal) isn't
				// something a theme controls or this check can meaningfully
				// judge; only the wrapper a theme actually styles matters
				// here (rich-blocks.css fits it to the slide already).
				if (element.closest('.asciinema-player-wrapper') !== null) continue;

				const rect = element.getBoundingClientRect();
				// Skip elements with no visible box (display: none, empty text).
				if (rect.width === 0 && rect.height === 0) continue;

				// A scroll slide (`.scroll-content`) is deliberately taller than
				// the slide; the slide clips it (overflow: hidden) and the
				// presenter scrolls it into view. Only its horizontal overflow
				// is a real bug.
				const inScrollContent = element.closest('.scroll-content') !== null;

				const overflows = inScrollContent
					? rect.left < slideRect.left - TOLERANCE || rect.right > slideRect.right + TOLERANCE
					: rect.top < slideRect.top - TOLERANCE ||
						rect.left < slideRect.left - TOLERANCE ||
						rect.bottom > slideRect.bottom + TOLERANCE ||
						rect.right > slideRect.right + TOLERANCE;

				if (overflows) {
					violations.push({
						selector,
						text: (element.textContent ?? '').trim().slice(0, 80),
						rect: { top: rect.top, right: rect.right, bottom: rect.bottom, left: rect.left }
					});
				}
			}
		}

		return violations;
	});
}

// ============================================================================
// Occluded or clipped text
// ============================================================================

/** One text-bearing element hidden under opaque chrome or cut off by an ancestor's overflow. */
export interface OccludedOrClippedTextViolation {
	selector: string;
	text: string;
	kind: 'clipped' | 'occluded';
	detail: string;
}

const OCCLUSION_CANDIDATE_SELECTOR = 'h1, h2, h3, h4, h5, h6, p, li, td, th, blockquote, pre, figcaption, dt, dd';

/**
 * Finds text-bearing elements that the overflow check can't see because
 * they never leave the slide's 1920x1080 box: text hidden under opaque
 * chrome (a status bar, a page-number band) and text cut off by an
 * ancestor with `overflow: hidden`/`clip`/`scroll`/`auto`. `findOverflow`
 * only tests whether an element's own box leaves the slide; both of these
 * defects put the box entirely inside the slide while the rendered text is
 * still not readable.
 *
 * Two independent checks, either of which flags an element:
 *
 * (a) Clipping: the element's bounding box extends more than 2px outside
 * the visible box of any ancestor (walking up to, and including, the slide
 * root) whose computed `overflow`/`overflow-x`/`overflow-y` isn't
 * `visible`, intersecting every such ancestor's clip box on the way. A
 * scroll-layout slide's own `.slide { overflow: hidden }` (the mechanism
 * that lets `.scroll-content` be taller than the slide, see
 * docs/reference/theme-porting.md section 3.7) is expected to clip content
 * vertically, so for an element inside `.scroll-content` that clip is
 * exempted on the vertical axis only, matching `findOverflow`'s own
 * horizontal-only rule for scroll slides. Any other clipping ancestor
 * (including a horizontal clip on the slide itself, or any clip found
 * inside `.scroll-content`) still counts.
 *
 * (b) Occlusion: samples points inside the element's own text (the center
 * and four 25%-inset points of every client rect `Range.getClientRects()`
 * reports for the element's text, up to ~30 points) and asks
 * `document.elementFromPoint` what's on top. A point is covered when the
 * hit element is neither the candidate nor one of its descendants or
 * ancestors, and the hit (or one of the hit's own ancestors, down to the
 * slide root) paints an opaque background (`background-color` alpha above
 * 0.5, or a `background-image`) or is itself an `img`/`canvas`. More than
 * 20% of an element's sampled points covered reports it.
 *
 * `elementFromPoint` already skips `pointer-events: none` elements, which
 * is exactly what's wanted here: a decorative overlay (scanlines, a
 * vignette) with `pointer-events: none` is invisible to this hit-test the
 * same way it's invisible to a mouse, so it never falsely reports real
 * chrome as occluding text under it. No separate exclusion is needed for
 * it.
 *
 * The canvas is scaled to fit the viewport by a CSS transform on the
 * 1920x1080 frame; `getBoundingClientRect()`, `Range.getClientRects()`,
 * and `elementFromPoint` all already operate in the same post-transform
 * viewport coordinate space, so no extra scale correction is needed here
 * (contrast `findSmallText`, which measures a font-size *number* and must
 * convert it back to canvas pixels).
 */
export async function findOccludedOrClippedText(page: Page): Promise<OccludedOrClippedTextViolation[]> {
	return page.evaluate((selector) => {
		const CLIP_TOLERANCE = 2;
		const MAX_SAMPLE_POINTS = 30;
		const OCCLUSION_THRESHOLD = 0.2;

		const slide = document.querySelector('.slide[data-layout]');
		if (!slide) return [];

		// Matches the 4th (alpha) component specifically, not just "whatever
		// number comes last before the close paren" - `rgb(0, 0, 0)` (plain
		// opaque black, no alpha channel at all) has no 4th component, and a
		// naive "last number before `)`" match misreads its blue channel as
		// alpha instead, reporting a perfectly opaque black background as
		// transparent.
		function parseAlpha(color: string): number {
			const match = color.match(/rgba?\(\s*[\d.]+\s*,\s*[\d.]+\s*,\s*[\d.]+\s*(?:,\s*([\d.]+)\s*)?\)/);
			if (!match) return color === 'transparent' ? 0 : 1; // unparseable or keyword: assume opaque unless explicitly transparent
			return match[1] === undefined ? 1 : Number(match[1]); // no 4th component: fully opaque
		}

		function isOpaqueBackground(element: Element): boolean {
			const style = getComputedStyle(element);
			if (style.backgroundImage && style.backgroundImage !== 'none') return true;
			return parseAlpha(style.backgroundColor) > 0.5;
		}

		// Stops *before* the slide root itself: the slide's own base
		// background is not "chrome" painted on top of the text, it's the
		// ordinary backdrop showing through empty space (an SVG's hit-testable
		// bounding box is its whole element, even where nothing is actually
		// drawn at a given pixel, e.g. a mermaid diagram's mostly-empty
		// canvas). Only an ancestor strictly between the hit and the slide
		// root counts as something actually layered over the text.
		function hitIsOpaqueChrome(hit: Element): boolean {
			let node: Element | null = hit;
			while (node && node !== slide) {
				if (node.tagName === 'IMG' || node.tagName === 'CANVAS') return true;
				if (isOpaqueBackground(node)) return true;
				node = node.parentElement;
			}
			return false;
		}

		interface ClipBox {
			top: number;
			right: number;
			bottom: number;
			left: number;
		}

		function intersect(a: ClipBox, b: ClipBox): ClipBox {
			return {
				top: Math.max(a.top, b.top),
				left: Math.max(a.left, b.left),
				right: Math.min(a.right, b.right),
				bottom: Math.min(a.bottom, b.bottom)
			};
		}

		/**
		 * The client rects of an element's own real text, one per actual text
		 * node rather than one `Range` spanning the whole element. Selecting
		 * the whole element in one `Range` (as an earlier version of this
		 * check did) can hand back an extra, spurious rect for a run of
		 * `white-space: pre` whitespace between lines (shiki's highlighted
		 * code keeps literal newline characters for copy-paste) that stretches
		 * far wider than any actual glyph, mis-measuring both checks below.
		 * Per-text-node rects, with whitespace-only nodes skipped, avoid that
		 * artifact.
		 */
		function textContentRects(element: Element): DOMRect[] {
			const rects: DOMRect[] = [];
			const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT, {
				acceptNode(node) {
					return node.textContent && node.textContent.trim().length > 0 ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT;
				}
			});
			let textNode: Text | null;
			// eslint-disable-next-line no-cond-assign
			while ((textNode = walker.nextNode() as Text | null)) {
				const range = document.createRange();
				range.selectNodeContents(textNode);
				for (const rect of Array.from(range.getClientRects())) {
					if (rect.width > 0 && rect.height > 0) rects.push(rect);
				}
			}
			return rects;
		}

		const violations: { selector: string; text: string; kind: 'clipped' | 'occluded'; detail: string }[] = [];

		const candidates = slide.querySelectorAll<HTMLElement>(selector);
		for (const element of candidates) {
			// The asciinema player's own internal accessibility DOM isn't
			// something a theme controls (see findOverflow's identical
			// exclusion). Mermaid/MapLibre draw their own text directly onto
			// the canvas/SVG, not through the theme's chrome, so occlusion or
			// clipping there is a library layout question, not a theme defect.
			if (element.closest('.asciinema-player-wrapper, .mermaid-diagram, .map-slide') !== null) continue;

			const style = getComputedStyle(element);
			if (style.visibility === 'hidden' || style.display === 'none') continue;

			const text = (element.textContent ?? '').trim();
			if (text.length === 0) continue;

			const elementRect = element.getBoundingClientRect();
			if (elementRect.width === 0 && elementRect.height === 0) continue;

			const contentRects = textContentRects(element);
			if (contentRects.length === 0) continue; // no real glyphs to test either check against

			// The box actually occupied by the element's own rendered text
			// (the union of its real text-node rects), not the element's full
			// CSS box: a theme can legitimately give an element a full-bleed
			// background wider than the text inside it (padding insets the
			// glyphs), and clipping *that* decorative box against an ancestor
			// is not a readability bug the way clipping the text itself is.
			const rawTextBox = contentRects.reduce(
				(box, rect) => ({
					top: Math.min(box.top, rect.top),
					left: Math.min(box.left, rect.left),
					right: Math.max(box.right, rect.right),
					bottom: Math.max(box.bottom, rect.bottom)
				}),
				{ top: Infinity, left: Infinity, right: -Infinity, bottom: -Infinity }
			);
			// Clamped to the element's own box: some fonts (decorative/hand-drawn
			// faces especially) report a `Range` line-box a handful of pixels
			// taller than the element's own layout box purely from font-metric
			// overshoot (ascent/descent padding baked into the font), with no
			// corresponding difference in the actual painted ink - a heading
			// that sits close to the slide's own edge can otherwise register a
			// phantom clip against the slide root that a screenshot shows
			// nothing wrong with. Clipping imposed by the element's *own* box
			// leaving an ancestor (rather than a font-metric quirk inside it)
			// is exactly what this clamp still catches: it only trims text
			// overshoot *beyond* the element itself, never hides an ancestor
			// genuinely smaller than the element's own rendered box.
			const textBox: ClipBox = {
				top: Math.max(rawTextBox.top, elementRect.top),
				left: Math.max(rawTextBox.left, elementRect.left),
				right: Math.min(rawTextBox.right, elementRect.right),
				bottom: Math.min(rawTextBox.bottom, elementRect.bottom)
			};

			const describeElement = (): string => {
				const classes = element.className && typeof element.className === 'string' ? `.${element.className.trim().split(/\s+/).join('.')}` : '';
				return `${element.tagName.toLowerCase()}${classes}`;
			};

			// --- (a) Clipping -----------------------------------------------
			const inScrollContent = element.closest('.scroll-content') !== null;
			let clipBox: ClipBox | null = null;
			let clippedBy: string | null = null;

			let ancestor: Element | null = element.parentElement;
			while (ancestor) {
				const ancestorStyle = getComputedStyle(ancestor);
				const clipsX = ancestorStyle.overflowX !== 'visible';
				const clipsY = ancestorStyle.overflowY !== 'visible';
				if (clipsX || clipsY) {
					const rect = ancestor.getBoundingClientRect();
					// A scroll slide clips vertically by design (see
					// docs/reference/theme-porting.md section 3.7): some themes
					// put that `overflow: hidden` on `.slide` itself, others on
					// `.slide-content` or another wrapper around
					// `.scroll-content`. Rather than special-case one specific
					// ancestor, exempt every ancestor's vertical clip for an
					// element inside `.scroll-content` - matching
					// `findOverflow`'s own rule that only horizontal overflow is
					// a real bug there.
					const exemptVertical = inScrollContent;

					const candidateBox: ClipBox = {
						top: clipsY && !exemptVertical ? rect.top : -Infinity,
						bottom: clipsY && !exemptVertical ? rect.bottom : Infinity,
						left: clipsX ? rect.left : -Infinity,
						right: clipsX ? rect.right : Infinity
					};

					if (candidateBox.top !== -Infinity || candidateBox.bottom !== Infinity || candidateBox.left !== -Infinity || candidateBox.right !== Infinity) {
						clipBox = clipBox ? intersect(clipBox, candidateBox) : candidateBox;
						clippedBy = clippedBy ?? `${ancestor.tagName.toLowerCase()}${ancestor.className && typeof ancestor.className === 'string' ? '.' + ancestor.className.trim().split(/\s+/).join('.') : ''}`;
					}
				}
				if (ancestor === slide) break;
				ancestor = ancestor.parentElement;
			}

			if (clipBox) {
				const overflowsClip =
					textBox.top < clipBox.top - CLIP_TOLERANCE ||
					textBox.left < clipBox.left - CLIP_TOLERANCE ||
					textBox.bottom > clipBox.bottom + CLIP_TOLERANCE ||
					textBox.right > clipBox.right + CLIP_TOLERANCE;
				if (overflowsClip) {
					violations.push({
						selector: describeElement(),
						text: text.slice(0, 80),
						kind: 'clipped',
						detail: `clipped by ${clippedBy ?? 'an ancestor'}'s overflow`
					});
					continue; // a clipped element's occlusion sampling isn't meaningful
				}
			}

			// --- (b) Occlusion ------------------------------------------------
			const points: { x: number; y: number }[] = [];
			const perRectBudget = Math.max(1, Math.floor(MAX_SAMPLE_POINTS / contentRects.length));
			for (const rect of contentRects) {
				if (points.length >= MAX_SAMPLE_POINTS) break;
				const samples: { x: number; y: number }[] = [
					{ x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 }, // center
					{ x: rect.left + rect.width * 0.25, y: rect.top + rect.height * 0.25 },
					{ x: rect.left + rect.width * 0.75, y: rect.top + rect.height * 0.25 },
					{ x: rect.left + rect.width * 0.25, y: rect.top + rect.height * 0.75 },
					{ x: rect.left + rect.width * 0.75, y: rect.top + rect.height * 0.75 }
				];
				for (const point of samples.slice(0, perRectBudget)) {
					if (points.length >= MAX_SAMPLE_POINTS) break;
					points.push(point);
				}
			}
			if (points.length === 0) continue;

			let covered = 0;
			for (const point of points) {
				const hit = document.elementFromPoint(point.x, point.y);
				if (!hit) continue;
				const isSelfOrRelated = hit === element || element.contains(hit) || hit.contains(element);
				if (isSelfOrRelated) continue;
				if (hitIsOpaqueChrome(hit)) covered++;
			}

			const coveredRatio = covered / points.length;
			if (coveredRatio > OCCLUSION_THRESHOLD) {
				violations.push({
					selector: describeElement(),
					text: text.slice(0, 80),
					kind: 'occluded',
					detail: `${Math.round(coveredRatio * 100)}% of sampled points covered by opaque chrome`
				});
			}
		}

		return violations;
	}, OCCLUSION_CANDIDATE_SELECTOR);
}

// ============================================================================
// Contrast
// ============================================================================

export interface ContrastResult {
	name: string;
	foreground: string;
	background: string;
	ratio: number;
	minimum: number;
	passes: boolean;
}

/**
 * Relative luminance per WCAG 2.x, from an sRGB color string as the browser
 * resolves it (`rgb(r, g, b)` or `rgba(r, g, b, a)`).
 */
function relativeLuminance(rgb: string): number | null {
	const match = rgb.match(/rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)/);
	if (!match) return null;

	const [r, g, b] = [match[1], match[2], match[3]].map((component) => {
		const value = Number(component) / 255;
		return value <= 0.03928 ? value / 12.92 : Math.pow((value + 0.055) / 1.055, 2.4);
	});

	return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrastRatio(a: string, b: string): number | null {
	const luminanceA = relativeLuminance(a);
	const luminanceB = relativeLuminance(b);
	if (luminanceA === null || luminanceB === null) return null;

	const lighter = Math.max(luminanceA, luminanceB);
	const darker = Math.min(luminanceA, luminanceB);
	return (lighter + 0.05) / (darker + 0.05);
}

/**
 * Reads `--bg`, `--fg`, `--accent-text`, and `--muted` off the theme's
 * canvas root and checks their contrast against `--bg`: `--fg` and
 * `--accent-text` at 7:1, `--muted` at 4.5:1.
 */
export async function readContrast(page: Page): Promise<ContrastResult[]> {
	const colors = await page.evaluate(() => {
		// The theme's custom properties (--bg, --fg, etc.) are set on the
		// 1920x1080 canvas frame inside .slide-container (see SlideCanvas.tsx),
		// not on .slide-container itself - that element is only the letterbox
		// area around the canvas now.
		const root = document.querySelector('.slide-container [data-theme]');
		if (!root) return null;
		const style = getComputedStyle(root);
		const resolve = (name: string): string => {
			// Resolve a custom property by rendering it on a probe element, so
			// values like a bare hex or a color function both come back as the
			// browser's canonical rgb()/rgba() serialization.
			const probe = document.createElement('div');
			probe.style.color = `var(${name})`;
			root.appendChild(probe);
			const resolved = getComputedStyle(probe).color;
			probe.remove();
			return resolved;
		};

		return {
			bg: resolve('--bg'),
			fg: resolve('--fg'),
			accentText: resolve('--accent-text'),
			muted: resolve('--muted'),
			statusOk: resolve('--status-ok'),
			statusWarn: resolve('--status-warn'),
			statusError: resolve('--status-error'),
			raw: {
				bg: style.getPropertyValue('--bg').trim(),
				fg: style.getPropertyValue('--fg').trim(),
				accentText: style.getPropertyValue('--accent-text').trim(),
				muted: style.getPropertyValue('--muted').trim()
			}
		};
	});

	if (!colors) return [];

	const pairs: { name: string; foreground: string; minimum: number }[] = [
		{ name: 'fg-on-bg', foreground: colors.fg, minimum: 7 },
		{ name: 'accent-text-on-bg', foreground: colors.accentText, minimum: 7 },
		{ name: 'muted-on-bg', foreground: colors.muted, minimum: 4.5 },
		// Status tokens are read as a fill (a badge, a dot, an icon), not as
		// body text, so they only need to clear the 3:1 non-text contrast
		// minimum against --bg, not the 7:1/4.5:1 bars above.
		{ name: 'status-ok-on-bg', foreground: colors.statusOk, minimum: 3 },
		{ name: 'status-warn-on-bg', foreground: colors.statusWarn, minimum: 3 },
		{ name: 'status-error-on-bg', foreground: colors.statusError, minimum: 3 }
	];

	return pairs.map(({ name, foreground, minimum }) => {
		const ratio = contrastRatio(foreground, colors.bg) ?? 0;
		return {
			name,
			foreground,
			background: colors.bg,
			ratio: Math.round(ratio * 100) / 100,
			minimum,
			passes: ratio >= minimum
		};
	});
}

// ============================================================================
// Code text contrast
// ============================================================================

export interface CodeContrastResult {
	name: string;
	foreground: string;
	background: string;
	ratio: number;
	minimum: number;
	passes: boolean;
}

/**
 * Checks the contrast of code text against its own panel background, for
 * every highlighted code block (`pre.shiki`) on the slide. A theme that
 * points `--shiki-foreground` at the slide's own text colour instead of a
 * colour meant for the dark code panel makes plain identifiers unreadable,
 * which the slide-level `readContrast` check (fg/accent-text/muted against
 * `--bg`) never sees, since it never looks at `--shiki-*`.
 *
 * For each code block, checks:
 * - the plain (unclassified) token colour against the panel background, at
 *   least 7:1;
 * - the same plain colour on a dimmed line (`pre.has-highlighted .line:not(.highlighted)`,
 *   which the base theme renders at `opacity: 0.5`), blended toward the
 *   background by that opacity, at least 4.5:1;
 * - the comment token colour (`--shiki-token-comment`) against the panel
 *   background, at least 4.5:1 (also blended by its line's opacity, if
 *   dimmed).
 *
 * The background behind a token is the first non-transparent
 * `background-color` found walking up from the token itself: a theme's
 * `--shiki-background` usually sets it on the `pre`, but a highlighted
 * line can carry its own background (a slab distinct from the rest of the
 * block), so the walk starts at the token rather than assuming the pre's
 * own background applies to every line in it.
 */
export async function readCodeContrast(page: Page): Promise<CodeContrastResult[]> {
	const rawResults = await page.evaluate(() => {
		function parseRgb(rgb: string): { r: number; g: number; b: number; a: number } | null {
			const match = rgb.match(/rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)(?:\s*,\s*([\d.]+))?\)/);
			if (!match) return null;
			return {
				r: Number(match[1]),
				g: Number(match[2]),
				b: Number(match[3]),
				a: match[4] === undefined ? 1 : Number(match[4])
			};
		}

		/**
		 * Walks up from `start`, compositing every non-transparent
		 * `background-color` it finds ("over" alpha compositing, nearest
		 * layer on top) until the accumulated color is fully opaque or
		 * ancestors run out. A single non-transparent-but-translucent layer
		 * (e.g. a highlighted line's tinted slab) is not the whole story:
		 * its own alpha lets whatever sits behind it show through, so using
		 * its raw color alone understates or overstates the real contrast.
		 */
		function panelBackground(start: Element): string | null {
			let node: Element | null = start;
			let composite: { r: number; g: number; b: number; a: number } | null = null;

			while (node) {
				const rgb = parseRgb(getComputedStyle(node).backgroundColor);
				if (rgb && rgb.a > 0) {
					if (!composite) {
						composite = rgb;
					} else {
						// The layer found first (closer to the token) sits on top.
						composite = {
							r: composite.r * composite.a + rgb.r * (1 - composite.a),
							g: composite.g * composite.a + rgb.g * (1 - composite.a),
							b: composite.b * composite.a + rgb.b * (1 - composite.a),
							a: composite.a + rgb.a * (1 - composite.a)
						};
					}
					if (composite.a >= 0.999) break;
				}
				node = node.parentElement;
			}

			if (!composite) return null;
			return `rgb(${composite.r}, ${composite.g}, ${composite.b})`;
		}

		/** Product of every ancestor's own `opacity`, up to (excluding) `pre`. */
		function opacityWithin(element: Element, pre: Element): number {
			let node: Element | null = element;
			let opacity = 1;
			while (node && node !== pre) {
				opacity *= Number(getComputedStyle(node).opacity) || 1;
				node = node.parentElement;
			}
			return opacity;
		}

		/** Blend a foreground colour toward the background by its effective opacity. */
		function blend(fg: { r: number; g: number; b: number }, bg: { r: number; g: number; b: number }, opacity: number): string {
			const r = bg.r + opacity * (fg.r - bg.r);
			const g = bg.g + opacity * (fg.g - bg.g);
			const b = bg.b + opacity * (fg.b - bg.b);
			return `rgb(${r}, ${g}, ${b})`;
		}

		const results: {
			name: string;
			foreground: string;
			background: string;
			minimum: number;
		}[] = [];

		const blocks = document.querySelectorAll('pre.shiki');
		blocks.forEach((pre, blockIndex) => {
			const isHighlighted = pre.classList.contains('has-highlighted');
			const label = blocks.length > 1 ? ` (block ${blockIndex + 1})` : '';

			function record(token: Element | null, name: string, minimum: number): void {
				if (!token) return;
				const background = panelBackground(token);
				if (!background) return;
				const backgroundRgb = parseRgb(background);
				if (!backgroundRgb) return;
				const rgb = parseRgb(getComputedStyle(token).color);
				if (!rgb) return;
				const opacity = opacityWithin(token, pre);
				results.push({ name: `${name}${label}`, foreground: blend(rgb, backgroundRgb, opacity), background, minimum });
			}

			// A plain, unclassified token: shiki gives it no explicit token
			// color, so it inherits `--shiki-foreground` from whichever
			// element sets it (the pre, or a highlighted line's own override).
			// The "default text" bar (7:1) applies to the full-opacity line;
			// under `has-highlighted` that is the highlighted line itself,
			// since the other lines are the ones dimmed to `opacity: 0.5`.
			const plainSelector = 'code .line:not(.highlighted) span[style*="--shiki-foreground"]';
			if (isHighlighted) {
				record(pre.querySelector('code .line.highlighted span[style*="--shiki-foreground"]'), 'code-text', 7);
				record(pre.querySelector(plainSelector), 'code-dimmed-text', 4.5);
			} else {
				record(pre.querySelector(plainSelector), 'code-text', 7);
			}

			// A comment token: shiki marks it with `--shiki-token-comment`.
			record(pre.querySelector('code span[style*="--shiki-token-comment"]'), 'code-comment', 4.5);
		});

		return results;
	});

	return rawResults.map(({ name, foreground, background, minimum }) => {
		const ratio = contrastRatio(foreground, background) ?? 0;
		return {
			name,
			foreground,
			background,
			ratio: Math.round(ratio * 100) / 100,
			minimum,
			passes: ratio >= minimum
		};
	});
}

// ============================================================================
// Minimum text size
// ============================================================================

export interface SmallTextViolation {
	selector: string;
	text: string;
	fontSizePx: number;
	minimumPx: number;
}

/**
 * Finds visible text below the minimum size for its category, by walking
 * every text node inside `.slide-content` (a `TreeWalker`) rather than
 * querying a fixed list of tags. The previous implementation
 * (`content.querySelectorAll('p, li, h1, ..., pre code, blockquote')`)
 * missed real small text in at least two ways found by eye in keynote's
 * three-column and sidebar layouts: its selector list didn't include every
 * element a theme can put real slide text in (a `div`/`span` wrapper isn't
 * `p`/`li`/a heading/`td`/`th`, so it was invisible to the check no matter
 * its font size), and even where a selected element (a `li`, say) wraps a
 * nested element with its own smaller `font-size` (a `span`, a secondary
 * line), the old code read the *outer* selected element's own computed
 * size, never the nested element actually carrying the small text - it
 * only ever measured a font-size on a tag it happened to have queried for.
 * Walking text nodes and reading `getComputedStyle` on each text node's
 * *own* immediate parent (never a container further up) fixes both: any
 * element that owns visible text is measured, at the size that element
 * itself renders it, regardless of its tag.
 *
 * Skips whitespace-only text, `script`/`style` content, and any element
 * that is `visibility: hidden` or has zero size (`display: none`, or
 * collapsed). Also skips `.mermaid-diagram`/`.map-slide` (drawn by mermaid
 * or MapLibre, not the theme), the live code block's own document-scale UI
 * (`.code-actions`, `.static-placeholder`) and the bundled asciinema
 * player's internals (`.asciinema-player-wrapper`, which has its own
 * play/speed controls, `.control-button`, none of it theme-styled slide
 * content); and `.slide-tag`/`.slide-badge`, a theme's page numbers, tags,
 * badges, and status-bar chrome. That chrome already has to clear the same
 * 24px hard floor every other "everything else" element does, but it lives
 * outside `.slide-content` (a `.slide-tag`/`.slide-badge` is a sibling of
 * `.slide-content`, not a descendant, see `Slide.tsx`) or is CSS generated
 * content (`::before`/`::after`, which produces no text node a `TreeWalker`
 * can see at all) for most themes' page-number/status-bar bands - it is not
 * slide content this function's body/code/table/heading classification is
 * meant to apply to, so it's skipped here rather than measured against the
 * wrong bar.
 *
 * Each text-owning element is classified once, by context: inside `pre` or
 * `code` gets the code floor (36px), inside a `table` gets the table floor
 * (40px), a heading (`h1`-`h6`) gets the 24px hard floor (a heading's own,
 * separate 60px *recommendation* is `findSmallHeadlines`'s job, and only
 * for `h1`/`h2`), everything else gets the body floor (40px).
 */
export async function findSmallText(page: Page, minimums = MINIMUM_FONT_SIZES): Promise<SmallTextViolation[]> {
	return page.evaluate((mins) => {
		const content = document.querySelector('.slide-content');
		if (!content) return [];

		const scale = (() => {
			const slide = document.querySelector('.slide[data-layout]');
			if (!slide) return 1;
			const rect = slide.getBoundingClientRect();
			return rect.width > 0 ? rect.width / 1920 : 1;
		})();

		function isExcluded(element: Element): boolean {
			return (
				element.closest(
					'.mermaid-diagram, .map-slide, .live-code-block .code-actions, .live-code-block .static-placeholder, .asciinema-player-wrapper, .slide-tag, .slide-badge'
				) !== null
			);
		}

		function minimumFor(element: Element): number {
			if (element.closest('pre, code') !== null) return mins.code;
			if (element.closest('table') !== null) return mins.table;
			if (/^h[1-6]$/i.test(element.tagName)) return mins.floor;
			return mins.body;
		}

		function describe(element: Element): string {
			const classes = typeof element.className === 'string' && element.className.trim() ? `.${element.className.trim().split(/\s+/).join('.')}` : '';
			return `${element.tagName.toLowerCase()}${classes}`;
		}

		const violations: { selector: string; text: string; fontSizePx: number; minimumPx: number }[] = [];
		const measured = new Set<Element>();

		const walker = document.createTreeWalker(content, NodeFilter.SHOW_TEXT, {
			acceptNode(node) {
				if (!node.textContent || node.textContent.trim().length === 0) return NodeFilter.FILTER_REJECT;
				const parent = node.parentElement;
				if (!parent) return NodeFilter.FILTER_REJECT;
				const tag = parent.tagName.toLowerCase();
				if (tag === 'script' || tag === 'style') return NodeFilter.FILTER_REJECT;
				return NodeFilter.FILTER_ACCEPT;
			}
		});

		let node: Text | null;
		// eslint-disable-next-line no-cond-assign
		while ((node = walker.nextNode() as Text | null)) {
			const parent = node.parentElement;
			if (!parent || measured.has(parent)) continue;
			if (isExcluded(parent)) continue;

			const style = getComputedStyle(parent);
			if (style.visibility === 'hidden') continue;
			const rect = parent.getBoundingClientRect();
			if (rect.width === 0 || rect.height === 0) continue;

			measured.add(parent);

			const fontSizePx = parseFloat(style.fontSize) / scale;
			const minimumPx = minimumFor(parent);

			if (fontSizePx < minimumPx - 0.5) {
				violations.push({
					selector: describe(parent),
					text: (parent.textContent ?? '').trim().slice(0, 80),
					fontSizePx: Math.round(fontSizePx * 10) / 10,
					minimumPx
				});
			}
		}

		return violations;
	}, minimums);
}

// ============================================================================
// Headline size recommendation
// ============================================================================

export interface SmallHeadlineWarning {
	selector: string;
	text: string;
	fontSizePx: number;
	recommendedPx: number;
}

/**
 * Finds `h1`/`h2` elements inside `.slide-content` below the recommended
 * headline size (`HEADLINE_RECOMMENDED_SIZE`, 60px). Unlike `findSmallText`,
 * this is advisory: a theme is free to run a shorter headline smaller (a
 * `long` h1/h2, or a deliberate design choice), as long as it still clears
 * the hard floor `findSmallText` enforces. Callers surface the result as a
 * warning (a test annotation or a console message), never a failure.
 */
export async function findSmallHeadlines(
	page: Page,
	recommendedPx = HEADLINE_RECOMMENDED_SIZE
): Promise<SmallHeadlineWarning[]> {
	return page.evaluate((recommended) => {
		const content = document.querySelector('.slide-content');
		if (!content) return [];

		const scale = (() => {
			const slide = document.querySelector('.slide[data-layout]');
			if (!slide) return 1;
			const rect = slide.getBoundingClientRect();
			return rect.width > 0 ? rect.width / 1920 : 1;
		})();

		function isExcluded(element: Element): boolean {
			return element.closest('.mermaid-diagram, .map-slide') !== null;
		}

		const warnings: { selector: string; text: string; fontSizePx: number; recommendedPx: number }[] = [];
		const candidates = content.querySelectorAll<HTMLElement>('h1, h2');

		for (const element of candidates) {
			if (isExcluded(element)) continue;
			const text = (element.textContent ?? '').trim();
			if (text.length === 0) continue;

			const fontSizePx = parseFloat(getComputedStyle(element).fontSize) / scale;
			if (fontSizePx < recommended - 0.5) {
				warnings.push({
					selector: element.tagName.toLowerCase(),
					text: text.slice(0, 80),
					fontSizePx: Math.round(fontSizePx * 10) / 10,
					recommendedPx: recommended
				});
			}
		}

		return warnings;
	}, recommendedPx);
}

// ============================================================================
// Entrance animation end state
// ============================================================================

export interface AnimationEndStateViolation {
	selector: string;
	text: string;
	reason: 'opacity-below-1' | 'visibility-hidden' | 'zero-size-clip';
	detail: string;
}

const ANIMATION_TEXT_SELECTOR = 'h1, h2, h3, h4, h5, h6, p, li, td, th, blockquote, figcaption, dt, dd';

/**
 * Waits for every running Web Animation on the page to finish (a fixed
 * timeout, not a fixed wait, so a theme's own animation durations don't
 * need to be known here), then checks every text-bearing element for a
 * hidden-looking end state: `opacity` under 1, `visibility: hidden`, or a
 * zero-size `clip`/`clip-path`. The suite's other checks all run in print
 * mode (`?print=true`), which turns entrance animations off outright (see
 * docs/reference/theme-porting.md section 3.6) and so never exercises
 * whether one of them actually finishes in a visible state; this check is
 * the one place that loads a slide live and lets its animations run to
 * completion before looking.
 *
 * `opacity` is read with a small tolerance (0.98) rather than a strict
 * `< 1`, since a theme can leave an animation's `fill: forwards` keyframe
 * a hair under 1 by design (a barely-there flicker meant to read as
 * "settled", not "still fading"); this check is for an entrance animation
 * that never finishes revealing its own text, not for that.
 */
export async function findAnimationEndStateViolations(page: Page): Promise<AnimationEndStateViolation[]> {
	await page.evaluate(async () => {
		const animations = document.getAnimations();
		await Promise.race([
			Promise.all(animations.map((animation) => animation.finished.catch(() => undefined))),
			new Promise((resolve) => setTimeout(resolve, 5000))
		]);
	});

	return page.evaluate((selector) => {
		const OPACITY_TOLERANCE = 0.98;
		const violations: AnimationEndStateViolation[] = [];

		const candidates = document.querySelectorAll<HTMLElement>(selector);
		for (const element of candidates) {
			const text = (element.textContent ?? '').trim();
			if (text.length === 0) continue;

			const rect = element.getBoundingClientRect();
			if (rect.width === 0 || rect.height === 0) continue;

			const style = getComputedStyle(element);
			const selectorLabel = element.tagName.toLowerCase();
			const textSample = text.slice(0, 80);

			const opacity = parseFloat(style.opacity);
			if (!Number.isNaN(opacity) && opacity < OPACITY_TOLERANCE) {
				violations.push({
					selector: selectorLabel,
					text: textSample,
					reason: 'opacity-below-1',
					detail: `opacity: ${style.opacity}`
				});
				continue;
			}

			if (style.visibility === 'hidden') {
				violations.push({
					selector: selectorLabel,
					text: textSample,
					reason: 'visibility-hidden',
					detail: 'visibility: hidden'
				});
				continue;
			}

			const clip = style.clip;
			const clipPath = style.clipPath;
			const clipsToZero = (value: string) => {
				const rectMatch = /^rect\(\s*([\d.]+px|auto)\s*,\s*([\d.]+px|auto)\s*,\s*([\d.]+px|auto)\s*,\s*([\d.]+px|auto)\s*\)$/.exec(
					value
				);
				if (rectMatch) {
					const [, top, right, bottom, left] = rectMatch;
					const toNumber = (part: string) => (part === 'auto' ? null : parseFloat(part));
					const t = toNumber(top);
					const r = toNumber(right);
					const b = toNumber(bottom);
					const l = toNumber(left);
					if (t !== null && b !== null && b - t <= 0) return true;
					if (l !== null && r !== null && r - l <= 0) return true;
				}
				return value === 'circle(0px)' || value === 'circle(0px at 50% 50%)' || /^inset\(\s*100%/.test(value);
			};

			if ((clip && clip !== 'auto' && clipsToZero(clip)) || (clipPath && clipPath !== 'none' && clipsToZero(clipPath))) {
				violations.push({
					selector: selectorLabel,
					text: textSample,
					reason: 'zero-size-clip',
					detail: `clip: ${clip}; clip-path: ${clipPath}`
				});
			}
		}

		return violations;
	}, ANIMATION_TEXT_SELECTOR);
}
