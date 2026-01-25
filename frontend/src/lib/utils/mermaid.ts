/**
 * Mermaid diagram initialization and configuration utilities.
 * Handles mermaid.js setup with manual initialization (startOnLoad: false).
 */
import mermaid from 'mermaid'
import type { Theme } from '$lib/types'

let isInitialized = false
let currentTheme: Theme | undefined

/**
 * Mermaid theme configuration type.
 * Represents the configuration object passed to mermaid.initialize().
 */
export interface MermaidThemeConfig {
  theme: 'default' | 'dark' | 'forest' | 'neutral' | 'base'
  themeVariables: {
    primaryColor?: string
    primaryTextColor?: string
    primaryBorderColor?: string
    lineColor?: string
    secondaryColor?: string
    tertiaryColor?: string
    background?: string
    mainBkg?: string
    fontFamily?: string
    fontSize?: string
    nodeBorder?: string
    clusterBkg?: string
    clusterBorder?: string
    edgeLabelBackground?: string
    textColor?: string
    titleColor?: string
    nodeTextColor?: string
  }
}

/**
 * Get mermaid theme configuration for a tap presentation theme.
 * Maps tap themes to mermaid theme settings with appropriate colors and fonts.
 * Font sizes are optimized for presentation scale (18-20px base).
 *
 * @param theme The tap presentation theme
 * @returns Mermaid theme configuration
 */
export function getMermaidTheme(theme: Theme): MermaidThemeConfig {
  switch (theme) {
    case 'paper':
      return {
        theme: 'neutral',
        themeVariables: {
          // Paper: Clean light theme with blue accent (#2563eb) matching theme
          primaryColor: '#f0f4ff', // Subtle blue-tinted light background
          primaryTextColor: '#18181b', // Near-black text
          primaryBorderColor: '#2563eb', // Blue accent from theme
          lineColor: '#64748b', // Muted gray for clean lines
          secondaryColor: '#f8fafc', // Very light surface
          tertiaryColor: '#ffffff',
          background: '#fafafa', // Match theme background
          mainBkg: '#f0f4ff', // Subtle blue tint
          fontFamily:
            'Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
          fontSize: '18px', // Larger for presentation scale
          nodeBorder: '#2563eb', // Blue accent
          clusterBkg: '#f8fafc',
          clusterBorder: '#cbd5e1', // Subtle gray border
          edgeLabelBackground: '#ffffff',
          textColor: '#18181b',
          titleColor: '#2563eb', // Blue accent for titles
          nodeTextColor: '#18181b',
        },
      }

    case 'noir':
      return {
        theme: 'dark',
        themeVariables: {
          // Noir: Cinematic dark theme with refined gold accent (#c9a227)
          primaryColor: '#1a1a1a', // Deep charcoal nodes
          primaryTextColor: '#f5f5f5', // Light text
          primaryBorderColor: '#c9a227', // Refined gold accent
          lineColor: '#c9a227', // Gold lines for elegance
          secondaryColor: '#141414', // Slightly lighter dark
          tertiaryColor: '#0f0f0f',
          background: '#0a0a0a', // True dark background
          mainBkg: '#181818', // Elevated surface
          fontFamily:
            'Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
          fontSize: '18px', // Larger for presentation scale
          nodeBorder: '#c9a227', // Gold accent
          clusterBkg: '#121212',
          clusterBorder: 'rgba(201, 162, 39, 0.5)', // Muted gold border
          edgeLabelBackground: '#181818',
          textColor: '#f5f5f5',
          titleColor: '#c9a227', // Gold titles
          nodeTextColor: '#f5f5f5',
        },
      }

    case 'aurora':
      return {
        theme: 'dark',
        themeVariables: {
          // Aurora: Vibrant gradients with cyan/teal accents
          primaryColor: '#1e1b4b', // Deep indigo for nodes
          primaryTextColor: '#ffffff',
          primaryBorderColor: '#22d3ee', // Cyan accent from theme
          lineColor: '#0ea5e9', // Electric blue lines
          secondaryColor: '#312e81', // Deep purple
          tertiaryColor: '#0f0a1f',
          background: '#0a0614', // Deep aurora background
          mainBkg: '#1e1b4b', // Indigo node background
          fontFamily:
            "'Space Grotesk', system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
          fontSize: '18px', // Larger for presentation scale
          nodeBorder: '#22d3ee', // Cyan border
          clusterBkg: 'rgba(91, 33, 182, 0.3)', // Purple cluster
          clusterBorder: '#7c3aed', // Violet border
          edgeLabelBackground: '#1e1b4b',
          textColor: '#ffffff',
          titleColor: '#22d3ee', // Cyan titles
          nodeTextColor: '#ffffff',
        },
      }

    case 'phosphor':
      return {
        theme: 'dark',
        themeVariables: {
          // Phosphor: CRT terminal with P3 phosphor green (#39ff14)
          primaryColor: '#0a1a0a', // Very dark green for nodes
          primaryTextColor: '#39ff14', // P3 phosphor green
          primaryBorderColor: '#39ff14',
          lineColor: '#30d912', // Slightly dimmer green for lines
          secondaryColor: '#081408',
          tertiaryColor: '#050505',
          background: '#050505', // Softened CRT black
          mainBkg: '#0a1a0a', // Dark green node background
          fontFamily:
            "'JetBrains Mono', 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Liberation Mono', Menlo, monospace",
          fontSize: '18px', // Larger for presentation scale
          nodeBorder: '#39ff14', // Phosphor green border
          clusterBkg: '#081408',
          clusterBorder: '#228b22', // Dimmer green for clusters
          edgeLabelBackground: '#050505',
          textColor: '#39ff14',
          titleColor: '#39ff14',
          nodeTextColor: '#39ff14',
        },
      }

    case 'poster':
      return {
        theme: 'dark',
        themeVariables: {
          // Poster: High contrast with red accent (#ff4d4d), bold aesthetic
          primaryColor: '#0a0a0a', // Pure black nodes
          primaryTextColor: '#ffffff', // Pure white text
          primaryBorderColor: '#ffffff', // White borders for contrast
          lineColor: '#ff4d4d', // Red accent for lines
          secondaryColor: '#111111',
          tertiaryColor: '#000000',
          background: '#0a0a0a', // Dark background
          mainBkg: '#0a0a0a', // Black node background
          fontFamily:
            "'Inter', system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
          fontSize: '20px', // Extra large for bold poster aesthetic
          nodeBorder: '#ffffff', // White borders
          clusterBkg: '#111111',
          clusterBorder: '#ff4d4d', // Red cluster borders
          edgeLabelBackground: '#0a0a0a',
          textColor: '#ffffff',
          titleColor: '#ff4d4d', // Red titles
          nodeTextColor: '#ffffff',
        },
      }

    default:
      return {
        theme: 'default',
        themeVariables: {
          fontFamily:
            'Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
          fontSize: '18px',
        },
      }
  }
}

/**
 * Initialize mermaid with default configuration.
 * Uses startOnLoad: false for manual control over diagram rendering.
 * Safe to call multiple times - will only reinitialize if theme changes.
 *
 * @param theme Optional tap theme to use for styling diagrams
 */
export function initializeMermaid(theme?: Theme): void {
  // Skip if already initialized with the same theme
  if (isInitialized && currentTheme === theme) {
    return
  }

  const themeConfig = theme ? getMermaidTheme(theme) : { theme: 'default' as const, themeVariables: {} }

  mermaid.initialize({
    startOnLoad: false,
    securityLevel: 'strict',
    ...themeConfig,
    flowchart: {
      htmlLabels: true,
      nodeSpacing: 50,
      rankSpacing: 50,
      padding: 15,
      useMaxWidth: false,
      curve: 'basis',
      wrappingWidth: 300,
    },
  })

  isInitialized = true
  currentTheme = theme
}

/**
 * Check if mermaid has been initialized.
 */
export function isMermaidInitialized(): boolean {
  return isInitialized
}

/**
 * Reset initialization state (primarily for testing).
 */
export function resetMermaidInitialization(): void {
  isInitialized = false
  currentTheme = undefined
}

/**
 * Get the current theme used for mermaid initialization.
 */
export function getCurrentMermaidTheme(): Theme | undefined {
  return currentTheme
}

/**
 * Get the mermaid instance for direct access if needed.
 */
export function getMermaid() {
  return mermaid
}

/**
 * Counter for generating unique IDs for mermaid diagrams.
 */
let diagramCounter = 0

/**
 * Reset the diagram counter (primarily for testing).
 */
export function resetDiagramCounter(): void {
  diagramCounter = 0
}

/**
 * Result of rendering a mermaid diagram.
 */
export interface MermaidRenderResult {
  /** The rendered SVG string */
  svg: string
  /** Whether the render was successful */
  success: true
}

/**
 * Error result when mermaid rendering fails.
 */
export interface MermaidRenderError {
  /** Whether the render was successful */
  success: false
  /** The error message */
  error: string
  /** The original mermaid code */
  code: string
}

/**
 * Render a mermaid diagram from code.
 *
 * @param code The mermaid diagram code
 * @param theme Optional tap theme to use for styling
 * @returns Promise resolving to the rendered SVG or error
 */
export async function renderMermaidDiagram(
  code: string,
  theme?: Theme
): Promise<MermaidRenderResult | MermaidRenderError> {
  initializeMermaid(theme)

  const id = `mermaid-diagram-${++diagramCounter}`

  try {
    const { svg } = await mermaid.render(id, code)
    return { svg, success: true }
  } catch (err) {
    const errorMessage = err instanceof Error ? err.message : String(err)
    return {
      success: false,
      error: errorMessage,
      code,
    }
  }
}

/**
 * Find and render all mermaid code blocks within an element.
 * Replaces <pre><code class="language-mermaid"> blocks with rendered SVGs.
 * Also re-renders existing mermaid diagrams when theme changes.
 *
 * @param element The DOM element to search within
 * @param theme Optional tap theme to use for styling diagrams
 * @returns Promise resolving when all diagrams are rendered
 */
export async function renderMermaidBlocksInElement(
  element: HTMLElement,
  theme?: Theme
): Promise<void> {
  // Find all unrendered mermaid code blocks
  const codeBlocks = element.querySelectorAll<HTMLElement>(
    'pre > code.language-mermaid'
  )

  // Find all already-rendered diagrams that may need theme update
  const existingDiagrams = element.querySelectorAll<HTMLElement>(
    '.mermaid-diagram[data-mermaid-code]'
  )

  // Also find error containers that have stored code for retry
  const errorContainers = element.querySelectorAll<HTMLElement>(
    '.mermaid-error[data-mermaid-code]'
  )

  if (codeBlocks.length === 0 && existingDiagrams.length === 0 && errorContainers.length === 0) {
    return
  }

  // Render new mermaid code blocks
  const newBlockPromises = Array.from(codeBlocks).map(async (codeBlock) => {
    const pre = codeBlock.parentElement
    if (!pre) return

    const code = codeBlock.textContent ?? ''
    const result = await renderMermaidDiagram(code, theme)

    if (result.success) {
      // Create container for the rendered diagram, storing the code for re-rendering
      const container = document.createElement('div')
      container.className = 'mermaid-diagram'
      container.dataset.mermaidCode = code
      container.dataset.mermaidTheme = theme ?? ''
      container.innerHTML = result.svg
      // Fix foreignObject text clipping by expanding widths
      fixForeignObjectWidths(container)
      pre.replaceWith(container)
    } else {
      // Show error message, storing the code for potential re-render
      const errorContainer = document.createElement('div')
      errorContainer.className = 'mermaid-error'
      errorContainer.dataset.mermaidCode = code
      errorContainer.dataset.mermaidTheme = theme ?? ''
      errorContainer.innerHTML = `
        <div class="mermaid-error-message">Mermaid diagram error: ${escapeHtml(result.error)}</div>
        <pre class="mermaid-error-code"><code>${escapeHtml(result.code)}</code></pre>
      `
      pre.replaceWith(errorContainer)
    }
  })

  // Re-render existing diagrams if theme has changed
  const existingDiagramPromises = Array.from(existingDiagrams).map(async (diagram) => {
    const code = diagram.dataset.mermaidCode
    const previousTheme = diagram.dataset.mermaidTheme

    // Skip if no code stored or theme hasn't changed
    if (!code || previousTheme === (theme ?? '')) {
      return
    }

    const result = await renderMermaidDiagram(code, theme)

    if (result.success) {
      diagram.innerHTML = result.svg
      diagram.dataset.mermaidTheme = theme ?? ''
      // Fix foreignObject text clipping
      fixForeignObjectWidths(diagram)
    } else {
      // Convert to error container
      const errorContainer = document.createElement('div')
      errorContainer.className = 'mermaid-error'
      errorContainer.dataset.mermaidCode = code
      errorContainer.dataset.mermaidTheme = theme ?? ''
      errorContainer.innerHTML = `
        <div class="mermaid-error-message">Mermaid diagram error: ${escapeHtml(result.error)}</div>
        <pre class="mermaid-error-code"><code>${escapeHtml(result.code)}</code></pre>
      `
      diagram.replaceWith(errorContainer)
    }
  })

  // Re-render error containers (in case new theme fixes the issue)
  const errorContainerPromises = Array.from(errorContainers).map(async (errorContainer) => {
    const code = errorContainer.dataset.mermaidCode
    const previousTheme = errorContainer.dataset.mermaidTheme

    // Skip if no code stored or theme hasn't changed
    if (!code || previousTheme === (theme ?? '')) {
      return
    }

    const result = await renderMermaidDiagram(code, theme)

    if (result.success) {
      // Convert to successful diagram
      const container = document.createElement('div')
      container.className = 'mermaid-diagram'
      container.dataset.mermaidCode = code
      container.dataset.mermaidTheme = theme ?? ''
      container.innerHTML = result.svg
      errorContainer.replaceWith(container)
    } else {
      // Update error with new theme
      errorContainer.dataset.mermaidTheme = theme ?? ''
      const messageEl = errorContainer.querySelector('.mermaid-error-message')
      if (messageEl) {
        messageEl.textContent = `Mermaid diagram error: ${result.error}`
      }
    }
  })

  await Promise.all([...newBlockPromises, ...existingDiagramPromises, ...errorContainerPromises])
}

/**
 * Escape HTML special characters to prevent XSS.
 */
function escapeHtml(text: string): string {
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

/**
 * Fix mermaid foreignObject text clipping by expanding their widths.
 * Mermaid calculates text width using a generic font, but custom fonts may render wider.
 * This function expands foreignObject elements to ensure all text is visible.
 */
function fixForeignObjectWidths(container: HTMLElement): void {
  const foreignObjects = container.querySelectorAll('foreignObject')
  foreignObjects.forEach((fo) => {
    const text = fo.textContent?.trim() || ''
    if (text.length === 0) return

    const currentWidth = parseFloat(fo.getAttribute('width') || '0')
    // Calculate minimum width based on character count (roughly 14px per char at 18px font)
    const minWidth = text.length * 14
    const newWidth = Math.max(currentWidth * 1.6, minWidth)

    fo.setAttribute('width', String(newWidth))
  })
}
