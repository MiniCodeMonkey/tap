/**
 * TypeScript types for the Tap presentation viewer.
 * These types match the Go backend structs defined in:
 * - internal/transformer/transformer.go
 * - internal/config/config.go
 */

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
	| 'blank';

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
 * Available presentation themes.
 * Each theme defines CSS custom properties for colors, typography, and visual effects.
 */
export type Theme =
	| 'paper'
	| 'noir'
	| 'aurora'
	| 'phosphor'
	| 'poster'
	| 'ink'
	| 'bauhaus'
	| 'editorial';

/**
 * Default theme used when no theme is specified.
 */
export const DEFAULT_THEME: Theme = 'paper';

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
	codeTheme?: string;
	fragments?: boolean;
	/** Whether to show the progress bar (default: true) */
	showProgressBar?: boolean;
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
}

/**
 * Fragment group for incremental reveals.
 * Matches Go's TransformedFragment struct.
 */
export interface FragmentGroup {
	content: string;
	index: number;
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
	fragments?: FragmentGroup[];
	background?: BackgroundConfig;
	codeBlocks?: CodeBlock[];
	/** Enable scroll reveal for long content */
	scroll?: boolean;
	/** Animation duration in milliseconds (default: 2000) */
	scrollSpeed?: number;
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
export type WebSocketMessageType = 'connected' | 'reload' | 'slide' | 'theme';

/**
 * WebSocket message from the server.
 */
export interface WebSocketMessage {
	type: WebSocketMessageType;
	slideIndex?: number;
	/** Theme name for theme switching messages */
	theme?: string;
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
