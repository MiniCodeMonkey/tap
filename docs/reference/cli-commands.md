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

Create a new presentation. `tap new` always opens an interactive wizard that walks you through a title, a theme, and a filename; the flags pre-fill steps rather than skip the wizard. The command needs a terminal, so it cannot run unattended.

### Usage

```bash
tap new
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--theme <slug>` | `-t` | Pre-fill the wizard's theme step with this theme |
| `--output <file>` | `-o` | Pre-fill the wizard's output filename |

### Examples

```bash
# Interactive wizard
tap new

# Start the wizard with the Terminal theme already chosen
tap new --theme terminal

# Combine flags
tap new --theme keynote --output launch.md
```

### Output

Creates a new markdown file with frontmatter and eight starter slides that show a title slide, a fragment, a `code-focus` slide, a two-column slide with `::left` and `::right` markers, and a `quote` slide:

```markdown
---
title: "My Talk"
theme: terminal
author: "Your Name"
date: "2026-09-19"
aspectRatio: "16:9"
transition: fade
---

# My Talk

Your Name

---

## Agenda

- Introduction
- Main Content
- Conclusion

<!-- pause -->

Take your time to go through each section.
```

---

## tap dev

Start the development server with live reload and live code execution for real-time preview.

### Usage

```bash
tap dev [file]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port to serve on (default: `3000`) |
| `--presenter-password <pass>` | | Password to protect the presenter view |
| `--headless` | | Run without the terminal UI, for testing/automation |

If the default port is already taken, `tap dev` tries the next ports in turn (up to 20 above it) and prints the URL of whichever one it actually bound, so two decks (or two agents) can run side by side without flags. Passing `--port` explicitly instead fails outright when that exact port is busy:

```
port 3000 is already in use (another tap dev/tap serve may be running); pass --port <other>
```

### Examples

```bash
# Start dev server for a presentation
tap dev slides.md

# Start on a specific port
tap dev slides.md --port 8080

# Protect the presenter view
tap dev slides.md --presenter-password secret123
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
- **Cross-device sync**: Control from one device, display on another

---

## tap build

Build a production-ready static version of your presentation.

### Usage

```bash
tap build <file>
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output <dir>` | `-o` | Output directory (default: `dist`) |

### Examples

```bash
# Basic build
tap build slides.md

# Build to a custom directory
tap build slides.md --output ./public
```

::: warning
Static builds do not include live code execution. Code blocks with drivers show their content as written, not a live result.
:::

---

## tap serve

Serve a built presentation locally for preview.

### Usage

```bash
tap serve [dir]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port to serve on (default: `3000`) |

If the default port is already taken, `tap serve` tries the next ports in turn (up to 20 above it) and prints the URL of whichever one it actually bound. Passing `--port` explicitly instead fails outright when that exact port is busy:

```
port 8080 is already in use (another tap dev/tap serve may be running); pass --port <other>
```

The port is bound before the startup message prints, so a busy port is reported immediately rather than surfacing later as a server error.

### Examples

```bash
tap serve
tap serve ./public
tap serve dist --port 8080
```

::: tip
Use `tap serve` to verify your production build before deploying. This catches issues like broken asset paths.
:::

---

## tap pdf

Export your presentation to PDF format.

### Usage

```bash
tap pdf <file>
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output <file>` | `-o` | Output PDF file path (default: `<input>.pdf`) |
| `--content <type>` | | Content to include: `slides`, `notes`, or `both` (default: `slides`) |

### Content Types

| Type | Description |
|------|-------------|
| `slides` | Exports presentation slides only (default) |
| `notes` | Exports speaker notes as a document |
| `both` | Exports slides with corresponding notes |

### Examples

```bash
# Basic PDF export
tap pdf slides.md

# Custom output filename
tap pdf slides.md --output quarterly-review.pdf

# Export slides with notes (handout format)
tap pdf slides.md --content both

# Export notes only (speaker script)
tap pdf slides.md --content notes
```

### Behavior

Each slide is exported in its final state: every fragment revealed, every step at its last value, and no animation. Deck components are built and registered the same way `tap screenshot` does it, so a component appears in the PDF with `printMode = true` and `step = steps`.

Page size follows the deck's own `aspectRatio`.

`tap pdf` exits with status 1, writing no PDF, when a deck component fails to build. It prints the same line `tap build` and `tap screenshot` do:

```
error: slides/RollingDeploy.jsx:12:8: Expected ")" but found "}"
```

A slide that shows an error card at export time, such as a component that throws while rendering, is still written to the PDF. `tap pdf` prints one line per affected slide to standard error and exits 0:

```
warning: slide 4 shows an error card
```

::: tip
PDF export captures your presentation at export time. If you have live code execution, the results shown are whatever was displayed when you ran the export.
:::

---

## tap screenshot

Render one slide, or every slide, to a PNG. Tap starts a temporary server and drives the same headless Chromium `tap pdf` uses. It is built for checking a slide you just wrote, by hand or from a script.

### Usage

```bash
tap screenshot <file> [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--slide <n>` | One-based slide number to capture. Required unless `--all` |
| `--all` | Capture every slide's final state into a folder instead of one slide |
| `--step <k>` | Presenter step to render, without print mode |
| `--fragment <k>` | Fragment index to render, without print mode (default `-1`) |
| `--theme <slug>` | Theme slug to render with, instead of the deck's own theme |
| `--out <path>` | Output PNG file, or output folder with `--all`. Default derived from the deck's file name |
| `--wait <ms>` | Keep the capture live and wait this long after the page is ready, instead of settling it (`0` to `60000`) |
| `--width <px>` | Viewport width in pixels; height follows the deck's aspect ratio (default `1920`) |

### Behavior

With neither `--step` nor `--fragment`, the slide renders its final state through print mode, exactly as `tap pdf` would.

With either flag, the slide renders that exact presenter state, **settled**: the requested step or fragment, with every component and theme animation given its finished appearance rather than caught partway through. The step is not moved to the deck's final value the way true print mode moves it.

`--wait <ms>` skips the settling and keeps the capture live, then waits that many milliseconds before taking the shot. The clock starts once the page is **ready**, not at navigation: network idle, fonts loaded, and any animation already running finished. A short mount animation is therefore already over when the wait begins, so `--wait` suits an animation that starts on a timer, or one that runs longer than those readiness waits.

On success the command prints the path of each file written, one per line, and nothing else.

`--all` writes `slide-001.png`, `slide-002.png`, and so on into the output folder. It does not stop at the first broken slide: it tries every slide, prints the paths it did write to standard output, then prints one `slide N: <reason>` line per broken slide to standard error and exits 1. `--all` cannot be combined with `--slide`, `--step`, or `--fragment`.

It exits with status 1, and a message on standard error, on any of: a missing deck, an out-of-range slide, step, fragment, or `--wait`, an unknown theme, a deck component that fails to build, a browser that cannot start, or a rendered slide that shows a slide or component error card. A component build error fails before any image is written.

### Examples

```bash
# Final state of slide 12
tap screenshot deck.md --slide 12

# Slide 12 at presenter step 3, settled
tap screenshot deck.md --slide 12 --step 3

# Live, 400ms past readiness, for a timer-driven animation
tap screenshot deck.md --slide 12 --step 3 --wait 400

# Custom output file
tap screenshot deck.md --slide 12 --out slide.png

# Every slide's final state into a folder
tap screenshot deck.md --all --out shots/

# Render with a specific theme
tap screenshot deck.md --slide 3 --theme bauhaus

# Use the exit status as a self-check
tap screenshot deck.md --slide 2 --out check.png || echo "slide 2 is broken"
```

---

## tap add

Add a new slide to an existing presentation interactively.

### Usage

```bash
tap add [file]
```

---

## tap add component

Scaffold a deck-supplied React component from a template. See [Custom Components](/guide/custom-components).

### Usage

```bash
tap add component <Name> [flags]
```

`<Name>` must be a PascalCase identifier, for example `RollingDeploy`. The command refuses to overwrite an existing file.

### Flags

| Flag | Description |
|------|-------------|
| `--inline` | Scaffold an inline block component instead of a whole-slide one |
| `--ts` | Write a `.tsx` file, plus `tap-env.d.ts` and `tap-shims.d.ts` next to the deck |
| `--deck <file>` | Deck file the component belongs to. Default: the current directory |

Without `--inline`, the file goes to `slides/<Name>.jsx`. With `--inline`, it goes to `components/<Name>.jsx`. With `--ts`, the extension is `.tsx`, and the two declaration files are written only when they do not already exist. `tap-shims.d.ts` is skipped when `node_modules/@types/react` exists next to the deck or above it.

The command prints the files it wrote, then the markdown snippet to paste into the deck.

### Examples

```bash
tap add component RollingDeploy                  # slides/RollingDeploy.jsx
tap add component LatencyDrop --inline           # components/LatencyDrop.jsx
tap add component RollingDeploy --ts             # slides/RollingDeploy.tsx
tap add component RollingDeploy --deck deck.md   # relative to deck.md's folder
```

---

## tap theme

Read a built-in theme's tokens and illustration style from the command line. Useful for writing a component that matches the theme, and for prompting an image model for illustrations that fit it.

### tap theme list

```bash
tap theme list
tap theme list --json
```

Prints every built-in theme's slug, name, polarity, and pitch as a table, or as a JSON array with `--json`.

### tap theme show

```bash
tap theme show <slug> [flags]
```

| Flag | Description |
|------|-------------|
| `--json` | Print the theme as JSON |
| `--prompt` | Print a style brief for an image model |
| `--deck <file>` | Read the theme from that deck's frontmatter instead of naming a slug |

Without a flag, prints the theme's name, polarity, pitch, colors, fonts, motion, spacing tokens, and illustration style.

With `--prompt`, prints a plain-text style brief to paste in front of an image model request: the palette with hex values and roles, how the palette should be used, line and shape language, texture, mood, things to avoid, a type feel hint, and the canvas ratio.

### Examples

```bash
tap theme list
tap theme show terminal
tap theme show terminal --json
tap theme show blueprint --prompt
tap theme show --deck slides.md --prompt
```

---

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Enable verbose output |

## Output Streams

Every command writes its real output to standard output and its errors and
warnings to standard error. A script can therefore read, for example,
`tap screenshot`'s written paths without filtering diagnostics out of them.

Commands exit with status 1 on failure. `tap build` fails on an unknown
layout, an undeclared slot, or a deck component that does not build;
`tap screenshot` also fails on an out-of-range slide, step, or fragment, an
unknown theme, or a rendered slide that shows an error card.

---

## Quick Reference

| Command | Description | Example |
|---------|-------------|---------|
| `tap new` | Create a new presentation (interactive wizard) | `tap new --theme terminal` |
| `tap dev [file]` | Start dev server | `tap dev slides.md` |
| `tap build <file>` | Build for production | `tap build slides.md` |
| `tap serve [dir]` | Serve built files | `tap serve dist` |
| `tap pdf <file>` | Export to PDF | `tap pdf slides.md` |
| `tap screenshot <file>` | Render a slide to a PNG | `tap screenshot slides.md --slide 4` |
| `tap add [file]` | Add a slide interactively | `tap add slides.md` |
| `tap add component <Name>` | Scaffold a deck component | `tap add component RollingDeploy` |
| `tap theme list` | List every built-in theme | `tap theme list --json` |
| `tap theme show <slug>` | Show a theme's tokens and style | `tap theme show blueprint --prompt` |

---

## Next Steps

- [Frontmatter Options](/reference/frontmatter-options) - Configure your presentation
- [Slide Directives](/reference/slide-directives) - Per-slide settings
- [Layouts Reference](/reference/layouts-reference) - All available layouts
- [Components Reference](/reference/components-reference) - Deck-supplied React components
- [Drivers](/reference/drivers) - Live code execution configuration
