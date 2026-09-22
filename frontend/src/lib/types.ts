/**
 * TypeScript types for the Tap presentation viewer.
 * These types match the Go backend structs defined in:
 * - internal/transformer/transformer.go
 * - internal/config/config.go
 */

import type { ComponentType } from 'react';

// ============================================================================
// Layout Types
// ============================================================================

/**
 * Available slide layouts.
 * Auto-detected from content or specified via slide directives.
 */
export type Layout =
	| 'default'
	| 'title'
	| 'section'
	| 'two-column'
	| 'code-focus'
	| 'quote'
	| 'big-stat'
	| 'three-column'
	| 'cover'
	| 'sidebar'
	| 'split-media'
	| 'blank'
	| 'component';

// ============================================================================
// Transition Types
// ============================================================================

/**
 * Available slide transitions.
 */
export type Transition = 'none' | 'fade' | 'slide' | 'push' | 'zoom';

// ============================================================================
// Theme Types
// ============================================================================

/**
 * A presentation theme's slug, e.g. 'base' or 'terminal'. The full set of
 * built-in slugs lives in internal/themes/themes.json (see
 * lib/themes/loader.ts); this stays a plain string so the frontend doesn't
 * need a matching union type edited every time a theme is added.
 */
export type Theme = string;

/**
 * Default theme used when no theme is specified.
 */
export const DEFAULT_THEME: Theme = 'base';

// ============================================================================
// Config Types (matches internal/config/config.go)
// ============================================================================

/**
 * Connection configuration for a driver.
 * Matches Go's ConnectionConfig struct.
 */
export interface ConnectionConfig {
	host?: string;
	user?: string;
	password?: string;
	database?: string;
	path?: string;
	port?: number;
}

/**
 * Driver configuration for code execution.
 * Matches Go's DriverConfig struct.
 */
export interface DriverConfig {
	connections?: Record<string, ConnectionConfig>;
	command?: string;
	args?: string[];
	timeout?: number;
}

/**
 * Theme color override keys.
 * Maps to CSS custom properties:
 * - background -> --color-bg
 * - text -> --color-text
 * - muted -> --color-muted
 * - accent -> --color-accent
 * - codeBg -> --color-code-bg
 */
export interface ThemeColors {
	background?: string;
	text?: string;
	muted?: string;
	accent?: string;
	codeBg?: string;
}

/**
 * Presentation configuration from YAML frontmatter.
 * Matches Go's Config struct.
 */
export interface PresentationConfig {
	drivers?: Record<string, DriverConfig>;
	themeColors?: ThemeColors;
	title?: string;
	theme?: string;
	/** Path to a custom CSS theme file (relative to markdown file) */
	customTheme?: string;
	author?: string;
	date?: string;
	aspectRatio?: string;
	transition?: Transition;
	/** Whether to show the progress bar (default: true) */
	showProgressBar?: boolean;
	/** Whether the theme draws slide numbers (default: true) */
	slideNumbers?: boolean;
	/** Layout the presenter view opens in (default: 'standard') */
	presenterLayout?: string;
}

// ============================================================================
// Map Types
// ============================================================================

/**
 * Easing functions for map animations.
 */
export type MapEasing = 'linear' | 'ease-in' | 'ease-out' | 'ease-in-out';

/**
 * Configuration for map animations.
 * Parsed from ```map code blocks.
 */
export interface MapConfig {
	/** Starting coordinates [lat, lng] */
	start: [number, number];
	/** Ending coordinates [lat, lng] */
	end: [number, number];
	/** Initial zoom level (1-20) */
	zoom: number;
	/** Ending zoom level (defaults to zoom) */
	endZoom: number;
	/** Animation duration in milliseconds */
	duration: number;
	/** Animation easing function */
	easing: MapEasing;
	/** Camera pitch angle (0-85 degrees) */
	pitch: number;
	/** Camera bearing/rotation (0-360) */
	bearing: number;
	/** Map style URL or 'geocodio' for default */
	style: string;
	/** Show start/end markers */
	markers: boolean;
	/** Draw a line connecting start and end */
	showPath: boolean;
}

// ============================================================================
// Slide Types (matches internal/transformer/transformer.go)
// ============================================================================

/**
 * Background configuration for a slide.
 * Matches Go's BackgroundConfig struct.
 */
export interface BackgroundConfig {
	value: string;
	type: 'color' | 'image' | 'gradient';
}

/**
 * Code block ready for frontend rendering.
 * Matches Go's TransformedCodeBlock struct.
 */
export interface CodeBlock {
	language: string;
	code: string;
	driver?: string;
	connection?: string;
	/** Line-highlight spec such as "3" or "1,3-5", from a fence like "```php {1,3-5}". */
	highlightLines?: string;
}

/**
 * A deck-supplied component as it appears in a slide's JSON.
 * Matches Go's WholeSlideComponent (slide.component) and InlineComponent
 * (an entry of slide.components) structs; `index` and `props` are only
 * present on an inline entry.
 */
export interface SlideComponentInfo {
	/** The fence's position among the slide's inline components, in document order. Inline only. */
	index?: number;
	/** The component file's path, relative to the deck. */
	source: string;
	/** The bundle's JS URL. Omitted when the bundle failed to build. */
	url?: string;
	/** The bundle's CSS URL, when the component imports any. */
	css?: string;
	/** JSON from the fence body. Inline only; {} for the whole-slide form. */
	props?: Record<string, unknown>;
	/** Formatted build error ("<file>:<line>:<column>: <message>") when the bundle failed to build. */
	error?: string;
}

/**
 * Props tap passes to a deck component's default export, whole-slide or
 * inline. Matches the "Component contract" section of the deck components
 * spec.
 */
export interface DeckComponentProps {
	/** Slot HTML; whole-slide form only, {} for inline. */
	slots: Record<string, string>;
	/** JSON from the fence body; {} for the whole-slide form. */
	props: Record<string, unknown>;
	slide: Slide;
	/** 0..steps. */
	step: number;
	steps: number;
	active: boolean;
	/** True: render the final state, no animation. */
	printMode: boolean;
}

/**
 * Slide ready for frontend rendering.
 * Matches Go's TransformedSlide struct.
 */
export interface Slide {
	index: number;
	layout: Layout;
	html: string;
	notes?: string;
	transition?: Transition;
	background?: BackgroundConfig;
	codeBlocks?: CodeBlock[];
	/** Decorative metadata label (e.g., "// workshop") */
	tag?: string;
	/** Decorative metadata badge (e.g., "v2.0") */
	badge?: string;
	/** Enable scroll reveal for long content */
	scroll?: boolean;
	/** Animation duration in milliseconds (default: 2000) */
	scrollSpeed?: number;
	/** HTML content for each named slot in the slide's layout */
	slots: Record<string, string>;
	/** Slot names in the order they appear in the slide's markdown */
	slotOrder: string[];
	/** Number of incremental reveal fragments in the slide */
	fragmentCount: number;
	/** Number of presenter steps, including the initial state */
	steps: number;
	/**
	 * True when the slide's skip directive is set. Presenting passes over it
	 * and slide counts leave it out, but it keeps its place and its number
	 * in the deck.
	 */
	skip?: boolean;
	/** The whole-slide component when layout is "component". */
	component?: SlideComponentInfo;
	/** Each inline ```component fence found on the slide, in document order. */
	components?: SlideComponentInfo[];
}

// ============================================================================
// Presentation Types
// ============================================================================

/**
 * Complete presentation data from the backend.
 * Matches Go's TransformedPresentation struct.
 */
export interface Presentation {
	config: PresentationConfig;
	slides: Slide[];
}

// ============================================================================
// WebSocket Message Types
// ============================================================================

/**
 * WebSocket message types for hot reload and sync.
 */
export type WebSocketMessageType = 'connected' | 'reload' | 'slide' | 'theme' | 'recording';

/**
 * WebSocket message from the server.
 */
export interface WebSocketMessage {
	type: WebSocketMessageType;
	slideIndex?: number;
	/** Theme name for theme switching messages */
	theme?: string;
	/** Fragment index within the slide, -1 when none is revealed. Absent means "this slide, initial state". */
	fragment?: number;
	/** Presenter step within the slide, 0 when none. Absent means "this slide, initial state". */
	step?: number;
	/** Whether the slide's scroll reveal animation has completed. Absent means "this slide, initial state". */
	scrollRevealed?: boolean;
	/**
	 * True only on the hub's register-time state, sent once right after this
	 * client connects (see internal/server/websocket.go's register case).
	 * Absent (or false) on every live navigation broadcast, including the
	 * very first one this client happens to receive - a hub with no state
	 * yet sends no such message at all, so "first message received" is not
	 * a reliable way to detect it; this field is.
	 */
	initial?: boolean;
	/**
	 * Short content hash of the deck currently served, sent only on a
	 * "connected" message (see internal/server/websocket.go's register
	 * case). Absent when the hub has never had a presentation set.
	 */
	revision?: string;
	/** Recording disk status on a "recording" message. Absent means the disk is fine. */
	disk?: 'low' | 'full';
	/**
	 * Set to 'present' only on a "connected" message sent while the hub is
	 * running tap present rather than tap dev (see
	 * internal/server/websocket.go's SetPresentMode). Absent otherwise.
	 */
	mode?: 'present';
}

// ============================================================================
// API Types
// ============================================================================

/**
 * Request body for code execution API.
 */
export interface ExecuteRequest {
	driver: string;
	code: string;
	connection?: string;
}

/**
 * Response from code execution API.
 */
export interface ExecuteResponse {
	success: boolean;
	output?: string;
	error?: string;
	data?: Record<string, unknown>[];
}

// ============================================================================
// Layout Component Types
// ============================================================================

/**
 * Props passed to a layout component for rendering one slide.
 */
export interface LayoutProps {
	slots: Record<string, string>;
	slide: Slide;
	step: number;
	active: boolean;
	printMode: boolean;
	/** True for a thumbnail/preview render (the overview grid, the presenter's next-slide panel). Only the "component" layout reads this; every other layout ignores it. */
	preview?: boolean;
}

/**
 * A registered layout: its React component plus the slot names it expects.
 */
export interface LayoutDefinition {
	component: ComponentType<LayoutProps>;
	slots: string[];
}
