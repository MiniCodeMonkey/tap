---
title: Building & Export
---

# Building & Export

Build and export your presentations for deployment or offline sharing.

## Building for Production

Use the `tap build` command to create a production-ready static version of your presentation:

```bash
tap build slides.md
```

This generates optimized HTML, CSS, and JavaScript that can be deployed to any static hosting provider.

### Build Output Structure

After building, your presentation is output to the `dist/` directory:

```
dist/
├── index.html         # Audience view, with the deck's JSON embedded
├── presenter.html     # Presenter view (needs `tap dev`; see below)
├── assets/            # Hashed JS and CSS chunks, bundled fonts, and copied images
└── components/        # Bundled deck components, when the deck uses any
```

Everything the deck needs is in that folder. The fonts, the syntax
highlighter, and the asciinema player are all bundled, so a built deck
needs no network at all.

**A build works from any URL path.** Every asset reference is relative, so
the same `dist/` folder serves correctly from a domain root, from a
sub-path like `https://example.com/talks/q3/`, and from a GitHub Pages
project site like `https://user.github.io/your-repo/`. There is no
base-path flag to set and no rewriting to do after the fact.

### Build Options

| Flag | Short | Description |
|------|-------|-------------|
| `--output <dir>` | `-o` | Output directory (default: `dist`) |

**Example with custom output:**

```bash
tap build slides.md --output ./public
```

### What a Static Build Cannot Do

- **Live code execution.** A code block with a `driver` shows its code as
  written; nothing runs. Use `tap dev` for a live demo.
- **Hot reload and cross-device sync.** There is no server, so the viewer
  never opens a websocket and never tries to reconnect.
- **The presenter view.** `presenter.html` is written into the build, but
  it fetches the presentation from `/api/presentation`, which only
  `tap dev` serves, so it cannot load from a static build. The presenter
  view, with its notes, timer, and next-slide panel, is a `tap dev`
  feature. Export notes with `tap export pdf --content notes` or
  `--content both` if you need them alongside a static deck.
- **The AI image generator and the slide builder.** Both live in the
  `tap dev` terminal.

Everything else works, including themes, the `t` key, fragments, steps,
deck components, mermaid, asciinema, and the overview.

`tap build` exits with status 1, printing to standard error, when a slide
names an unknown layout, uses a slot its layout does not declare, or has a
deck component that fails to build.

## Previewing the Build

Use `tap serve` to preview your built presentation locally before deploying:

```bash
tap serve dist
```

This starts a local HTTP server serving your built files, simulating a production environment.

### Serve Options

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port to serve on (default: `3000`) |

**Example:**

```bash
tap serve dist --port 8080
```

::: tip
Use `tap serve` to verify your presentation works correctly before deploying to production. This catches issues like broken asset paths or base URL misconfiguration.
:::

## Deploying to Static Hosts

Tap presentations are static HTML files that work on any static hosting provider.

### Netlify

1. Build your presentation:
   ```bash
   tap build slides.md
   ```

2. Deploy the `dist/` directory:
   ```bash
   npx netlify deploy --dir=dist --prod
   ```

Or configure automatic deployments via `netlify.toml`:

```toml
[build]
  command = "tap build slides.md"
  publish = "dist"

[[redirects]]
  from = "/*"
  to = "/index.html"
  status = 200
```

### Vercel

```bash
tap build slides.md
npx vercel dist
```

### GitHub Pages

1. Build the deck:
   ```bash
   tap build slides.md
   ```

2. Deploy the `dist/` directory to the `gh-pages` branch. Every asset path
   in the build is relative, so a project page under
   `https://user.github.io/your-repo/` works without extra configuration.

### Any Static Host

Tap presentations work anywhere static files are served:
- AWS S3 + CloudFront
- Cloudflare Pages
- Firebase Hosting
- Your own nginx/Apache server

Simply upload the contents of the `dist/` directory to your web root.

## PDF Export

Export your presentation to PDF for offline sharing or printing:

```bash
tap export pdf slides.md
```

This generates a high-quality PDF with each slide as a page.

### PDF Export Options

| Flag | Short | Description |
|------|-------|-------------|
| `--output <file>` | `-o` | Output filename (default: the deck's name with a `.pdf` extension) |
| `--content <type>` | | What to include: `slides` (default), `notes`, or `both` |

Page size follows the deck's own `aspectRatio`, so there is no paper-size
flag.

### Export Formats

**Slides only (default):**

```bash
tap export pdf slides.md
```

Exports just the presentation slides, one per page.

**Notes only:**

```bash
tap export pdf slides.md --content notes
```

Exports speaker notes as a document, useful for printing a script.

**Slides with notes:**

```bash
tap export pdf slides.md --content both
```

Exports each slide with its corresponding speaker notes below, ideal for handouts or review materials.

### PDF Examples

```bash
# Basic PDF export
tap export pdf presentation.md

# Custom output filename
tap export pdf slides.md --output quarterly-review.pdf

# Slides with notes below each one, as a handout
tap export pdf slides.md --content both --output handout.pdf

# Speaker notes only, as a script
tap export pdf slides.md --content notes --output script.pdf
```

## Screenshotting One Slide

`tap export images` renders a single slide state to a PNG through the same
headless browser, which is faster than a full PDF when you only want to
check one slide:

```bash
tap export images slides.md --slide 12
tap export images slides.md --slide 12 --step 3
tap export images slides.md --all --output shots/
```

It exits with status 1 when the slide shows an error card, so it works as
a check in a script. See [CLI Commands](/reference/cli-commands#tap-export-images).

::: tip
PDF export captures your presentation at a specific moment. If you have live code execution enabled, the results shown in the PDF will be whatever was displayed at export time.
:::

## Best Practices

### Pre-Deployment Checklist

1. **Test locally** - Run `tap serve dist` and verify everything works
2. **Check all links** - Ensure navigation and external links function
3. **Verify images** - Confirm all images load correctly
4. **Test responsiveness** - Check different screen sizes
5. **Review presenter mode** - Run `tap dev` and check `/presenter`; it does not work from a static build

### Optimization Tips

- Use optimized images (WebP, compressed PNG/JPG)
- Keep presentations focused to reduce bundle size
- Test on target deployment platform before the presentation day

### Version Control

Consider committing your built files or using CI/CD:

```bash
# Option 1: Commit dist/ to repo
tap build slides.md
git add dist/
git commit -m "Update built presentation"

# Option 2: Build in CI/CD pipeline
# Let your CI service build and deploy automatically
```

## Quick Reference

| Command | Description |
|---------|-------------|
| `tap build slides.md` | Build for production |
| `tap build slides.md --output ./public` | Build to custom directory |
| `tap serve dist` | Preview built presentation |
| `tap export pdf slides.md` | Export to PDF (slides only) |
| `tap export pdf slides.md --content notes` | Export notes only |
| `tap export pdf slides.md --content both` | Export slides with notes |
| `tap export images slides.md --slide 4` | Render one slide to a PNG |

## Next Steps

- [CLI Commands](/reference/cli-commands) - Complete command reference
- [Frontmatter Options](/reference/frontmatter-options) - Configure your presentation
- [Presenter Mode](/guide/presenter-mode) - Present your slides effectively
