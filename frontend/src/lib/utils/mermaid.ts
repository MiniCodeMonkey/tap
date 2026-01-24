/**
 * Mermaid diagram initialization and configuration utilities.
 * Handles mermaid.js setup with manual initialization (startOnLoad: false).
 */
import mermaid from 'mermaid'

let isInitialized = false

/**
 * Initialize mermaid with default configuration.
 * Uses startOnLoad: false for manual control over diagram rendering.
 * Safe to call multiple times - will only initialize once.
 */
export function initializeMermaid(): void {
  if (isInitialized) {
    return
  }

  mermaid.initialize({
    startOnLoad: false,
    theme: 'default',
    securityLevel: 'strict',
  })

  isInitialized = true
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
 * @returns Promise resolving to the rendered SVG or error
 */
export async function renderMermaidDiagram(
  code: string
): Promise<MermaidRenderResult | MermaidRenderError> {
  initializeMermaid()

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
 * @returns Promise resolving when all diagrams are rendered
 */
export async function renderMermaidBlocksInElement(
  element: HTMLElement
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
    const result = await renderMermaidDiagram(code)

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
