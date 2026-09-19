/**
 * The `tap` helper module: what a deck component imports as `import { ... }
 * from 'tap'`. Bundled into the host (frontend/src/lib/host.ts), never into
 * a component's own bundle - internal/components/host_shims.go resolves a
 * component's `import ... from 'tap'` to window.__TAP_HOST__["tap"] at
 * build time, so every component shares this one instance.
 *
 * A component should respect `active` and `printMode`: a preview render
 * (the overview grid, the presenter's next-slide panel) passes
 * `active: false` and `printMode: true` for every thumbnail at once, so a
 * component that starts an animation or a timer unconditionally on mount
 * would run once per thumbnail, all simultaneously. Read them with
 * `useActive()` / `usePrintMode()` (or `useStep()`, which is print-mode
 * aware for `step`) before starting anything that runs on its own.
 */

import {
	createContext,
	createElement,
	Fragment,
	useContext,
	useLayoutEffect,
	useRef,
	useState,
	type MutableRefObject,
	type ReactNode
} from 'react';

export { Slot } from '../components/Slot';

// ============================================================================
// Deck component context
// ============================================================================

export interface DeckComponentContextValue {
	step: number;
	steps: number;
	active: boolean;
	printMode: boolean;
	/** True for a thumbnail/preview render (the overview grid, the presenter's next-slide panel). */
	preview: boolean;
	/**
	 * The deck component's own rendered root, once mounted. Not part of the
	 * documented component contract; DeckComponent sets it so useTheme can
	 * find the nearest [data-theme] ancestor without every component
	 * needing to forward a ref of its own.
	 */
	rootRef: MutableRefObject<HTMLElement | null>;
}

const defaultRootRef: MutableRefObject<HTMLElement | null> = { current: null };

export const DeckComponentContext = createContext<DeckComponentContextValue>({
	step: 0,
	steps: 0,
	active: false,
	printMode: false,
	preview: false,
	rootRef: defaultRootRef
});

// ============================================================================
// Step hooks and component
// ============================================================================

/** The current presenter step and the slide's total step count. */
export function useStep(): { step: number; steps: number } {
	const { step, steps } = useContext(DeckComponentContext);
	return { step, steps };
}

/**
 * Whether this render should settle, without animation: PDF export, a
 * thumbnail/preview, or a `tap screenshot` capture (without `--wait`).
 * PDF export and a preview always pair this with the final step (see
 * useStep()); a screenshot capture pairs it with the REQUESTED step
 * instead - `usePrintMode()` only says "don't animate getting there",
 * never which step "there" is.
 */
export function usePrintMode(): boolean {
	return useContext(DeckComponentContext).printMode;
}

/** Whether this slide is the one currently shown to the viewer (false during a preview render). */
export function useActive(): boolean {
	return useContext(DeckComponentContext).active;
}

interface StepProps {
	children: ReactNode;
	/** Show from this step onward. */
	at?: number;
	/** Show from this step... */
	from?: number;
	/** ...through this step, inclusive. Only meaningful together with `from`. */
	to?: number;
}

/**
 * Reveals its children once the slide has advanced far enough:
 * `<Step at={n}>` from step n onward, `<Step from={a} to={b}>` for steps a
 * through b inclusive. Always compares against the current `step` -
 * `printMode` never overrides this: a true print export or a preview
 * already receives `step` forced to the slide's final step count (see
 * LayoutComponent.tsx), so it settles into "the final step's state" on its
 * own; a settled screenshot capture (`tap screenshot`, without `--wait`)
 * pairs `printMode` with the REQUESTED step instead, and a Step must
 * honor that request the same as it would live, just without animating.
 */
export function Step({ children, at, from, to }: StepProps) {
	const { step } = useContext(DeckComponentContext);

	const threshold = from ?? at ?? 0;
	const visible = from !== undefined ? step >= from && step <= (to ?? Infinity) : step >= threshold;

	if (!visible) {
		return null;
	}
	return createElement(Fragment, null, children);
}

// ============================================================================
// Theme tokens
// ============================================================================

/** The theme token names a component can read with useTheme(). */
export interface ThemeTokens {
	bg: string;
	fg: string;
	muted: string;
	accent: string;
	accentText: string;
	accent2: string;
	surface: string;
	/** A healthy or passing state; a deck component pairs it with a label, an icon, or a shape, never color alone. */
	statusOk: string;
	/** A warning state; a deck component pairs it with a label, an icon, or a shape, never color alone. */
	statusWarn: string;
	/** A failed state; a deck component pairs it with a label, an icon, or a shape, never color alone. */
	statusError: string;
	fontDisplay: string;
	fontBody: string;
	fontMono: string;
	ease: string;
	dur: string;
	spaceUnit: string;
	radius: string;
	strokeWidth: string;
}

/** CSS custom property name for each ThemeTokens key. */
const TOKEN_PROPERTIES: Record<keyof ThemeTokens, string> = {
	bg: '--bg',
	fg: '--fg',
	muted: '--muted',
	accent: '--accent',
	accentText: '--accent-text',
	accent2: '--accent-2',
	surface: '--surface',
	statusOk: '--status-ok',
	statusWarn: '--status-warn',
	statusError: '--status-error',
	fontDisplay: '--font-display',
	fontBody: '--font-body',
	fontMono: '--font-mono',
	ease: '--ease',
	dur: '--dur',
	spaceUnit: '--space-unit',
	radius: '--radius',
	strokeWidth: '--stroke-width'
};

const EMPTY_TOKENS: ThemeTokens = {
	bg: '',
	fg: '',
	muted: '',
	accent: '',
	accentText: '',
	accent2: '',
	surface: '',
	statusOk: '',
	statusWarn: '',
	statusError: '',
	fontDisplay: '',
	fontBody: '',
	fontMono: '',
	ease: '',
	dur: '',
	spaceUnit: '',
	radius: '',
	strokeWidth: ''
};

/** Reads every ThemeTokens value as a CSS custom property from `element`. A missing property reads as an empty string. */
function readTokens(element: Element): ThemeTokens {
	const style = getComputedStyle(element);
	const tokens = { ...EMPTY_TOKENS };
	for (const key of Object.keys(TOKEN_PROPERTIES) as (keyof ThemeTokens)[]) {
		tokens[key] = style.getPropertyValue(TOKEN_PROPERTIES[key]).trim();
	}
	return tokens;
}

/**
 * The active theme's tokens, read from the CSS custom properties on the
 * nearest `[data-theme]` ancestor of the component's own root. Re-reads
 * whenever that ancestor's `data-theme` attribute changes (a theme switch
 * from the dev server's live reload, or the `t` key in the viewer), or
 * whenever its inline `style` attribute changes (a deck's `themeColors`
 * frontmatter applies its overrides as inline custom properties on this
 * same element - see SlideCanvas.tsx).
 * Returns every value as an empty string until the component's root has
 * mounted, and for any token the current theme doesn't define. Reads the
 * tokens in a layout effect, not a plain effect, so they are populated
 * before the browser paints the component's first frame instead of after.
 */
export function useTheme(): ThemeTokens {
	const { rootRef } = useContext(DeckComponentContext);
	const [tokens, setTokens] = useState<ThemeTokens>(EMPTY_TOKENS);
	const themeElementRef = useRef<Element | null>(null);

	useLayoutEffect(() => {
		const root = rootRef.current;
		if (!root) return;

		const themeElement = root.closest('[data-theme]');
		themeElementRef.current = themeElement;
		if (!themeElement) return;

		setTokens(readTokens(themeElement));

		const observer = new MutationObserver(() => {
			setTokens(readTokens(themeElement));
		});
		observer.observe(themeElement, { attributes: true, attributeFilter: ['data-theme', 'style'] });

		return () => observer.disconnect();
	}, [rootRef]);

	return tokens;
}

// ============================================================================
// Color contrast
// ============================================================================

/**
 * Parses a CSS color string into 0-255 RGB channels plus an alpha 0-1, or
 * null for anything else. Accepts "#rgb", "#rrggbb", "#rrggbbaa",
 * "rgb(...)" and "rgba(...)". The alpha is returned rather than ignored:
 * a translucent fill (e.g. the base theme's `--surface: rgba(0, 0, 0,
 * 0.02)`) parses as near-black by its own channels alone, but reads as
 * near-white once composited over the page - see textOn below.
 */
function parseColor(color: string): [number, number, number, number] | null {
	const hex = /^#([0-9a-f]{3}|[0-9a-f]{6}|[0-9a-f]{8})$/i.exec(color.trim());
	if (hex?.[1]) {
		const digits = hex[1].length === 3 ? hex[1].replace(/(.)/g, '$1$1') : hex[1];
		const alpha = digits.length === 8 ? parseInt(digits.slice(6, 8), 16) / 255 : 1;
		return [
			parseInt(digits.slice(0, 2), 16),
			parseInt(digits.slice(2, 4), 16),
			parseInt(digits.slice(4, 6), 16),
			alpha
		];
	}
	const rgb = /^rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*(?:,\s*([\d.]+)\s*)?\)$/i.exec(color.trim());
	if (rgb) {
		return [Number(rgb[1]), Number(rgb[2]), Number(rgb[3]), rgb[4] !== undefined ? Number(rgb[4]) : 1];
	}
	return null;
}

/** Relative (WCAG) luminance of an 0-255 RGB triple, 0 (black) to 1 (white). */
function relativeLuminance([r, g, b]: readonly [number, number, number, ...number[]]): number {
	const channel = (value: number) => {
		const c = value / 255;
		return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
	};
	return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

/** WCAG contrast ratio between two relative luminances: 1 (no contrast) to 21 (black on white). */
function contrastRatio(a: number, b: number): number {
	const lighter = Math.max(a, b) + 0.05;
	const darker = Math.min(a, b) + 0.05;
	return lighter / darker;
}

/**
 * Picks `theme.bg` or `theme.fg`, whichever has the higher contrast ratio
 * against `fill`, for text painted on top of a `fill` background -
 * `theme.accent` is a fill color, not a text color, and which of bg/fg
 * reads on it differs per theme (a bright accent wants dark text, a
 * saturated dark accent wants light text). `fill` accepts "#rgb",
 * "#rrggbb", "#rrggbbaa", "rgb(...)" and "rgba(...)". An alpha below 1
 * (e.g. the base theme's `--surface: rgba(0, 0, 0, 0.02)`, near-black by
 * its own channels but near-white as painted on the page) is composited
 * over `theme.bg` before computing luminance, when `theme.bg` itself
 * parses. Falls back to `theme.fg` for any other input, including the
 * empty string every token holds before a component's root has mounted
 * (see useTheme above).
 */
export function textOn(fill: string, theme: ThemeTokens): string {
	const fillColor = parseColor(fill);
	const bgColor = parseColor(theme.bg);
	const fgColor = parseColor(theme.fg);
	if (!fillColor || !bgColor || !fgColor) return theme.fg;

	const [fillR, fillG, fillB, fillAlpha] = fillColor;
	const effectiveFill: [number, number, number] =
		fillAlpha < 1
			? [
					fillR * fillAlpha + bgColor[0] * (1 - fillAlpha),
					fillG * fillAlpha + bgColor[1] * (1 - fillAlpha),
					fillB * fillAlpha + bgColor[2] * (1 - fillAlpha)
				]
			: [fillR, fillG, fillB];

	const fillLuminance = relativeLuminance(effectiveFill);
	const contrastWithBg = contrastRatio(fillLuminance, relativeLuminance(bgColor));
	const contrastWithFg = contrastRatio(fillLuminance, relativeLuminance(fgColor));
	return contrastWithBg >= contrastWithFg ? theme.bg : theme.fg;
}
