import { defineConfig, type Plugin } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { resolve } from 'path'
import { readFileSync } from 'fs'

/**
 * Safety-net Vite plugin for @fontsource font files.
 *
 * Vite normally resolves font url() references from @fontsource CSS imports
 * (imported via JS in main.ts/presenter.ts) and emits them as separate asset
 * files. However, if Tailwind CSS v4's @import processing leaves any
 * unresolved ./files/ references in the final CSS, this plugin catches them,
 * copies the font files into the build output, and rewrites the URLs.
 *
 * History: Previously this plugin base64-inlined ALL font files, which created
 * a ~30MB CSS file that browsers couldn't fully parse, breaking themes defined
 * later in the stylesheet. The fix was to lower assetsInlineLimit so Vite
 * emits fonts as separate files natively.
 */
function fontsourceFallbackPlugin(): Plugin {
  return {
    name: 'fontsource-fallback',
    enforce: 'post',
    generateBundle(_, bundle) {
      // All @fontsource packages used across themes
      const fontsourceDirs = [
        'inter', 'space-grotesk', 'jetbrains-mono', 'anton',
        'noto-serif-jp', 'bebas-neue', 'playfair-display', 'source-serif-pro',
        'instrument-sans', 'ibm-plex-sans', 'ibm-plex-mono',
        'sora', 'fira-code', 'outfit', 'plus-jakarta-sans'
      ];

      for (const [fileName, chunk] of Object.entries(bundle)) {
        if (chunk.type !== 'asset' || !fileName.endsWith('.css') || typeof chunk.source !== 'string') continue;

        const unresolvedRefs = [...chunk.source.matchAll(/url\(\.\/files\/([^)]+)\)/g)];
        if (unresolvedRefs.length === 0) continue;

        const emittedFonts = new Set<string>();

        for (const [, fontFile] of unresolvedRefs) {
          if (emittedFonts.has(fontFile)) continue;

          for (const pkg of fontsourceDirs) {
            try {
              const fontPath = resolve(__dirname, `node_modules/@fontsource/${pkg}/files/${fontFile}`);
              const fontData = readFileSync(fontPath);
              const emitName = `assets/${fontFile}`;
              bundle[emitName] = {
                type: 'asset',
                fileName: emitName,
                name: fontFile,
                source: fontData,
                needsCodeReference: false,
              } as any;
              emittedFonts.add(fontFile);
              break;
            } catch {
              // Not in this package, try next
            }
          }
        }

        // Rewrite unresolved ./files/ references to point to emitted assets
        chunk.source = chunk.source.replace(
          /url\(\.\/files\/([^)]+)\)/g,
          (match, fontFile: string) => {
            if (emittedFonts.has(fontFile)) {
              return `url(./${fontFile})`;
            }
            return match;
          }
        );
      }
    }
  };
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte(), fontsourceFallbackPlugin()],
  build: {
    // Output to embedded directory for go:embed
    outDir: '../embedded/dist',
    emptyOutDir: true,
    // Don't inline font assets as base64 - serve them as separate files.
    // The previous value of 100000 caused all fonts to be base64-inlined
    // into a ~30MB CSS file that broke theme parsing.
    assetsInlineLimit: 4096,
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        presenter: resolve(__dirname, 'presenter.html'),
      },
      output: {
        // Single JS and CSS file output
        entryFileNames: 'assets/[name].js',
        chunkFileNames: 'assets/[name].js',
        assetFileNames: 'assets/[name].[ext]',
      },
    },
  },
  // Resolve aliases for cleaner imports
  resolve: {
    alias: {
      '$lib': resolve(__dirname, 'src/lib'),
    },
  },
})
