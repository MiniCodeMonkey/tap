import type { Plugin } from 'vite'
import { readFile } from 'fs/promises'
import { resolve, relative } from 'path'

/**
 * Vite plugin that injects raw markdown content into VitePress pages.
 * The content is available as window.__DOC_RAW at runtime.
 */
export function rawMarkdownPlugin(): Plugin {
  const docsDir = resolve(__dirname, '../..')

  return {
    name: 'vitepress-raw-markdown',
    enforce: 'pre',

    async transform(code, id) {
      // Only process markdown files in the docs directory
      if (!id.endsWith('.md') || !id.includes(docsDir)) {
        return null
      }

      // Skip the homepage
      const relativePath = relative(docsDir, id)
      if (relativePath === 'index.md') {
        return null
      }

      try {
        // Read the raw markdown content
        const rawContent = await readFile(id, 'utf-8')

        // Use JSON.stringify for safe embedding - handles all escaping correctly
        const jsonEncoded = JSON.stringify(rawContent)

        // Script block to inject (will be placed after frontmatter)
        const scriptBlock = `
<script setup>
import { onMounted } from 'vue'

onMounted(() => {
  if (typeof window !== 'undefined') {
    window.__DOC_RAW = ${jsonEncoded}
  }
})
</script>
`

        // Check if the file has frontmatter (starts with ---)
        const frontmatterMatch = code.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n/)

        if (frontmatterMatch) {
          // Insert script block after frontmatter
          const frontmatterEnd = frontmatterMatch[0].length
          return code.slice(0, frontmatterEnd) + scriptBlock + code.slice(frontmatterEnd)
        } else {
          // No frontmatter, prepend script block
          return scriptBlock + code
        }
      } catch (err) {
        console.error(`Failed to read markdown file: ${id}`, err)
        return null
      }
    }
  }
}
