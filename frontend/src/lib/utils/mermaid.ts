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

    case 'aurora':
      return {
        theme: 'forest',
        themeVariables: {
          // Aurora: Deep purple to teal gradient with glassmorphism feel
          primaryColor: '#14b8a6', // teal accent
          primaryTextColor: '#ffffff',
          primaryBorderColor: '#14b8a6', // teal accent
          lineColor: '#0ea5e9', // electric blue
          secondaryColor: '#4c1d95', // deep purple
          tertiaryColor: '#0f0a1f',
          background: '#0f0a1f',
          mainBkg: '#1e1b4b',
          fontFamily:
            "'Space Grotesk', system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
          fontSize: '18px',
          nodeBorder: '#14b8a6',
          clusterBkg: 'rgba(78, 29, 149, 0.3)',
          clusterBorder: '#0ea5e9',
          edgeLabelBackground: '#1e1b4b',
          textColor: '#ffffff',
          titleColor: '#14b8a6',
          nodeTextColor: '#ffffff',
        },
      }

    case 'phosphor':
      return {
        theme: 'dark',
        themeVariables: {
          // Phosphor: CRT terminal aesthetic with green phosphor glow
          primaryColor: '#001a00', // very dark green for nodes
          primaryTextColor: '#00ff00', // phosphor green
          primaryBorderColor: '#00ff00',
          lineColor: '#00ff00',
          secondaryColor: '#001100',
          tertiaryColor: '#000000',
          background: '#000000',
          mainBkg: '#001a00',
          fontFamily:
            "'JetBrains Mono', 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Liberation Mono', Menlo, monospace",
          fontSize: '16px',
          nodeBorder: '#00ff00',
          clusterBkg: '#001100',
          clusterBorder: '#00ff00',
          edgeLabelBackground: '#000000',
          textColor: '#00ff00',
          titleColor: '#00ff00',
          nodeTextColor: '#00ff00',
        },
      }

    case 'poster':
      return {
        theme: 'default',
        themeVariables: {
          // Poster: High contrast editorial design with warm coral accent
          primaryColor: '#fafafa', // near-white for nodes
          primaryTextColor: '#0d0d0f', // dark text on light nodes
          primaryBorderColor: '#ff6b4a', // coral accent
          lineColor: '#ff6b4a',
          secondaryColor: '#f5f5f5',
          tertiaryColor: '#ffffff',
          background: '#0d0d0f',
          mainBkg: '#fafafa',
          fontFamily:
            "'Inter', system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
          fontSize: '16px',
          nodeBorder: '#ff6b4a',
          clusterBkg: '#f5f5f5',
          clusterBorder: '#ff6b4a',
          edgeLabelBackground: '#0d0d0f',
          textColor: '#fafafa',
          titleColor: '#ff6b4a',
          nodeTextColor: '#0d0d0f',
        },
      }

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
