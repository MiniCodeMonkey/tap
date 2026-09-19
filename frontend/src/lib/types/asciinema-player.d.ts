/**
 * Type declarations for the `asciinema-player` npm package.
 *
 * Pinned to 3.9.0 (see frontend/package.json), which predates the package's
 * own bundled .d.ts, so this covers just the API surface
 * frontend/src/lib/utils/asciinema.ts actually uses.
 */
declare module 'asciinema-player' {
	export interface Player {
		dispose(): void;
		getCurrentTime(): number;
		getDuration(): number | undefined;
		play(): void;
		pause(): void;
	}

	export interface Options {
		cols?: number;
		rows?: number;
		autoPlay?: boolean;
		preload?: boolean;
		loop?: boolean;
		startAt?: number | string;
		speed?: number;
		idleTimeLimit?: number;
		theme?: string;
		poster?: string;
		fit?: 'width' | 'height' | 'both' | 'none' | false;
		controls?: boolean | 'auto';
		terminalFontSize?: string;
		terminalFontFamily?: string;
		terminalLineHeight?: number;
	}

	export function create(src: string, containerElement: HTMLElement, options?: Options): Player;
}
