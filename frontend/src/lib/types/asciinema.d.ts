/**
 * Type declarations for the asciinema-player library.
 * The library is loaded from CDN at runtime.
 */

/**
 * Asciinema player instance.
 */
export interface AsciinemaPlayerInstance {
	play(): void;
	pause(): void;
	getCurrentTime(): number;
	getDuration(): number;
	dispose(): void;
}

/**
 * Options for creating an asciinema player.
 */
export interface AsciinemaPlayerOptions {
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
	markers?: Array<[number, string]>;
	pauseOnMarkers?: boolean;
	terminalFontSize?: string;
	terminalFontFamily?: string;
	terminalLineHeight?: number;
	logger?: Console;
}

/**
 * AsciinemaPlayer global factory.
 */
export interface AsciinemaPlayerFactory {
	create(
		src: string,
		container: HTMLElement,
		options?: AsciinemaPlayerOptions
	): AsciinemaPlayerInstance;
}

// Augment the global Window interface
declare global {
	interface Window {
		AsciinemaPlayer?: AsciinemaPlayerFactory;
	}
}
