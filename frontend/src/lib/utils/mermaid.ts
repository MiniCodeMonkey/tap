/**
 * Mermaid diagram initialization and configuration utilities.
 * Handles mermaid.js setup with manual initialization (startOnLoad: false).
 */
import mermaid from 'mermaid';

let isInitialized = false;
let currentConfigKey: string | undefined;

/**
 * Mermaid theme variables. Colors are literal values, not CSS custom
 * properties: mermaid's theming engine parses each one to derive shades
 * (borders, gradients) and throws on a `var(...)` string it can't parse as
 * a color.
 */
export interface MermaidThemeVariables {
	primaryColor?: string;
	primaryTextColor?: string;
	primaryBorderColor?: string;
	lineColor?: string;
	secondaryColor?: string;
	tertiaryColor?: string;
	background?: string;
	mainBkg?: string;
	fontFamily?: string;
	fontSize?: string;
	nodeBorder?: string;
	clusterBkg?: string;
	clusterBorder?: string;
	edgeLabelBackground?: string;
	textColor?: string;
	titleColor?: string;
	nodeTextColor?: string;
}

/**
 * Mermaid theme configuration type.
 * Represents the configuration object passed to mermaid.initialize().
 */
export interface MermaidThemeConfig {
	theme: 'base';
	themeVariables: MermaidThemeVariables;
	curve: 'basis' | 'linear' | 'natural' | 'step' | 'stepAfter' | 'stepBefore';
}

/**
 * Overrides a later task can pass to customize the theme per Tap theme,
 * without changing how rendering itself works.
 */
export interface MermaidThemeOverrides {
	/** Theme variable overrides, merged over the base defaults. */
	themeVariables?: MermaidThemeVariables;
	/**
	 * CSS declarations (mermaid `classDef` syntax, comma-separated, e.g.
	 * `"fill:#0f1a16,stroke:#9db8a8,color:#dcebe0"`) for nodes a diagram
	 * marks with the `quiet` class, for app-server or background nodes that
	 * should recede. Also drops the edge label background box when set, for
	 * a plainer look.
	 */
	quietStyle?: string;
	/** Line curve style for flowchart edges. */
	curve?: MermaidThemeConfig['curve'];
}

/**
 * Default theme variables for the `base` Tap theme, matching the literal
 * colors in `lib/themes/base.css`. `fontFamily` is a single bare family
 * name because mermaid silently ignores a value with commas or quotes and
 * measures labels in the wrong font as a result.
 */
const BASE_THEME_VARIABLES: MermaidThemeVariables = {
	primaryColor: '#ffffff',
	primaryTextColor: '#18181b',
	primaryBorderColor: '#2563eb',
	lineColor: '#52525b',
	secondaryColor: 'rgba(0, 0, 0, 0.02)',
	tertiaryColor: '#ffffff',
	background: '#ffffff',
	mainBkg: '#ffffff',
	fontFamily: 'Inter',
	fontSize: '18px',
	nodeBorder: '#2563eb',
	clusterBkg: 'rgba(0, 0, 0, 0.02)',
	clusterBorder: 'rgba(24, 24, 27, 0.12)',
	edgeLabelBackground: '#ffffff',
	textColor: '#18181b',
	titleColor: '#2563eb',
	nodeTextColor: '#18181b'
};

/**
 * Get mermaid theme configuration for a tap presentation theme.
 * `base` is the only Tap theme, so this always builds from
 * `BASE_THEME_VARIABLES`, merging in any overrides a caller passes.
 *
 * @param overrides Theme variable, quiet-style, and curve overrides
 * @returns Mermaid theme configuration
 */
export function getMermaidTheme(overrides?: MermaidThemeOverrides): MermaidThemeConfig {
	const themeVariables: MermaidThemeVariables = {
		...BASE_THEME_VARIABLES,
		...(overrides?.quietStyle ? { edgeLabelBackground: 'transparent' } : {}),
		...overrides?.themeVariables
	};

	return {
		theme: 'base',
		themeVariables,
		curve: overrides?.curve ?? 'basis'
	};
}

/**
 * Initialize mermaid with default configuration.
 * Uses startOnLoad: false for manual control over diagram rendering.
 * Safe to call multiple times - will only reinitialize if the configuration changed.
 *
 * @param overrides Optional theme overrides to use for styling diagrams
 */
export function initializeMermaid(overrides?: MermaidThemeOverrides): void {
	const configKey = JSON.stringify(overrides ?? {});

	// Skip if already initialized with the same configuration
	if (isInitialized && currentConfigKey === configKey) {
		return;
	}

	const themeConfig = getMermaidTheme(overrides);

	mermaid.initialize({
		startOnLoad: false,
		securityLevel: 'strict',
		theme: themeConfig.theme,
		themeVariables: themeConfig.themeVariables,
		flowchart: {
			htmlLabels: true,
			nodeSpacing: 50,
			rankSpacing: 50,
			padding: 15,
			useMaxWidth: false,
			curve: themeConfig.curve,
			wrappingWidth: 300
		}
	});

	isInitialized = true;
	currentConfigKey = configKey;
}

/**
 * Check if mermaid has been initialized.
 */
export function isMermaidInitialized(): boolean {
	return isInitialized;
}

/**
 * Reset initialization state (primarily for testing).
 */
export function resetMermaidInitialization(): void {
	isInitialized = false;
	currentConfigKey = undefined;
}

/**
 * Get the mermaid instance for direct access if needed.
 */
export function getMermaid() {
	return mermaid;
}

/**
 * Counter for generating unique IDs for mermaid diagrams.
 */
let diagramCounter = 0;

/**
 * Reset the diagram counter (primarily for testing).
 */
export function resetDiagramCounter(): void {
	diagramCounter = 0;
}

/**
 * Result of rendering a mermaid diagram.
 */
export interface MermaidRenderResult {
	/** The rendered SVG string */
	svg: string;
	/** Whether the render was successful */
	success: true;
}

/**
 * Error result when mermaid rendering fails.
 */
export interface MermaidRenderError {
	/** Whether the render was successful */
	success: false;
	/** The error message */
	error: string;
	/** The original mermaid code */
	code: string;
}

/**
 * Wait for web fonts to finish loading before mermaid measures label text.
 * Mermaid lays out diagrams synchronously against whatever font is active
 * at render time, so rendering before fonts are ready produces wrong label
 * widths. `document.fonts` is unavailable in some test environments, so
 * this is a no-op there.
 */
async function waitForFonts(): Promise<void> {
	if (typeof document !== 'undefined' && document.fonts?.ready) {
		await document.fonts.ready;
	}
}

/**
 * Append a `classDef quiet <declarations>` line to mermaid source, so a
 * diagram that marks nodes with `class nodeName quiet` picks up the theme's
 * quiet styling. A no-op when the theme sets no quiet style, or the diagram
 * already defines its own `quiet` class.
 */
function withQuietClassDef(code: string, quietStyle: string | undefined): string {
	if (!quietStyle || /classDef\s+quiet\b/.test(code)) {
		return code;
	}
	return `${code}\nclassDef quiet ${quietStyle}`;
}

/**
 * Render a mermaid diagram from code.
 *
 * @param code The mermaid diagram code
 * @param overrides Optional theme overrides to use for styling
 * @returns Promise resolving to the rendered SVG or error
 */
export async function renderMermaidDiagram(
	code: string,
	overrides?: MermaidThemeOverrides
): Promise<MermaidRenderResult | MermaidRenderError> {
	initializeMermaid(overrides);
	await waitForFonts();

	const id = `mermaid-diagram-${++diagramCounter}`;
	const renderedCode = withQuietClassDef(code, overrides?.quietStyle);

	try {
		const { svg } = await mermaid.render(id, renderedCode);
		return { svg, success: true };
	} catch (err) {
		const errorMessage = err instanceof Error ? err.message : String(err);
		return {
			success: false,
			error: errorMessage,
			code
		};
	}
}

/**
 * Find and render all mermaid code blocks within an element.
 * Replaces <pre><code class="language-mermaid"> blocks with rendered SVGs.
 * Also re-renders existing mermaid diagrams when the configuration changes.
 *
 * @param element The DOM element to search within
 * @param overrides Optional theme overrides to use for styling diagrams
 * @returns Promise resolving when all diagrams are rendered
 */
export async function renderMermaidBlocksInElement(
	element: HTMLElement,
	overrides?: MermaidThemeOverrides
): Promise<void> {
	const configKey = JSON.stringify(overrides ?? {});

	// Scoped to .slot descendants throughout: a layout can render DOM a deck
	// component owns outside of a slot, and rich-block processing must never
	// mutate that DOM.
	// Find all unrendered mermaid code blocks
	const codeBlocks = element.querySelectorAll<HTMLElement>('.slot pre > code.language-mermaid');

	// Find all already-rendered diagrams that may need a re-render
	const existingDiagrams = element.querySelectorAll<HTMLElement>(
		'.slot .mermaid-diagram[data-mermaid-code]'
	);

	// Also find error containers that have stored code for retry
	const errorContainers = element.querySelectorAll<HTMLElement>('.slot .mermaid-error[data-mermaid-code]');

	if (codeBlocks.length === 0 && existingDiagrams.length === 0 && errorContainers.length === 0) {
		return;
	}

	// Render new mermaid code blocks
	const newBlockPromises = Array.from(codeBlocks).map(async (codeBlock) => {
		const pre = codeBlock.parentElement;
		if (!pre) return;

		// The host slide may have been unmounted by rapid navigation since
		// this pass started; skip this block's render work instead of doing
		// it for an element nothing will ever show.
		if (!pre.isConnected) return;

		const code = codeBlock.textContent ?? '';
		// Claim this element for this configuration before awaiting, so that
		// if another call (e.g. the theme's config resolving shortly after an
		// initial render with the base config) is racing to render this same
		// block, only the render whose configuration was requested last gets
		// applied below: an earlier request finishing after a later one has
		// nothing to overwrite it with, since it's no longer the latest claim.
		pre.dataset.mermaidPendingConfig = configKey;
		const result = await renderMermaidDiagram(code, overrides);

		if (pre.dataset.mermaidPendingConfig !== configKey) {
			// A newer render request for this block landed while this one was
			// pending; drop this stale result.
			return;
		}

		if (result.success) {
			// Create container for the rendered diagram, storing the code for re-rendering
			const container = document.createElement('div');
			container.className = 'mermaid-diagram';
			container.dataset.mermaidCode = code;
			container.dataset.mermaidConfig = configKey;
			container.innerHTML = result.svg;
			// Fix foreignObject text clipping by expanding widths
			fixForeignObjectWidths(container);
			pre.replaceWith(container);
		} else {
			// Show error message, storing the code for potential re-render
			const errorContainer = document.createElement('div');
			errorContainer.className = 'mermaid-error';
			errorContainer.dataset.mermaidCode = code;
			errorContainer.dataset.mermaidConfig = configKey;
			errorContainer.innerHTML = `
        <div class="mermaid-error-message">Mermaid diagram error: ${escapeHtml(result.error)}</div>
        <pre class="mermaid-error-code"><code>${escapeHtml(result.code)}</code></pre>
      `;
			pre.replaceWith(errorContainer);
		}
	});

	// Re-render existing diagrams if the configuration has changed
	const existingDiagramPromises = Array.from(existingDiagrams).map(async (diagram) => {
		const code = diagram.dataset.mermaidCode;
		const previousConfig = diagram.dataset.mermaidConfig;

		// Skip if no code stored or configuration hasn't changed
		if (!code || previousConfig === configKey) {
			return;
		}

		// See newBlockPromises above: skip a detached element's render work.
		if (!diagram.isConnected) return;

		// See the newBlockPromises claim above: guards against a stale render
		// from an earlier configuration overwriting a newer one.
		diagram.dataset.mermaidPendingConfig = configKey;
		const result = await renderMermaidDiagram(code, overrides);

		if (diagram.dataset.mermaidPendingConfig !== configKey) {
			return;
		}

		if (result.success) {
			diagram.innerHTML = result.svg;
			diagram.dataset.mermaidConfig = configKey;
			// Fix foreignObject text clipping
			fixForeignObjectWidths(diagram);
		} else {
			// Convert to error container
			const errorContainer = document.createElement('div');
			errorContainer.className = 'mermaid-error';
			errorContainer.dataset.mermaidCode = code;
			errorContainer.dataset.mermaidConfig = configKey;
			errorContainer.innerHTML = `
        <div class="mermaid-error-message">Mermaid diagram error: ${escapeHtml(result.error)}</div>
        <pre class="mermaid-error-code"><code>${escapeHtml(result.code)}</code></pre>
      `;
			diagram.replaceWith(errorContainer);
		}
	});

	// Re-render error containers (in case the new configuration fixes the issue)
	const errorContainerPromises = Array.from(errorContainers).map(async (errorContainer) => {
		const code = errorContainer.dataset.mermaidCode;
		const previousConfig = errorContainer.dataset.mermaidConfig;

		// Skip if no code stored or configuration hasn't changed
		if (!code || previousConfig === configKey) {
			return;
		}

		// See newBlockPromises above: skip a detached element's render work.
		if (!errorContainer.isConnected) return;

		// See the newBlockPromises claim above: guards against a stale render
		// from an earlier configuration overwriting a newer one.
		errorContainer.dataset.mermaidPendingConfig = configKey;
		const result = await renderMermaidDiagram(code, overrides);

		if (errorContainer.dataset.mermaidPendingConfig !== configKey) {
			return;
		}

		if (result.success) {
			// Convert to successful diagram
			const container = document.createElement('div');
			container.className = 'mermaid-diagram';
			container.dataset.mermaidCode = code;
			container.dataset.mermaidConfig = configKey;
			container.innerHTML = result.svg;
			errorContainer.replaceWith(container);
		} else {
			// Update error with new configuration
			errorContainer.dataset.mermaidConfig = configKey;
			const messageEl = errorContainer.querySelector('.mermaid-error-message');
			if (messageEl) {
				messageEl.textContent = `Mermaid diagram error: ${result.error}`;
			}
		}
	});

	await Promise.all([...newBlockPromises, ...existingDiagramPromises, ...errorContainerPromises]);
}

/**
 * Escape HTML special characters to prevent XSS.
 */
function escapeHtml(text: string): string {
	const div = document.createElement('div');
	div.textContent = text;
	return div.innerHTML;
}

/**
 * Fix mermaid foreignObject text clipping by expanding their widths.
 * Mermaid calculates text width using a generic font, but custom fonts may render wider.
 * This function expands foreignObject elements to ensure all text is visible.
 */
function fixForeignObjectWidths(container: HTMLElement): void {
	const foreignObjects = container.querySelectorAll('foreignObject');
	foreignObjects.forEach((fo) => {
		const text = fo.textContent?.trim() || '';
		if (text.length === 0) return;

		const currentWidth = parseFloat(fo.getAttribute('width') || '0');
		// Calculate minimum width based on character count (roughly 14px per char at 18px font)
		const minWidth = text.length * 14;
		const newWidth = Math.max(currentWidth * 1.6, minWidth);

		fo.setAttribute('width', String(newWidth));
	});
}
