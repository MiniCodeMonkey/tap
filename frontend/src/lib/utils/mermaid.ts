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
