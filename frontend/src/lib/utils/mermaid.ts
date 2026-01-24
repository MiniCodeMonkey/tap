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
          // Paper: Clean light theme with warm stone accent
          primaryColor: '#f5f5f4', // stone-100
          primaryTextColor: '#0a0a0a',
          primaryBorderColor: '#78716c', // stone-500 (accent)
          lineColor: '#78716c',
          secondaryColor: '#fafafa',
          tertiaryColor: '#ffffff',
          background: '#ffffff',
          mainBkg: '#f5f5f4',
          fontFamily:
            'Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
          fontSize: '16px',
          nodeBorder: '#78716c',
          clusterBkg: '#fafaf9',
          clusterBorder: '#d6d3d1',
          edgeLabelBackground: '#ffffff',
          textColor: '#0a0a0a',
          titleColor: '#0a0a0a',
          nodeTextColor: '#0a0a0a',
        },
      }

    case 'noir':
      return {
        theme: 'dark',
        themeVariables: {
          // Noir: Cinematic dark theme with gold accent
          primaryColor: '#1a1a1a',
          primaryTextColor: '#fafafa',
          primaryBorderColor: '#d4af37', // gold accent
          lineColor: '#d4af37',
          secondaryColor: '#111111',
          tertiaryColor: '#0a0a0a',
          background: '#0a0a0a',
          mainBkg: '#161616',
          fontFamily:
            'Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
          fontSize: '16px',
          nodeBorder: '#d4af37',
          clusterBkg: '#111111',
          clusterBorder: '#d4af37',
          edgeLabelBackground: '#161616',
          textColor: '#fafafa',
          titleColor: '#d4af37',
          nodeTextColor: '#fafafa',
        },
      }

    // Placeholder for future themes (US-006)
    case 'aurora':
    case 'phosphor':
    case 'poster':
    default:
      return {
        theme: 'default',
        themeVariables: {
          fontFamily:
            'Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
          fontSize: '16px',
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
 *
 * @param element The DOM element to search within
 * @param theme Optional tap theme to use for styling diagrams
 * @returns Promise resolving when all diagrams are rendered
 */
export async function renderMermaidBlocksInElement(
  element: HTMLElement,
  theme?: Theme
): Promise<void> {
  // Find all mermaid code blocks
  const codeBlocks = element.querySelectorAll<HTMLElement>(
    'pre > code.language-mermaid'
  )

  if (codeBlocks.length === 0) {
    return
  }

  const renderPromises = Array.from(codeBlocks).map(async (codeBlock) => {
    const pre = codeBlock.parentElement
    if (!pre) return

    const code = codeBlock.textContent ?? ''
    const result = await renderMermaidDiagram(code, theme)

    if (result.success) {
      // Create container for the rendered diagram
      const container = document.createElement('div')
      container.className = 'mermaid-diagram'
      container.innerHTML = result.svg
      pre.replaceWith(container)
    } else {
      // Show error message
      const errorContainer = document.createElement('div')
      errorContainer.className = 'mermaid-error'
      errorContainer.innerHTML = `
        <div class="mermaid-error-message">Mermaid diagram error: ${escapeHtml(result.error)}</div>
        <pre class="mermaid-error-code"><code>${escapeHtml(result.code)}</code></pre>
      `
      pre.replaceWith(errorContainer)
    }
  })

  await Promise.all(renderPromises)
}

/**
 * Escape HTML special characters to prevent XSS.
 */
function escapeHtml(text: string): string {
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}
