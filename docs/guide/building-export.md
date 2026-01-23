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
├── index.html         # Main presentation entry point
├── assets/
│   ├── style.css      # Optimized presentation styles
│   └── main.js        # Bundled JavaScript
├── images/            # Copied image assets
└── fonts/             # Font files (if used)
```

### Build Options

| Flag | Description |
|------|-------------|
| `--out <dir>` | Custom output directory (default: `dist/`) |
| `--base <path>` | Base path for deployment (e.g., `/slides/`) |
| `--minify` | Enable additional minification |

**Example with custom output:**

```bash
tap build slides.md --out ./public --base /demo/
```

## Previewing the Build

Use `tap serve` to preview your built presentation locally before deploying:

```bash
tap serve dist
```

This starts a local HTTP server serving your built files, simulating a production environment.

### Serve Options

| Flag | Description |
|------|-------------|
| `--port <number>` | Port to serve on (default: 8080) |
| `--host <ip>` | Host to bind to (default: localhost) |

**Example:**

```bash
tap serve dist --port 3000
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

1. Build with the correct base path:
   ```bash
   tap build slides.md --base /your-repo-name/
   ```

2. Deploy the `dist/` directory to the `gh-pages` branch

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
tap pdf slides.md
```

This generates a high-quality PDF with each slide as a page.

### PDF Export Options

| Flag | Description |
|------|-------------|
| `--out <file>` | Output filename (default: `slides.pdf`) |
| `--format <type>` | Page format: `slides`, `notes`, `both` |
| `--paper <size>` | Paper size: `letter`, `a4`, `16:9`, `4:3` |
| `--margin <px>` | Page margins in pixels |

### Export Formats

**Slides only (default):**

```bash
tap pdf slides.md
```

Exports just the presentation slides, one per page.

**Notes only:**

```bash
tap pdf slides.md --format notes
```

Exports speaker notes as a document, useful for printing a script.

**Slides with notes:**

```bash
tap pdf slides.md --format both
```

Exports each slide with its corresponding speaker notes below, ideal for handouts or review materials.

### PDF Examples

```bash
# Basic PDF export
tap pdf presentation.md

# Custom output filename
tap pdf slides.md --out quarterly-review.pdf

# A4 paper with notes
tap pdf slides.md --format both --paper a4 --out handout.pdf

# Slides only, letter size
tap pdf slides.md --format slides --paper letter
```

::: tip
PDF export captures your presentation at a specific moment. If you have live code execution enabled, the results shown in the PDF will be whatever was displayed at export time.
:::

## Best Practices

### Pre-Deployment Checklist

1. **Test locally** - Run `tap serve dist` and verify everything works
2. **Check all links** - Ensure navigation and external links function
3. **Verify images** - Confirm all images load correctly
4. **Test responsiveness** - Check different screen sizes
5. **Review presenter mode** - Make sure `/presenter` route works

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
| `tap build slides.md --out ./public` | Build to custom directory |
| `tap serve dist` | Preview built presentation |
| `tap pdf slides.md` | Export to PDF (slides only) |
| `tap pdf slides.md --format notes` | Export notes only |
| `tap pdf slides.md --format both` | Export slides with notes |

## Next Steps

- [CLI Commands](/reference/cli-commands) - Complete command reference
- [Frontmatter Options](/reference/frontmatter-options) - Configure your presentation
- [Presenter Mode](/guide/presenter-mode) - Present your slides effectively
