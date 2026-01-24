/**
 * Unit tests for mermaid initialization and configuration.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  initializeMermaid,
  isMermaidInitialized,
  resetMermaidInitialization,
  getMermaid,
} from './mermaid'

// Mock mermaid module
vi.mock('mermaid', () => ({
  default: {
    initialize: vi.fn(),
  },
}))

describe('mermaid initialization', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    resetMermaidInitialization()
  })

  describe('initializeMermaid', () => {
    it('initializes mermaid with startOnLoad: false', async () => {
      const mermaid = await import('mermaid')

      initializeMermaid()

      expect(mermaid.default.initialize).toHaveBeenCalledWith({
        startOnLoad: false,
        theme: 'default',
        securityLevel: 'strict',
      })
    })

    it('only initializes once when called multiple times', async () => {
      const mermaid = await import('mermaid')

      initializeMermaid()
      initializeMermaid()
      initializeMermaid()

      expect(mermaid.default.initialize).toHaveBeenCalledTimes(1)
    })
  })

  describe('isMermaidInitialized', () => {
    it('returns false before initialization', () => {
      expect(isMermaidInitialized()).toBe(false)
    })

    it('returns true after initialization', () => {
      initializeMermaid()
      expect(isMermaidInitialized()).toBe(true)
    })
  })

  describe('resetMermaidInitialization', () => {
    it('resets initialization state', () => {
      initializeMermaid()
      expect(isMermaidInitialized()).toBe(true)

      resetMermaidInitialization()
      expect(isMermaidInitialized()).toBe(false)
    })

    it('allows re-initialization after reset', async () => {
      const mermaid = await import('mermaid')

      initializeMermaid()
      resetMermaidInitialization()
      initializeMermaid()

      expect(mermaid.default.initialize).toHaveBeenCalledTimes(2)
    })
  })

  describe('getMermaid', () => {
    it('returns the mermaid instance', async () => {
      const mermaid = await import('mermaid')
      const instance = getMermaid()
      expect(instance).toBe(mermaid.default)
    })
  })
})
