---
title: CLI Commands
---

# CLI Commands

Complete reference for all Tap CLI commands.

## Overview

Tap provides a simple CLI for creating, developing, building, and exporting presentations:

```bash
tap <command> [options]
```

Run `tap --help` to see all available commands, or `tap <command> --help` for command-specific help.

## tap new

Create a new presentation from a template.

### Usage

```bash
tap new [name]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `name` | Name for the new presentation file (optional) |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--theme <name>` | `-t` | Theme to use (default: `paper`) |
| `--template <type>` | | Template type: `blank`, `demo`, `talk` (default: `blank`) |
| `--dir <path>` | `-d` | Directory to create the file in (default: current directory) |
| `--force` | `-f` | Overwrite existing file if it exists |

### Examples

```bash
# Create presentation with default name (untitled.md)
tap new

# Create named presentation
tap new my-talk

# Create with specific theme
tap new quarterly-review --theme gradient

# Create from demo template with all features
tap new demo-presentation --template demo

# Create in specific directory
tap new slides --dir ./presentations

# Overwrite existing file
tap new slides --force
```

### Output

Creates a new markdown file with frontmatter and sample content:

```markdown
---
title: My Talk
theme: paper
---

# Welcome

Your first slide content here.

---

# Second Slide

More content...
```

---

## tap dev

Start the development server with live reload for real-time preview.

### Usage

```bash
tap dev <file>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `file` | Path to the markdown presentation file (required) |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port to serve on (default: `3000`) |
| `--host <ip>` | | Host to bind to (default: `localhost`) |
| `--open` | `-o` | Open browser automatically |
| `--no-live-reload` | | Disable live reload on file changes |
| `--password <pass>` | | Enable password protection for presenter mode |
| `--qr` | | Display QR code for mobile access |

### Examples

```bash
# Start dev server for a presentation
tap dev slides.md

# Start on a specific port
tap dev slides.md --port 8080

# Open browser automatically
tap dev slides.md --open

# Bind to all interfaces (for network access)
tap dev slides.md --host 0.0.0.0

# Enable presenter mode password
tap dev slides.md --password secret123

# Show QR code for mobile devices
tap dev slides.md --qr
```

### URLs

When the dev server starts, it provides:

| URL | Description |
|-----|-------------|
| `http://localhost:3000` | Audience view (main presentation) |
| `http://localhost:3000/presenter` | Presenter view with notes and timer |

### Features

- **Live reload**: Changes to your markdown file are instantly reflected
- **Live code execution**: Run SQL, shell commands, and other drivers
- **Presenter mode**: Access speaker notes and timer at `/presenter`
- **Cross-device sync**: Control from tablet/phone, display on main screen

::: tip
Use `--host 0.0.0.0` to access the presentation from other devices on your network.
:::

---

## tap build

Build a production-ready static version of your presentation.

### Usage

```bash
tap build <file>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `file` | Path to the markdown presentation file (required) |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--out <dir>` | `-o` | Output directory (default: `dist/`) |
| `--base <path>` | `-b` | Base path for deployment (default: `/`) |
| `--minify` | `-m` | Enable additional minification |
| `--no-clean` | | Don't clean output directory before build |
| `--watch` | `-w` | Watch for changes and rebuild |

### Examples

```bash
# Basic build
tap build slides.md

# Build to custom directory
tap build slides.md --out ./public

# Build for subdirectory deployment (e.g., GitHub Pages)
tap build slides.md --base /my-repo/

# Build with minification
tap build slides.md --minify

# Watch mode for continuous building
tap build slides.md --watch
```

### Output Structure

```
dist/
├── index.html         # Main presentation entry point
├── assets/
│   ├── style.css      # Optimized presentation styles
│   └── main.js        # Bundled JavaScript
├── images/            # Copied image assets
└── fonts/             # Font files (if used)
```

::: warning
Static builds do not include live code execution. Code blocks with drivers will show their last executed result or a placeholder.
:::

---

## tap serve

Serve a built presentation locally for preview or local deployment.

### Usage

```bash
tap serve [dir]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `dir` | Directory to serve (default: `dist/`) |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port to serve on (default: `8080`) |
| `--host <ip>` | | Host to bind to (default: `localhost`) |
| `--open` | `-o` | Open browser automatically |
| `--cors` | | Enable CORS headers |

### Examples

```bash
# Serve the default dist directory
tap serve

# Serve a specific directory
tap serve ./public

# Serve on a different port
tap serve dist --port 3000

# Open browser automatically
tap serve dist --open

# Make accessible on network
tap serve dist --host 0.0.0.0
```

::: tip
Use `tap serve` to verify your production build before deploying. This catches issues like broken asset paths or base URL misconfiguration.
:::

---

## tap pdf

Export your presentation to PDF format.

### Usage

```bash
tap pdf <file>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `file` | Path to the markdown presentation file (required) |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--out <file>` | `-o` | Output filename (default: `<input>.pdf`) |
| `--format <type>` | `-f` | Export format: `slides`, `notes`, `both` (default: `slides`) |
| `--paper <size>` | | Paper size: `letter`, `a4`, `16:9`, `4:3` (default: `16:9`) |
| `--margin <px>` | `-m` | Page margins in pixels (default: `0`) |
| `--quality <level>` | `-q` | Image quality: `low`, `medium`, `high` (default: `high`) |
| `--no-animations` | | Export without animation frames |

### Export Formats

| Format | Description |
|--------|-------------|
| `slides` | Exports presentation slides only (default) |
| `notes` | Exports speaker notes as a document |
| `both` | Exports slides with corresponding notes below each |

### Examples

```bash
# Basic PDF export
tap pdf slides.md

# Custom output filename
tap pdf slides.md --out quarterly-review.pdf

# Export slides with notes (handout format)
tap pdf slides.md --format both

# Export notes only (speaker script)
tap pdf slides.md --format notes

# A4 paper size with notes
tap pdf slides.md --format both --paper a4

# Letter size with margins
tap pdf slides.md --paper letter --margin 20

# Lower quality for smaller file size
tap pdf slides.md --quality medium
```

::: tip
PDF export captures your presentation at export time. If you have live code execution, the results shown will be whatever was displayed when you ran the export.
:::

---

## tap add

Add a new slide or asset to an existing presentation.

### Usage

```bash
tap add [file]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `file` | Path to the presentation file (default: autodetect) |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--layout <name>` | `-l` | Layout for new slide (default: `default`) |
| `--position <n>` | `-p` | Insert position (slide number, default: end) |
| `--title <text>` | `-t` | Slide title |
| `--image <path>` | `-i` | Add an image to the assets |
| `--interactive` | | Interactive mode with prompts |

### Examples

```bash
# Add a blank slide at the end
tap add slides.md

# Add slide with specific layout
tap add slides.md --layout two-column

# Add slide at specific position
tap add slides.md --position 3 --title "New Section"

# Add slide with title
tap add slides.md --title "Conclusion" --layout section

# Import an image asset
tap add slides.md --image ./photo.png

# Interactive mode
tap add slides.md --interactive
```

### Available Layouts

Use `--layout` with any of these options:

| Layout | Description |
|--------|-------------|
| `default` | Standard content slide |
| `title` | Title slide with centered content |
| `section` | Section divider |
| `two-column` | Two-column layout |
| `three-column` | Three-column layout |
| `code-focus` | Optimized for code blocks |
| `big-stat` | Large statistic display |
| `quote` | Quotation layout |
| `cover` | Full-bleed image background |
| `sidebar` | Main content with sidebar |
| `split-media` | Split view with media |
| `blank` | Empty slide |

---

## Global Flags

These flags work with all commands:

| Flag | Description |
|------|-------------|
| `--help` | Show help for the command |
| `--version` | Show Tap version |
| `--verbose` | Enable verbose output |
| `--quiet` | Suppress non-error output |
| `--config <file>` | Path to config file |
| `--no-color` | Disable colored output |

### Examples

```bash
# Show help
tap --help
tap dev --help

# Show version
tap --version

# Verbose mode for debugging
tap build slides.md --verbose

# Quiet mode for CI/CD
tap build slides.md --quiet
```

---

## Quick Reference

| Command | Description | Example |
|---------|-------------|---------|
| `tap new [name]` | Create new presentation | `tap new my-talk` |
| `tap dev <file>` | Start dev server | `tap dev slides.md` |
| `tap build <file>` | Build for production | `tap build slides.md` |
| `tap serve [dir]` | Serve built files | `tap serve dist` |
| `tap pdf <file>` | Export to PDF | `tap pdf slides.md` |
| `tap add [file]` | Add slide or asset | `tap add slides.md` |

---

## Exit Codes

Tap uses standard exit codes:

| Code | Description |
|------|-------------|
| `0` | Success |
| `1` | General error |
| `2` | Invalid arguments or flags |
| `3` | File not found |
| `4` | Build error |
| `5` | Export error |

---

## Environment Variables

Configure Tap behavior via environment variables:

| Variable | Description |
|----------|-------------|
| `TAP_PORT` | Default port for `tap dev` (default: 3000) |
| `TAP_HOST` | Default host for `tap dev` (default: localhost) |
| `TAP_THEME` | Default theme for `tap new` (default: minimal) |
| `NO_COLOR` | Disable colored output when set |

Environment variables can be overridden by command-line flags.

---

## Next Steps

- [Frontmatter Options](/reference/frontmatter-options) - Configure your presentation
- [Slide Directives](/reference/slide-directives) - Per-slide settings
- [Layouts Reference](/reference/layouts-reference) - All available layouts
- [Drivers](/reference/drivers) - Live code execution configuration
