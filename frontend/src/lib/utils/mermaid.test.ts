/**
 * Unit tests for mermaid initialization and configuration.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  initializeMermaid,
  isMermaidInitialized,
  resetMermaidInitialization,
  getMermaid,
  renderMermaidDiagram,
  renderMermaidBlocksInElement,
  resetDiagramCounter,
  getMermaidTheme,
  getCurrentMermaidTheme,
} from './mermaid'

// Mock mermaid module
vi.mock('mermaid', () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(),
  },
}))

describe('getMermaidTheme', () => {
  describe('paper theme', () => {
    it('returns neutral mermaid theme for paper', () => {
      const config = getMermaidTheme('paper')
      expect(config.theme).toBe('neutral')
    })

    it('uses light colors for paper theme', () => {
      const config = getMermaidTheme('paper')
      expect(config.themeVariables.background).toBe('#ffffff')
      expect(config.themeVariables.primaryTextColor).toBe('#0a0a0a')
      expect(config.themeVariables.textColor).toBe('#0a0a0a')
    })

    it('uses stone accent color for paper theme', () => {
      const config = getMermaidTheme('paper')
      expect(config.themeVariables.primaryBorderColor).toBe('#78716c')
      expect(config.themeVariables.lineColor).toBe('#78716c')
      expect(config.themeVariables.nodeBorder).toBe('#78716c')
    })

    it('uses Inter font family for paper theme', () => {
      const config = getMermaidTheme('paper')
      expect(config.themeVariables.fontFamily).toContain('Inter')
    })
  })

  describe('noir theme', () => {
    it('returns dark mermaid theme for noir', () => {
      const config = getMermaidTheme('noir')
      expect(config.theme).toBe('dark')
    })

    it('uses dark colors for noir theme', () => {
      const config = getMermaidTheme('noir')
      expect(config.themeVariables.background).toBe('#0a0a0a')
      expect(config.themeVariables.primaryTextColor).toBe('#fafafa')
      expect(config.themeVariables.textColor).toBe('#fafafa')
    })

    it('uses gold accent color for noir theme', () => {
      const config = getMermaidTheme('noir')
      expect(config.themeVariables.primaryBorderColor).toBe('#d4af37')
      expect(config.themeVariables.lineColor).toBe('#d4af37')
      expect(config.themeVariables.nodeBorder).toBe('#d4af37')
      expect(config.themeVariables.titleColor).toBe('#d4af37')
    })

    it('uses Inter font family for noir theme', () => {
      const config = getMermaidTheme('noir')
      expect(config.themeVariables.fontFamily).toContain('Inter')
    })
  })

  describe('default theme fallback', () => {
    it('returns default mermaid theme for aurora (placeholder)', () => {
      const config = getMermaidTheme('aurora')
      expect(config.theme).toBe('default')
    })

    it('returns default mermaid theme for phosphor (placeholder)', () => {
      const config = getMermaidTheme('phosphor')
      expect(config.theme).toBe('default')
    })

    it('returns default mermaid theme for poster (placeholder)', () => {
      const config = getMermaidTheme('poster')
      expect(config.theme).toBe('default')
    })

    it('includes font family even for default themes', () => {
      const config = getMermaidTheme('aurora')
      expect(config.themeVariables.fontFamily).toContain('Inter')
    })
  })
})

describe('mermaid initialization', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    resetMermaidInitialization()
    resetDiagramCounter()
  })

  describe('initializeMermaid', () => {
    it('initializes mermaid with startOnLoad: false', async () => {
      const mermaid = await import('mermaid')

      initializeMermaid()

      expect(mermaid.default.initialize).toHaveBeenCalledWith(
        expect.objectContaining({
          startOnLoad: false,
          securityLevel: 'strict',
        })
      )
    })

    it('only initializes once when called multiple times without theme', async () => {
      const mermaid = await import('mermaid')

      initializeMermaid()
      initializeMermaid()
      initializeMermaid()

      expect(mermaid.default.initialize).toHaveBeenCalledTimes(1)
    })

    it('reinitializes when theme changes', async () => {
      const mermaid = await import('mermaid')

      initializeMermaid('paper')
      initializeMermaid('noir')

      expect(mermaid.default.initialize).toHaveBeenCalledTimes(2)
    })

    it('does not reinitialize when called with the same theme', async () => {
      const mermaid = await import('mermaid')

      initializeMermaid('paper')
      initializeMermaid('paper')

      expect(mermaid.default.initialize).toHaveBeenCalledTimes(1)
    })

    it('uses paper theme config when specified', async () => {
      const mermaid = await import('mermaid')

      initializeMermaid('paper')

      expect(mermaid.default.initialize).toHaveBeenCalledWith(
        expect.objectContaining({
          theme: 'neutral',
          themeVariables: expect.objectContaining({
            background: '#ffffff',
          }),
        })
      )
    })

    it('uses noir theme config when specified', async () => {
      const mermaid = await import('mermaid')

      initializeMermaid('noir')

      expect(mermaid.default.initialize).toHaveBeenCalledWith(
        expect.objectContaining({
          theme: 'dark',
          themeVariables: expect.objectContaining({
            primaryBorderColor: '#d4af37',
          }),
        })
      )
    })
  })

  describe('getCurrentMermaidTheme', () => {
    it('returns undefined before initialization', () => {
      expect(getCurrentMermaidTheme()).toBeUndefined()
    })

    it('returns undefined after initialization without theme', () => {
      initializeMermaid()
      expect(getCurrentMermaidTheme()).toBeUndefined()
    })

    it('returns the theme after initialization with theme', () => {
      initializeMermaid('paper')
      expect(getCurrentMermaidTheme()).toBe('paper')
    })

    it('returns the latest theme after theme change', () => {
      initializeMermaid('paper')
      initializeMermaid('noir')
      expect(getCurrentMermaidTheme()).toBe('noir')
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

describe('mermaid rendering', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    resetMermaidInitialization()
    resetDiagramCounter()
  })

  describe('renderMermaidDiagram', () => {
    it('renders a valid mermaid diagram', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockResolvedValue({ svg: '<svg>test diagram</svg>' })

      const result = await renderMermaidDiagram('graph TD\nA-->B')

      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.svg).toBe('<svg>test diagram</svg>')
      }
    })

    it('generates unique IDs for each diagram', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockResolvedValue({ svg: '<svg></svg>' })

      await renderMermaidDiagram('graph TD\nA-->B')
      await renderMermaidDiagram('graph TD\nC-->D')

      expect(mockRender).toHaveBeenNthCalledWith(1, 'mermaid-diagram-1', expect.any(String))
      expect(mockRender).toHaveBeenNthCalledWith(2, 'mermaid-diagram-2', expect.any(String))
    })

    it('returns error result when rendering fails', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockRejectedValue(new Error('Parse error'))

      const result = await renderMermaidDiagram('invalid diagram')

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.error).toBe('Parse error')
        expect(result.code).toBe('invalid diagram')
      }
    })

    it('handles non-Error exceptions', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockRejectedValue('string error')

      const result = await renderMermaidDiagram('invalid diagram')

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.error).toBe('string error')
      }
    })

    it('initializes mermaid before rendering', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockResolvedValue({ svg: '<svg></svg>' })

      expect(isMermaidInitialized()).toBe(false)
      await renderMermaidDiagram('graph TD\nA-->B')
      expect(isMermaidInitialized()).toBe(true)
    })
  })

  describe('renderMermaidBlocksInElement', () => {
    it('renders mermaid code blocks in an element', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockResolvedValue({ svg: '<svg class="rendered">flowchart</svg>' })

      // Create DOM element with mermaid code block
      const container = document.createElement('div')
      container.innerHTML = `
        <pre><code class="language-mermaid">graph TD
A-->B</code></pre>
      `

      await renderMermaidBlocksInElement(container)

      // Check that the pre was replaced with a diagram container
      expect(container.querySelector('pre')).toBeNull()
      const diagram = container.querySelector('.mermaid-diagram')
      expect(diagram).not.toBeNull()
      expect(diagram?.innerHTML).toBe('<svg class="rendered">flowchart</svg>')
    })

    it('handles multiple mermaid blocks', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockResolvedValue({ svg: '<svg>diagram</svg>' })

      const container = document.createElement('div')
      container.innerHTML = `
        <pre><code class="language-mermaid">graph TD\nA-->B</code></pre>
        <p>Some text</p>
        <pre><code class="language-mermaid">graph TD\nC-->D</code></pre>
      `

      await renderMermaidBlocksInElement(container)

      const diagrams = container.querySelectorAll('.mermaid-diagram')
      expect(diagrams.length).toBe(2)
      expect(mockRender).toHaveBeenCalledTimes(2)
    })

    it('does nothing when no mermaid blocks exist', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)

      const container = document.createElement('div')
      container.innerHTML = `
        <pre><code class="language-javascript">const x = 1;</code></pre>
      `

      await renderMermaidBlocksInElement(container)

      expect(mockRender).not.toHaveBeenCalled()
      expect(container.querySelector('pre')).not.toBeNull()
    })

    it('shows error message when rendering fails', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockRejectedValue(new Error('Syntax error'))

      const container = document.createElement('div')
      container.innerHTML = `
        <pre><code class="language-mermaid">invalid syntax</code></pre>
      `

      await renderMermaidBlocksInElement(container)

      // Check that error container was created
      expect(container.querySelector('pre > code.language-mermaid')).toBeNull()
      const errorContainer = container.querySelector('.mermaid-error')
      expect(errorContainer).not.toBeNull()
      expect(errorContainer?.querySelector('.mermaid-error-message')?.textContent).toContain('Syntax error')
      expect(errorContainer?.querySelector('.mermaid-error-code code')?.textContent).toBe('invalid syntax')
    })

    it('escapes HTML in error messages', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockRejectedValue(new Error('<script>alert("xss")</script>'))

      const container = document.createElement('div')
      container.innerHTML = `
        <pre><code class="language-mermaid"><script>bad</script></code></pre>
      `

      await renderMermaidBlocksInElement(container)

      const errorMessage = container.querySelector('.mermaid-error-message')
      expect(errorMessage?.innerHTML).not.toContain('<script>')
      expect(errorMessage?.textContent).toContain('<script>')
    })

    it('preserves non-mermaid code blocks', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockResolvedValue({ svg: '<svg>diagram</svg>' })

      const container = document.createElement('div')
      container.innerHTML = `
        <pre><code class="language-javascript">const x = 1;</code></pre>
        <pre><code class="language-mermaid">graph TD\nA-->B</code></pre>
        <pre><code class="language-python">print("hello")</code></pre>
      `

      await renderMermaidBlocksInElement(container)

      // Mermaid block should be replaced
      expect(container.querySelector('.mermaid-diagram')).not.toBeNull()
      // Other code blocks should remain
      expect(container.querySelector('code.language-javascript')).not.toBeNull()
      expect(container.querySelector('code.language-python')).not.toBeNull()
    })
  })

  describe('resetDiagramCounter', () => {
    it('resets the diagram counter', async () => {
      const mermaid = await import('mermaid')
      const mockRender = vi.mocked(mermaid.default.render)
      mockRender.mockResolvedValue({ svg: '<svg></svg>' })

      await renderMermaidDiagram('graph TD\nA-->B')
      resetDiagramCounter()
      await renderMermaidDiagram('graph TD\nC-->D')

      // After reset, IDs should start from 1 again
      expect(mockRender).toHaveBeenNthCalledWith(1, 'mermaid-diagram-1', expect.any(String))
      expect(mockRender).toHaveBeenNthCalledWith(2, 'mermaid-diagram-1', expect.any(String))
    })
  })
})
