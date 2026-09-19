import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'path'
import { readFileSync, readdirSync } from 'fs'

/**
 * Extracts a theme's slug from a module id (an absolute or relative path)
 * ending in .../lib/themes/<slug>/theme.css or .../theme.json, or null for
 * anything else. Used to name a theme's build output by its slug rather
 * than Vite's default chunk-naming, which collides across every theme
 * (every theme.css module shares the same base name "theme").
 */
function themeSlugFromModuleId(id: string | null | undefined): string | null {
  const match = /\/lib\/themes\/([^/]+)\/theme\.(css|json)$/.exec(id ?? '');
  return match ? match[1] : null;
}

/**
 * Drops the `.woff` entry from every @fontsource `@font-face` `src` list in
 * the built CSS, keeping only `.woff2`, and deletes the `.woff` asset files
 * that only existed for that now-removed reference. Every browser tap
 * supports reads `.woff2` (it has done so for years), so the `.woff`
 * fallback each @fontsource package ships alongside it is dead weight.
 *
 * A theme's fonts reach the bundle through a CSS `@import` inside
 * theme.css (e.g. `@import '@fontsource/jetbrains-mono/latin-500.css'`),
 * which Vite inlines and resolves to hashed asset URLs entirely inside its
 * own internal CSS plugin - a `transform` hook on the imported file never
 * fires for it, so a pre-build rewrite can't reach this content. Instead,
 * this runs as a `generateBundle` hook, after every CSS chunk and font
 * asset already exists in the bundle: it strips each `, url(...)
 * format('woff')` clause from the emitted CSS, then deletes any `.woff`
 * asset whose emitted file name no longer appears in any remaining CSS or
 * JS chunk, so a font that also happens to be referenced elsewhere (it
 * shouldn't be, but this is checked rather than assumed) is never removed
 * out from under a real reference.
 */
function fontsourceWoff2OnlyPlugin(): Plugin {
  // Matches ", url(<hashed-file>.woff) format('woff')" or the double-quoted
  // spelling, immediately after a .woff2 entry in the same src list.
  const woffFallbackPattern = /,\s*url\(([^)]+\.woff)\)\s*format\((['"])woff\2\)/g;

  return {
    name: 'fontsource-woff2-only',
    enforce: 'post',
    generateBundle(_, bundle) {
      const removedWoffFiles = new Set<string>();

      for (const chunk of Object.values(bundle)) {
        if (chunk.type !== 'asset' || !chunk.fileName.endsWith('.css') || typeof chunk.source !== 'string') continue;

        chunk.source = chunk.source.replace(woffFallbackPattern, (_match, urlPath: string) => {
          // urlPath is relative to the CSS file (e.g. "./name.woff" or
          // "name.woff"); every font asset in this build lives flat in
          // assets/, matching every CSS chunk's own location, so the
          // fileName Rollup uses for the asset is just its basename.
          const fileName = urlPath.replace(/^\.\//, '');
          removedWoffFiles.add(fileName.startsWith('assets/') ? fileName : `assets/${fileName}`);
          return '';
        });
      }

      if (removedWoffFiles.size === 0) return;

      // A .woff file is only safe to delete if no surviving chunk (CSS or
      // JS) still references its file name anywhere.
      const stillReferenced = new Set<string>();
      for (const chunk of Object.values(bundle)) {
        const source = chunk.type === 'asset' ? chunk.source : chunk.type === 'chunk' ? chunk.code : undefined;
        if (typeof source !== 'string') continue;
        for (const fileName of removedWoffFiles) {
          const baseName = fileName.replace(/^assets\//, '');
          if (source.includes(baseName)) stillReferenced.add(fileName);
        }
      }

      for (const fileName of removedWoffFiles) {
        if (stillReferenced.has(fileName)) continue;
        if (bundle[fileName]) delete bundle[fileName];
      }
    }
  };
}

/**
 * Lists every installed @fontsource and @fontsource-variable package's
 * node_modules path (e.g. "@fontsource/jetbrains-mono"), across every
 * theme's font imports. Scanning node_modules directly means a new theme's
 * font package needs no matching edit here: `npm install`-ing it is enough.
 */
function installedFontsourcePackages(): string[] {
  const scopes = ['@fontsource', '@fontsource-variable'];
  const paths: string[] = [];

  for (const scope of scopes) {
    const scopeDir = resolve(__dirname, `node_modules/${scope}`);
    try {
      for (const pkg of readdirSync(scopeDir)) {
        paths.push(`${scope}/${pkg}`);
      }
    } catch {
      // Scope directory doesn't exist; no packages from it installed.
    }
  }

  return paths;
}

/**
 * Safety-net Vite plugin for @fontsource font files.
 *
 * Vite normally resolves font url() references from @fontsource CSS imports
 * (imported via JS in each theme's theme.css) and emits them as separate
 * asset files. However, if Vite's own CSS bundling leaves any unresolved
 * ./files/ references in the final CSS, this plugin catches them, copies
 * the font files into the build output, and rewrites the URLs.
 *
 * Fonts are emitted as separate asset files rather than base64-inlined
 * (see assetsInlineLimit below): inlining every theme's fonts into one CSS
 * file produces a file large enough that browsers fail to parse the rest of
 * it, breaking themes defined later in the stylesheet.
 */
function fontsourceFallbackPlugin(): Plugin {
  return {
    name: 'fontsource-fallback',
    enforce: 'post',
    generateBundle(_, bundle) {
      const fontsourceDirs = installedFontsourcePackages();

      for (const [fileName, chunk] of Object.entries(bundle)) {
        if (chunk.type !== 'asset' || !fileName.endsWith('.css') || typeof chunk.source !== 'string') continue;

        const unresolvedRefs = [...chunk.source.matchAll(/url\(\.\/files\/([^)]+)\)/g)];
        if (unresolvedRefs.length === 0) continue;

        const emittedFonts = new Set<string>();

        for (const [, fontFile] of unresolvedRefs) {
          if (emittedFonts.has(fontFile)) continue;

          for (const pkg of fontsourceDirs) {
            try {
              const fontPath = resolve(__dirname, `node_modules/${pkg}/files/${fontFile}`);
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

// The Go dev server's port, for proxying /api and /ws when running the
// Vite dev server standalone against it (e.g. the theme check suite's two
// private servers). Defaults to tap dev's own default port.
const apiProxyTarget = `http://localhost:${process.env.TAP_API_PORT ?? 3000}`;

// https://vite.dev/config/
export default defineConfig({
  // Relative base so a build deployed under a URL sub path (GitHub Pages
  // project sites, a reverse-proxy path prefix) resolves its own assets
  // instead of looking for them at the domain root. index.html and
  // presenter.html are always served as the last path segment (a file, not
  // a directory, per URL resolution rules), so a relative "assets/..."
  // reference resolves against the directory containing that HTML file
  // whether it's served at "/", "/presenter", or nested under a sub path -
  // the same relative reference works for the live Go server and the
  // static build alike.
  base: './',
  // fontsourceFallbackPlugin resolves any stray ./files/ references left
  // in the CSS before fontsourceWoff2OnlyPlugin strips .woff references
  // and deletes their now-unreferenced assets, so a font the fallback
  // plugin still had to resolve is caught by the woff2-only pass too.
  plugins: [react(), fontsourceFallbackPlugin(), fontsourceWoff2OnlyPlugin()],
  server: {
    fs: {
      // Allow importing ../internal/layouts/layouts.json from the frontend
      allow: ['..'],
    },
    proxy: {
      '/api': { target: apiProxyTarget, changeOrigin: true },
      '/ws': { target: apiProxyTarget, ws: true },
      // Images and asciinema casts a slide references under /local/... are
      // served by the Go server from the presentation's base directory, not
      // built by Vite, so they 404 on the Vite port without this proxy.
      '/local': { target: apiProxyTarget, changeOrigin: true },
      // Deck component bundles (JS, CSS, source maps) are built and served
      // by the Go dev server, not Vite, so they 404 on the Vite port
      // without this proxy.
      '/components': { target: apiProxyTarget, changeOrigin: true },
    },
  },
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
        // The two entry points keep fixed, unhashed names: internal/server/routes.go
        // and internal/builder/builder.go serve embedded/dist's index.html and
        // presenter.html as-is, and those HTML files reference these entry
        // chunks by the names Vite itself just wrote into them, so nothing
        // outside this build needs to know the name in advance either way.
        entryFileNames: 'assets/[name].js',
        // Every other chunk (each theme's on-demand theme.css/theme.json
        // module, and any other lazy import() such as the asciinema-player
        // chunk) gets a content hash, so a cached static deployment can't
        // end up loading a stale chunk under a name that now means
        // something else after an upgrade. A theme's own chunk is named by
        // its slug rather than Vite's default (every theme.css module
        // resolves to the same base name "theme", so without this every
        // theme's chunk collided on one shared, build-order-numbered name
        // like "theme7.css" - unrelated to which theme it actually was).
        chunkFileNames: (chunkInfo) => {
          const slug = themeSlugFromModuleId(chunkInfo.facadeModuleId);
          return slug ? `assets/theme-${slug}-[hash].js` : 'assets/[name]-[hash].js';
        },
        assetFileNames: (assetInfo) => {
          const slug = themeSlugFromModuleId(assetInfo.originalFileNames?.[0]);
          if (slug) return `assets/theme-${slug}-[hash][extname]`;
          return 'assets/[name]-[hash][extname]';
        },
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
