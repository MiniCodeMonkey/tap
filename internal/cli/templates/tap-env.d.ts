// Type declarations for the `tap` helper module and the deck component
// contract. Written once next to the deck by `tap add component`; an
// existing tap-env.d.ts is left alone, so edit this file freely.

declare module 'tap' {
	export interface ThemeTokens {
		bg: string;
		fg: string;
		muted: string;
		accent: string;
		accentText: string;
		surface: string;
		fontDisplay: string;
		fontBody: string;
		fontMono: string;
		ease: string;
		dur: string;
		spaceUnit: string;
		radius: string;
		strokeWidth: string;
	}

	export function useStep(): { step: number; steps: number };
	export function usePrintMode(): boolean;
	export function useActive(): boolean;
	export function useTheme(): ThemeTokens;

	export function Step(props: { children?: unknown; at?: number; from?: number; to?: number }): unknown;
	export function Slot(props: { html: string | undefined; className?: string }): unknown;

	/**
	 * Picks `theme.bg` or `theme.fg`, whichever contrasts more with `fill`,
	 * for text painted on top of a `fill` background - `theme.accent` is a
	 * fill color, not a text color, and which of bg/fg reads on it differs
	 * per theme. `fill` accepts "#rgb", "#rrggbb", "#rrggbbaa", "rgb(...)"
	 * and "rgba(...)"; any other input, including an empty string, falls
	 * back to `theme.fg`.
	 */
	export function textOn(fill: string, theme: ThemeTokens): string;

	/** Props tap passes to a deck component's default export, whole-slide or inline. */
	export interface DeckComponentProps {
		/** Slot HTML; whole-slide form only, {} for inline. */
		slots: Record<string, string>;
		/** JSON from the fence body; {} for the whole-slide form. */
		props: Record<string, unknown>;
		slide: unknown;
		/** 0..steps. */
		step: number;
		steps: number;
		active: boolean;
		/** True: render the final state, no animation. */
		printMode: boolean;
	}
}
