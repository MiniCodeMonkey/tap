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

Create a new presentation. `[deck]` is the path of the new deck, the same as `--output`. With a terminal attached, `tap new` opens an interactive wizard that walks you through a title, a theme, and a filename; the flags pre-fill those steps. With `--yes` (or `-y`), or with no terminal attached to standard input, the wizard is skipped: the file is written straight from the flags, with defaults for anything not given, and only the written path goes to standard output. This is the mode for scripts, CI, and LLM agents.

### Usage

```bash
tap new
tap new [deck] --yes [--title <title>] [--theme <slug>] [--output <file>] [--force]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--title <title>` | | Presentation title (default: `"My Presentation"`) |
| `--theme <slug>` | `-t` | Theme for the presentation (default: the first built-in theme) |
| `--output <file>` | `-o` | Output filename (default: derived from the title, for example `my-talk.md`) |
| `--yes` | `-y` | Skip the wizard and write the file from flags and defaults |
| `--force` | | Overwrite `--output` if it already exists (non-interactive mode only) |
| `--json` | | Print the written deck as JSON (skips the wizard) |

### Examples

```bash
# Interactive wizard
tap new

# Start the wizard with the Terminal theme already chosen
tap new --theme terminal

# Combine flags
tap new --theme keynote --output launch.md

# Non-interactive: no wizard, no terminal needed
tap new --yes --title "My Talk" --theme terminal --output talk.md

# Overwrite an existing file non-interactively
tap new --yes --output talk.md --force
```

Non-interactive mode refuses to overwrite an existing `--output` file unless `--force` is given. With no terminal attached to standard input, `tap new` behaves as if `--yes` was passed, so it never hangs waiting on the wizard.

### `--json`

```json
{"ok": true, "deck": "talk.md"}
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
tap dev [deck]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port to serve on (default: `3000`) |
| `--lan` | | Listen on the local network too, so a phone on the same network can open the presenter view. Without it, only this machine can connect |
| `--presenter-password <pass>` | | Password gating the presenter view and `/qr`, and gating who may drive other windows. Any characters are allowed |
| `--allow-origin <value>` | | An additional origin (`scheme://host:port`) allowed to connect to the websocket hub, **or** a host (`host:port`) allowed in a request's `Host` header. Repeatable |
| `--tunnel` | | Also serve the deck on a public `https` URL through a Cloudflare Quick Tunnel. Needs `cloudflared`; no Cloudflare account |
| `--headless` | | Run without the terminal UI, for testing/automation |
| `--allow-code` | | Run the deck's live code for this run without an approval, and save none. For `--headless` and scripts |
| `--app` | | Run as the engine of the Tap desktop app. An interface for the app, not for people. See [App mode](#app-mode-app) |

The server listens on this machine only, unless `--lan` opens it to the
local network. With `--lan`, any device on the network can open the deck
and run its live code blocks. The terminal then shows a `Network:`
presenter URL and a QR code for it. `--tunnel` works without `--lan` and
shows its own QR code instead. `/qr` needs `--lan`: without it, its
network URLs would not work, and the endpoint answers 404.

#### Sharing a deck with `--tunnel`

`--tunnel` starts a Cloudflare Quick Tunnel next to the dev server and
prints the public address it hands back:

```
Audience:  http://localhost:3000
Presenter: http://localhost:3000/presenter
Tunnel:    https://plain-shoes-arrive-lately.trycloudflare.com
```

A Quick Tunnel needs no Cloudflare account, no login and no configuration.
The name is random, it lasts only as long as the server, and nothing is
registered anywhere. Press `u` in the dev terminal to start or stop a
tunnel without restarting, and the terminal shows a QR code to point a
phone at.

It needs the `cloudflared` binary on `PATH`:

```bash
brew install cloudflared      # macOS
winget install --id Cloudflare.cloudflared   # Windows
```

Two things worth knowing. **Anyone with the link can watch the deck** while
the tunnel is up, so treat the URL as the password it is not. And the
tunnel is the only way a phone gets a *secure context* from a dev server,
which is what browser features such as the screen wake lock require; over
plain `http` to a LAN address those features are simply unavailable.

While a tunnel runs, its hostname is added to the `Host` allow-list below,
and it is removed again when the tunnel stops.

`tap dev` checks two things, to keep a page on another site from driving
your deck and to block DNS rebinding.

**The `Host` header.** Requests to `/ws`, `/api/`, `/presenter`, `/qr`,
`/local/`, and `/components/` must arrive with a `Host` that is
`localhost`, a loopback, private, or link-local IP address (IPv4 or IPv6),
a name ending in `.local`, this machine's own hostname, or a value passed
with `--allow-origin`. Anything else gets 403 `Forbidden: host not allowed;
use --allow-origin to allow it`.

**The websocket origin.** A connection is accepted when it carries no
`Origin` header, or when the origin's host equals the request's `Host`
**and** that host passes the check above, or when the origin itself is in
the allow list. A rejected one is logged, for example
`rejected websocket connection with Host "evil.example.com": not a local,
private, or allowed host`.

::: warning Reaching tap through a custom name or a tunnel
LAN IP addresses, `localhost`, and `.local` names pass the `Host` check
with no `--allow-origin` flag, but still need `--lan` to be reachable at
all. A custom DNS name, or a tunnel such as an ngrok or Tailscale
hostname, needs `--allow-origin`, repeating the flag for each one.

```bash
tap dev slides.md --allow-origin talk.example.com
tap dev slides.md --allow-origin https://tap.example.ngrok.app
```
:::

`tap serve`, which has no websocket and no API, is not affected by either
check.

The live app is served only at `/`, `/index.html`, `/presenter`, and
`/presenter.html`. `/presenter/` redirects 301 to `/presenter`, and any
other unknown path returns 404 rather than the app.

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

- **Live updates**: Saving the deck updates every open page in place. Only the slides you changed re-render, and each page keeps its slide, fragment and step (moved back if the slide lost steps). A changed custom theme file, or `r` in the terminal, reloads the page instead.
- **Live code execution**: Run SQL, shell commands, and other drivers
- **Presenter mode**: Access speaker notes and timer at `/presenter`
- **Cross-device sync**: Control from one device, display on another

---

## tap present

Serve the deck for a talk or a practice run of it. Unlike `tap dev`, `tap present` does not reload when files change (press `r` to reload), opens the slides at launch, and leaves out the keys that edit the deck. If you opt in the first time you run it, every `tap present` run is recorded from launch until you quit, following the projector across HDMI swaps.

### Usage

```bash
tap present [deck]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port to serve on (default: `3000`) |
| `--lan` | | Listen on the local network too, so a phone on the same network can open the presenter view. Without it, only this machine can connect |
| `--no-record` | | Do not record this run |
| `--allow-code` | | Run the deck's live code for this run without an approval, and save none. For scripts and other non-interactive runs |
| `--app` | | Run as the engine of the Tap desktop app. See [App mode](#app-mode-app) |
| `--presenter-password <pass>` | | Protect the presenter view with a password, for a phone remote over the tunnel (press `u`) |

The server listens on this machine only, unless `--lan` opens it to the
local network, the same as `tap dev`. With `--lan`, any device on the
network can open the deck and run its live code blocks.

### Examples

```bash
tap present                        # The deck in this folder
tap present slides.md
tap present slides.md --no-record  # Skip recording for this run
tap present slides.md --lan        # Let a phone on the same network connect
```

---

## tap build

Build a production-ready static version of your presentation.

### Usage

```bash
tap build [deck]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output <dir>` | `-o` | Output directory (default: `dist`) |
| `--json` | | Print the result as JSON |
| `--progress json` | none | Print progress to stderr as JSON lines, for a program driving tap (see Progress output). |

### Behavior

The progress spinner goes to standard error, and draws nothing when standard error is not a terminal, so a script capturing standard output gets only the result lines.

`tap build` exits with status 1 when a slide names an unknown layout, uses a slot its layout does not declare, or has a deck component that fails to build.

### Examples

```bash
# Basic build
tap build slides.md

# Build to a custom directory
tap build slides.md --output ./public
```

### `--json`

```json
{"ok": true, "output": "dist", "files": 12, "bytes": 483920}
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

## tap export pdf

Export your presentation to PDF format.

### Usage

```bash
tap export pdf [deck]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output <file>` | `-o` | Output PDF file path (default: `<deck>.pdf`) |
| `--content <type>` | | Content to include: `slides`, `notes`, or `both` (default: `slides`) |
| `--json` | | Print the result as JSON |
| `--progress json` | none | Print progress to stderr as JSON lines, for a program driving tap (see Progress output). |

### Content Types

| Type | Description |
|------|-------------|
| `slides` | Exports presentation slides only (default) |
| `notes` | Exports speaker notes as a document |
| `both` | Exports slides with corresponding notes |

### Examples

```bash
# Basic PDF export
tap export pdf slides.md

# Custom output filename
tap export pdf slides.md --output quarterly-review.pdf

# Export slides with notes (handout format)
tap export pdf slides.md --content both

# Export notes only (speaker script)
tap export pdf slides.md --content notes
```

### Behavior

`tap export pdf` exits with status **130** on Ctrl-C or SIGTERM, after finishing
its cleanup, printing `interrupted` on standard error. This holds whether
the signal reaches the process directly or the terminal signals the whole
process group. A second Ctrl-C during that cleanup exits at once, so a
browser that will not close cannot hold the terminal. The progress spinner
is written to standard error and draws nothing when standard error is not a
terminal, so standard output holds only the result lines; warnings print
after the spinner has stopped.

Each slide is exported in its final state: every fragment revealed, every step at its last value, and no animation. Deck components are built and registered the same way `tap export images` does it, so a component appears in the PDF with `printMode = true` and `step = steps`.

Page size follows the deck's own `aspectRatio`.

`tap export pdf` exits with status 1, writing no PDF, when a deck component fails to build. It prints the same line `tap build` and `tap export images` do:

```
error: slides/RollingDeploy.jsx:12:8: Expected ")" but found "}"
```

A slide that shows an error card at export time, such as a component that throws while rendering, is still written to the PDF. `tap export pdf` prints one line per affected slide to standard error and exits 0:

```
warning: slide 4 shows an error card: component blew up on purpose
```

The message is the one on the card, so a broken export names what broke
rather than only where.

::: tip
PDF export captures your presentation at export time. If you have live code execution, the results shown are whatever was displayed when you ran the export.
:::

### `--json`

```json
{"ok": true, "output": "slides.pdf", "pages": 24, "bytes": 1048576, "brokenSlides": []}
```

`brokenSlides` lists any slide that showed an error card at export time, with its 1-based slide number and the card's message:

```json
{"ok": true, "output": "slides.pdf", "pages": 24, "bytes": 1048576, "brokenSlides": [{"slide": 4, "message": "component blew up on purpose"}]}
```

---

## tap export images

Render one slide, or every slide, to a PNG. Tap starts a temporary server and drives the same headless Chromium `tap export pdf` uses. It is built for checking a slide you just wrote, by hand or from a script.

### Usage

```bash
tap export images [deck] [flags]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--slide <n>` | | One-based slide number to capture. Required unless `--all` |
| `--all` | | Capture every slide's final state into a folder instead of one slide |
| `--step <k>` | | Render the slide after this many steps, from 0 (default: the final step) |
| `--fragment <k>` | | Render the slide with this many fragments shown, from 0 (default: all) |
| `--theme <slug>` | `-t` | Theme slug to render with, instead of the deck's own theme |
| `--output <path>` | `-o` | Output PNG file, or output folder with `--all`. Default derived from the deck's file name |
| `--wait <ms>` | | Keep the capture live and wait this long after the page is ready, instead of settling it (`0` to `60000`) |
| `--width <px>` | | Viewport width in pixels; height follows the deck's aspect ratio (default `1920`) |
| `--json` | | Print the written files as JSON |
| `--progress json` | none | Print progress to stderr as JSON lines, for a program driving tap (see Progress output). |

### Behavior

With neither `--step` nor `--fragment`, the slide renders its final state through print mode, exactly as `tap export pdf` would.

With either flag, the slide renders that exact presenter state, **settled**: the requested step or fragment, with every component and theme animation given its finished appearance rather than caught partway through. The step is not moved to the deck's final value the way true print mode moves it.

`--wait <ms>` skips the settling and keeps the capture live, then waits that many milliseconds before taking the shot. The clock starts once the page is **ready**, not at navigation: network idle, fonts loaded, and any animation already running finished. A short mount animation is therefore already over when the wait begins, so `--wait` suits an animation that starts on a timer, or one that runs longer than those readiness waits.

On success the command prints the path of each file written, one per line, and nothing else.

`--all` writes `slide-001.png`, `slide-002.png`, and so on into the output folder. It does not stop at the first broken slide: it tries every slide, prints the paths it did write to standard output, then prints one `slide N: <reason>` line per broken slide to standard error and exits 1. `--all` cannot be combined with `--slide`, `--step`, or `--fragment`.

It exits with status 1, and a message on standard error, on any of: a missing deck, an out-of-range slide, step, fragment, or `--wait`, an unknown theme, a deck component that fails to build, or a rendered slide that shows a slide or component error card. A component build error fails before any image is written. It exits with status 2 when a browser cannot start.

On Ctrl-C or SIGTERM it finishes its cleanup, prints `interrupted` on standard error, and exits with status **130**, whether the signal reaches the process directly or the terminal signals the whole process group. A second Ctrl-C during the cleanup exits at once.

A slide that renders an error card fails with the card's own message:

```
Error: slide 2 shows an error card: component blew up on purpose
```

### Examples

```bash
# Final state of slide 12
tap export images deck.md --slide 12

# Slide 12 after its third step, settled
tap export images deck.md --slide 12 --step 3

# Live, 400ms past readiness, for a timer-driven animation
tap export images deck.md --slide 12 --step 3 --wait 400

# Custom output file
tap export images deck.md --slide 12 --output slide.png

# Every slide's final state into a folder
tap export images deck.md --all --output shots/

# Render with a specific theme
tap export images deck.md --slide 3 --theme bauhaus

# Use the exit status as a self-check
tap export images deck.md --slide 2 --output check.png || echo "slide 2 is broken"
```

### `--json`

```json
{"ok": true, "files": ["slide-012.png"]}
```

---

## tap slide add

Add a new slide to a deck. Without flags, an interactive wizard asks for a
layout and the content of each section; it needs a terminal. With
`--layout`, tap appends that layout's template without asking, and works
without a terminal. The wizard and `--layout` both offer all 12 layouts:
`title`, `section`, `default`, `two-column`, `code-focus`, `quote`,
`big-stat`, `three-column`, `sidebar`, `split-media`, `cover`, `blank`.

### Usage

```bash
tap slide add [deck]
tap slide add [deck] --layout <name>
tap slide add --layout <name> --print
```

### Flags

| Flag | Description |
|------|-------------|
| `--layout <name>` | Append that layout's template without the wizard. Works without a terminal |
| `--print` | With `--layout`, print the template and write nothing. The template has no `---` separator in front of it, and no deck is needed. With `--json` and no `--layout`, list every layout's template instead |
| `--json` | With `--layout`: `{"ok": true, "deck": "...", "layout": "...", "markdown": "..."}`. With `--print`, `deck` is left out. With `--print` and no `--layout`: `{"ok": true, "layouts": [...]}`, every layout's template |

### Examples

```bash
tap slide add                               # The wizard, for the deck in this folder
tap slide add talk.md                       # The wizard, for a specific deck
tap slide add talk.md --layout quote        # Append a quote slide, no wizard
tap slide add --layout big-stat --print     # Print the big-stat template, write nothing
tap slide add --layout big-stat --print --json
tap slide add --print --json                # List every layout's template
```

### `--json`

```json
{"ok": true, "deck": "talk.md", "layout": "quote", "markdown": "<!--\nlayout: quote\n-->\n\n> \"Your quote here\"\n"}
```

With `--print`, `deck` is left out:

```json
{"ok": true, "layout": "big-stat", "markdown": "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n"}
```

With `--print --json` and no `--layout`, tap lists every layout, in the
wizard's order, so a caller such as the desktop app's layout gallery does
not hard-code layout names:

```json
{"ok": true, "layouts": [
  {"name": "title", "template": "<!--\nlayout: title\n-->\n\n# My Title\n\nOptional subtitle\n"},
  {"name": "section", "template": "..."},
  ...
]}
```

---

## tap slide list

List every slide of a deck: its number, the lines it covers in the file, its layout, title, step and fragment counts, whether it is skipped, its errors, and its code blocks with their drivers.

### Usage

```bash
tap slide list [deck]
```

### Flags

| Flag | Description |
|------|-------------|
| `--json` | Print the slide list as JSON |

### Output

A table with one row per slide: `#`, `LINES`, `LAYOUT`, `TITLE`, `STEPS`, `FRAGMENTS`, and `NOTES` (whether the slide is skipped, the driver of each live code block, and how many errors it has). The deck's own errors, such as frontmatter that fails to parse, print before the table; each slide's own errors print after it.

Line numbers are 1-based. A slide's range covers its text, including its directive comment, without the blank lines around it. The `---` separator lines and the frontmatter belong to no slide.

Slide numbers count skipped slides, so they match the numbers every other tap command uses.

### Examples

```bash
tap slide list                 # The deck in this folder
tap slide list talk.md
tap slide list talk.md --json  # For editors and scripts
```

### `--json`

```json
{"ok": true, "slides": [...], "errors": []}
```

Each slide has `number`, `startLine`, `endLine`, `layout`, `title`, `fragments`, `steps`, `skip`, `errors`, and `codeBlocks` (each with `block`, `language`, `driver`, `live`, `line`). There is also a top-level `errors` list for problems with the deck as a whole.

For example, slide 4 of the conference talk example, which has a live SQL block:

```json
{
  "number": 4,
  "startLine": 41,
  "endLine": 51,
  "layout": "code-focus",
  "title": "",
  "fragments": 0,
  "steps": 0,
  "skip": false,
  "errors": [],
  "codeBlocks": [
    {
      "block": 1,
      "language": "sql",
      "driver": "sqlite",
      "live": true,
      "line": 45
    }
  ]
}
```

---

## tap component new

Scaffold a deck-supplied React component from a template. See [Custom Components](/guide/custom-components).

### Usage

```bash
tap component new <Name> [deck] [flags]
```

`<Name>` must be a PascalCase identifier, for example `RollingDeploy`. `[deck]` is a deck file or a deck folder; the default is the current folder. The command refuses to overwrite an existing file.

### Flags

| Flag | Description |
|------|-------------|
| `--inline` | Scaffold an inline block component instead of a whole-slide one |
| `--ts` | Write a `.tsx` file, plus `tap-env.d.ts` and `tap-shims.d.ts` next to the deck |
| `--json` | Print the written files and the snippet as JSON |

Without `--inline`, the file goes to `slides/<Name>.jsx`. With `--inline`, it goes to `components/<Name>.jsx`. With `--ts`, the extension is `.tsx`, and the two declaration files are written only when they do not already exist. `tap-shims.d.ts` is skipped when `node_modules/@types/react` exists next to the deck or above it.

The command prints the files it wrote, then the markdown snippet to paste into the deck.

### Examples

```bash
tap component new RollingDeploy                # slides/RollingDeploy.jsx
tap component new LatencyDrop --inline         # components/LatencyDrop.jsx
tap component new RollingDeploy --ts           # slides/RollingDeploy.tsx
tap component new RollingDeploy talks/deck.md  # next to talks/deck.md
```

### `--json`

```json
{"ok": true, "files": ["slides/RollingDeploy.jsx"], "snippet": "::component RollingDeploy\n"}
```

---

## tap image add

Copy an image into the `images/` folder next to a deck and print the
markdown that shows it.

### Usage

```bash
tap image add <file> [deck]
tap image add <file> [deck] --slide <n>
```

### Flags

| Flag | Description |
|------|-------------|
| `--slide <n>` | Also add the markdown at the end of slide N, from 1 |
| `--json` | Print the result as JSON |

### Behavior

The copy keeps the file's name, except that whitespace, parentheses, angle
brackets, quotes, backtick, `#`, and `?` are each replaced with `-`, so the
link tap writes needs no escaping. `my diagram (v2).png` is copied as
`my-diagram-v2.png`. When the resulting name is already taken, tap adds
`-2`, `-3`, and so on before the extension, for example `diagram-2.png`.
Accepted formats: png, jpg, jpeg, gif, webp, svg, and avif.

### Examples

```bash
tap image add ~/Desktop/diagram.png
tap image add diagram.png talk.md --slide 3
tap image add diagram.png --json
```

### `--json`

```json
{"ok": true, "deck": "talk.md", "image": "images/diagram.png", "markdown": "![diagram](images/diagram.png)"}
```

`slide` is present only with `--slide`:

```json
{"ok": true, "deck": "talk.md", "image": "images/diagram.png", "markdown": "![diagram](images/diagram.png)", "slide": 3}
```

### Errors

| Code | Cause |
|------|-------|
| `file_not_found` | `<file>` does not exist |
| `not_an_image` | `<file>` is not one of the accepted formats |
| `out_of_range` | `--slide` names a slide the deck does not have |

---

## tap image generate

Generate an image from a prompt with Google Gemini, as the `i` key in
`tap dev` does, and add it at the end of a slide.

### Usage

```bash
tap image generate [deck] --slide <n> --prompt "..."
```

### Flags

| Flag | Description |
|------|-------------|
| `--slide <n>` | Slide to add the image to, from 1 (required) |
| `--prompt <text>` | What the image shows (required) |
| `--json` | Print the result as JSON |

### Behavior

The image is saved as `images/generated-<hash>.<ext>` next to the deck,
and the slide gets an `<!-- ai-prompt: ... -->` comment with the prompt,
so `tap image regenerate` can make it again. `GEMINI_API_KEY` must be set,
in the environment or in a `.env` file next to the deck. On success the
command prints the image's path.

### Examples

```bash
tap image generate --slide 3 --prompt "a lighthouse at dusk, flat vector"
tap image generate talk.md --slide 3 --prompt "a lighthouse at dusk, flat vector" --json
```

### `--json`

```json
{"ok": true, "deck": "talk.md", "slide": 3, "image": "images/generated-1a2b3c4d.png", "prompt": "a lighthouse at dusk, flat vector", "markdown": "<!-- ai-prompt: a lighthouse at dusk, flat vector -->\n![](images/generated-1a2b3c4d.png)"}
```

### Errors

| Code | Cause | Exit |
|------|-------|------|
| `usage` | `--slide` or `--prompt` is missing | 1 |
| `out_of_range` | `--slide` names a slide the deck does not have | 1 |
| `no_api_key` | `GEMINI_API_KEY` is not set | 1 |
| `image_generation` | Generation failed | 2 for a network or server failure, 1 otherwise |

---

## tap image regenerate

Generate an AI image on a slide again, as the `i` key in `tap dev` does,
and replace it where it is. The old image file is deleted.

### Usage

```bash
tap image regenerate [deck] --slide <n> --image <path> [--prompt "..."]
```

### Flags

| Flag | Description |
|------|-------------|
| `--slide <n>` | Slide the image is on, from 1 (required) |
| `--image <path>` | Path of the AI image to replace, as the slide links to it (required) |
| `--prompt <text>` | A new prompt (default: the image's own prompt) |
| `--json` | Print the result as JSON |

### Behavior

`--image` names the image by the path the slide links to, for example
`images/generated-1a2b3c4d.png`. Without `--prompt`, tap reuses the
prompt in the image's `ai-prompt` comment. On success the command prints
the new image's path.

### Examples

```bash
tap image regenerate --slide 3 --image images/generated-1a2b3c4d.png
tap image regenerate talk.md --slide 3 --image images/generated-1a2b3c4d.png --prompt "a lighthouse at dawn, flat vector"
```

### `--json`

Same fields as `tap image generate`, plus `replaced`:

```json
{"ok": true, "deck": "talk.md", "slide": 3, "image": "images/generated-5e6f7a8b.png", "prompt": "a lighthouse at dawn, flat vector", "markdown": "<!-- ai-prompt: a lighthouse at dawn, flat vector -->\n![](images/generated-5e6f7a8b.png)", "replaced": "images/generated-1a2b3c4d.png"}
```

### Errors

Same as `tap image generate`, plus:

| Code | Cause |
|------|-------|
| `image_not_found` | `--image` does not name an AI image on that slide |

---

## tap deck schema

List every frontmatter key tap understands, with its type, its default, its allowed values, and what it does. Editors and tools can build a form or completions from `--json`.

### Usage

```bash
tap deck schema
```

### Flags

| Flag | Description |
|------|-------------|
| `--json` | Print the schema as JSON |

### Output

A table with one row per key: `KEY`, `TYPE`, `DEFAULT`, `VALUES`, and `DESCRIPTION`. A nested key, such as one under `themeColors` or `recording`, is shown with its parent joined by a dot, for example `recording.audio`. A key under a map, such as `drivers`, uses `<name>` for the name the deck picks, for example `drivers.<name>.command`.

A key's type is one of six: `string`, `boolean`, `integer`, `list` (of strings), `object` (a fixed set of nested keys), or `map` (entries under names the deck picks, each with the nested keys).

### Examples

```bash
tap deck schema
tap deck schema --json | head -40
```

### `--json`

```json
{"ok": true, "keys": [...]}
```

Each key has `name`, `type`, `default`, `values`, `description`, and `keys` (its nested keys, for an `object` or a `map`).

---

## tap theme

Read a built-in theme's tokens and illustration style from the command line. Useful for writing a component that matches the theme, and for prompting an image model for illustrations that fit it.

### tap theme list

```bash
tap theme list
tap theme list --json
```

| Flag | Description |
|------|-------------|
| `--json` | Print the list as JSON |

Prints every built-in theme's slug, name, polarity, and pitch as a table, or with `--json`.

#### `--json`

```json
{"ok": true, "themes": [{"slug": "terminal", "name": "Terminal", "polarity": "dark", "pitch": "..."}]}
```

### tap theme show

```bash
tap theme show [slug|deck] [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | | Print the theme as JSON |
| `--prompt` | | Print a style brief for an image model |
| `--image` | | Render a title slide in the theme to a 1280x720 PNG and print its path |
| `--output <file>` | `-o` | With `--image`, copy the PNG to this file and print that path |

The argument is a theme slug, or a deck file or folder whose theme to show. With no argument, tap uses the deck in the current folder.

Without a flag, prints the theme's name, polarity, pitch, colors, fonts, motion, spacing tokens, and illustration style.

With `--prompt`, prints a plain-text style brief to paste in front of an image model request: the palette with hex values and roles, how the palette should be used, line and shape language, texture, mood, things to avoid, a type feel hint, and the canvas ratio.

With `--image`, tap renders a title slide in the theme to a 1280x720 PNG and prints its path. The image is cached per theme and tap version, in the user cache folder under `tap/themes/<version>/<slug>.png` (`~/Library/Caches/tap/themes/...` on macOS), so a repeat call returns at once. `-o`/`--output` copies the PNG to a file of your choosing and prints that path instead.

### Examples

```bash
tap theme list
tap theme show terminal
tap theme show terminal --json
tap theme show blueprint --prompt
tap theme show slides.md --prompt
tap theme show terminal --image
tap theme show terminal --image -o terminal.png
tap theme show terminal --image --json
```

#### `--json`

```json
{"ok": true, "slug": "terminal", "name": "Terminal", "polarity": "dark", "pitch": "...", "tokens": {"...": "..."}, "illustration": {"...": "..."}, "canvas": {"ratio": "16:9", "width": 1920, "height": 1080}}
```

With `--image --json`:

```json
{"ok": true, "slug": "terminal", "image": "/Users/you/Library/Caches/tap/themes/2.0.0/terminal.png", "cached": true}
```

---

## tap theme set

Write the `theme:` key into a deck's frontmatter, the same change the `t` key makes in `tap dev`.

### Usage

```bash
tap theme set <slug> [deck]
```

### Flags

| Flag | Description |
|------|-------------|
| `--json` | Print the result as JSON |

### Behavior

`[deck]` is a deck file or folder; with no deck, tap uses the deck in the current folder. A deck with no frontmatter gets one. An unknown slug exits 1 with code `unknown_theme` and prints the list of themes.

### Examples

```bash
tap theme set terminal
tap theme set blueprint talk.md
tap theme set blueprint talk.md --json
```

### `--json`

```json
{"ok": true, "deck": "talk.md", "theme": "blueprint"}
```

### tap approval list

Lists the decks allowed to run live code, with the drivers each may use and when it was approved.

```bash
tap approval list
tap approval list --json
```

`--json` prints `{"ok": true, "approvals": [{"deck": "/Users/me/talks/talk.md", "drivers": ["shell", "sqlite"], "approvedAt": "2026-09-22T19:32:00Z"}]}`.

### tap approval revoke

Removes a deck's approval. tap asks again the next time it opens the deck. `<deck>` is the file or its folder. A moved or deleted deck can be revoked by its old path. An unapproved deck is exit 1 with the code `not_approved`.

```bash
tap approval revoke talk.md
tap approval revoke talk.md --json   # {"ok": true, "deck": "/Users/me/talks/talk.md"}
```

---

## Conventions

These hold across every command.

- `[deck]` is optional. It is a deck file or a folder. With no deck, tap
  uses the only deck in the current folder, opens a picker on a terminal,
  or exits with the list of decks.
- `--output/-o`, `--theme/-t`, `--port/-p`, `--yes/-y` and `--json` mean
  the same thing on every command that has them.
- `--json` prints `{"ok": true, ...}` or `{"ok": false, "error": {"code":
  "...", "message": "..."}}`. See each command's own `--json` section for
  its result fields.
- Numbers are 1-based. `--step N` is the slide after N steps, `--fragment
  N` is the slide with N fragments shown, `0` is the state before the
  first one, and a flag you leave out means the final state.
- Exit codes: `0` success, `1` a problem you can fix, `2` a problem in tap
  or its environment, `130` interrupted.
- The old names `tap pdf`, `tap screenshot`, `tap add` and `tap add
  component` print the new name and exit 1.

### Progress output

`tap export pdf`, `tap export images` and `tap build` accept `--progress json`. Each step prints one JSON object on its own line to stderr:

    {"phase":"render","done":7,"total":14}

- `export pdf` and `export images` print one `render` line per slide (per notes page for `--content notes`).
- `build` prints `load`, `parse`, `bundle` and `write`, with `total` 4.
- A first export downloads the export browser and prints `{"phase":"download","bytes":52428800,"totalBytes":170175488}` lines while it does. Each downloaded archive starts again from 0.
- The last line is `{"phase":"done","ok":true, ...}` with the same fields as the command's `--json` result, or `{"phase":"done","ok":false,"error":{"code":"...","message":"..."}}`.

stdout keeps the command's normal output (or its `--json` result). Warnings can still appear on stderr as plain text; read only the lines that start with `{`.

### Ready signal

Every tap page reports when the slide on screen has finished rendering: fonts and images loaded, maps drawn, components loaded, error cards shown, and transitions and theme animations done. It sets `window.__tapReady` to `{"revision": "...", "slide": 3, "step": 1}` (`slide` counts from 1), dispatches a `tap:ready` event on `window` with the same object, and, inside a macOS web view that registered a `tapReady` message handler, posts it to that handler. `window.__tapReady` is `null` while a slide is still rendering, and resets when the slide, step, fragment, theme or deck changes. `tap export pdf` and `tap export images` wait for this signal before each capture.

## Output Streams

Every command writes its real output to standard output and its errors and
warnings to standard error. A script can therefore read, for example,
`tap export images`'s written paths without filtering diagnostics out of them.

Commands use the exit codes described under [Conventions](#conventions): `0`
on success, `1` for a problem you can fix, `2` for a problem in tap or its
environment, and `130` when interrupted. `tap build` exits 1 on an unknown
layout, an undeclared slot, or a deck component that does not build;
`tap export images` also exits 1 on an out-of-range slide, step, or
fragment, an unknown theme, or a rendered slide that shows an error card,
and exits 2 when a browser cannot start or a temporary server cannot bind.

---

## Quick Reference

| Command | Description | Example |
|---------|-------------|---------|
| `tap new [deck]` | Create a new presentation (wizard, or non-interactive with `--yes`) | `tap new --yes --theme terminal --output talk.md` |
| `tap dev [deck]` | Start dev server | `tap dev slides.md` |
| `tap present [deck]` | Give the talk: serve, open, and record the run | `tap present slides.md` |
| `tap build [deck]` | Build for production | `tap build slides.md` |
| `tap serve [dir]` | Serve built files | `tap serve dist` |
| `tap export pdf [deck]` | Export to PDF | `tap export pdf slides.md` |
| `tap export images [deck]` | Render a slide to a PNG | `tap export images slides.md --slide 4` |
| `tap slide add [deck]` | Add a slide, by wizard or `--layout` | `tap slide add --layout quote talk.md` |
| `tap slide list [deck]` | List each slide, its lines, layout and errors | `tap slide list slides.md --json` |
| `tap component new <Name> [deck]` | Scaffold a deck component | `tap component new RollingDeploy` |
| `tap image add <file> [deck]` | Copy an image into `images/` | `tap image add diagram.png --slide 3` |
| `tap image generate [deck]` | Generate an AI image onto a slide | `tap image generate --slide 3 --prompt "..."` |
| `tap image regenerate [deck]` | Generate an AI image again, in place | `tap image regenerate --slide 3 --image images/generated-1a2b3c4d.png` |
| `tap deck schema` | List every frontmatter key, type and default | `tap deck schema --json` |
| `tap theme list` | List every built-in theme | `tap theme list --json` |
| `tap theme show [slug\|deck]` | Show a theme's tokens and style | `tap theme show blueprint --prompt` |
| `tap theme set <slug> [deck]` | Set a deck's theme | `tap theme set blueprint talk.md` |

---

## App mode (`--app`)

`tap dev --app <deck>` and `tap present --app [--no-record] <deck>` are the interface between tap and the Tap desktop app. They are documented so the app and tap agree, and they can change with the app. Use the plain commands yourself.

In `--app` mode, tap:

- listens on `127.0.0.1` only, on a free port (`--port` picks one), and cannot be combined with `--lan` or `--headless`;
- opens no browser and shows no terminal interface;
- exits when its standard input closes, so a closed or crashed app never leaves tap running;
- prints only JSON lines on standard output, and human-readable logs on standard error.

**The app must give tap two separate pipes: one for standard output, one for standard error.** Do not merge them into one, the way `2>&1` does. tap writes the protocol and the log from two different writers, and if both land on the same pipe, the two writers interleave inside it. Under load that tears the JSON event stream: a line arrives broken and the app cannot parse it, at well over a hundred broken event lines per run in measurement. This is not something the app can tune around, it breaks under every configuration, and it is an integration error rather than a property of tap's writers. Wire up two pipes before you launch tap.

Without `--presenter-password`, `--app` mode generates one. The reason is the websocket: it is deliberately open to anyone with the link, the same way `tap dev --tunnel` already works, so a phone in the audience can follow along without a token. But that same websocket relays slide and theme messages, so without a password anyone holding the link, or any local process, could steer the deck instead of only watching it. Viewing stays open; steering needs the secret. A generated password is only the default: passing `--presenter-password` yourself still wins.

An app that opens the presenter view must pass the ready line's `presenter` secret as `?key=`, or its own presenter window has no way to drive the deck it just launched.

### Standard output

The first line is the ready line:

```json
{"type": "ready", "port": 49152, "token": "…", "launch": "…", "presenter": "…"}
```

`presenter` is the presenter password in effect for this run, whether it was generated or passed with `--presenter-password`.

Every later line is an event:

| `type` | When | Fields |
|---|---|---|
| `file-changed` | a file in the deck folder changed that tap did not write (`tap dev` only) | `path`, and for a file other than the deck, `slides` and `errors` as `tap slide list --json` prints them |
| `question` | tap needs an answer | `id`, `kind` (`approval`, `record-consent`, `keep-recording`), `payload` |
| `recording` | the recording state changes (`tap present` only) | `state` (`recording`, `paused`, `stopped`), `segment`, `elapsed` (seconds), `disk` (`ok`, `low`, `full`) |
| `tunnel` | the tunnel state changes | `state` (`starting`, `running`, `stopped`), `url`, `qr` (a PNG of the presenter view's URL, base64) |
| `slide` | the audience position changes (`tap present` only) | `slide` (from 1), `step` |
| `error` | a fatal or reportable error | `code`, `message` |

When tap cannot start, an `error` event is the only line, and tap exits 1 or 2.

Standard output has a single writer, so lines never interleave. If the app stops reading it, for instance because its own event loop is stuck, tap does not block waiting for room: an event that cannot be queued at once is dropped instead. tap says so on standard error once when a run of drops starts, and once more, with the count, when the queue has room again and the run ends. A healthy app that keeps reading never sees a drop.

### Standard input

One JSON command per line:

| `type` | Effect |
|---|---|
| `answer` | Answers a question: `{"type": "answer", "id": "q1", "value": true}`. Every answer is `true` or `false`. |
| `saved` | The app saved its buffer to the deck file. tap drops the buffer and reads the file. `tap dev` only. |
| `reload` | Renders the deck again and reloads every page, as `r` does. |
| `tunnel` | `{"type": "tunnel", "start": true}` or `false`, as `u` does. |
| `recording` | `{"type": "recording", "action": "new-segment"}` or `"stop"`, as `c` does. `tap present` only. |
| `quit` | Shuts down. When the run has a recording, tap first asks `keep-recording`. |

Questions and commands travel only over standard input and output, which only the app can reach. No HTTP route and no WebSocket message answers a question or changes the recording.

Quit waits for nothing without a bound. The whole shutdown shares one deadline, a little over the recorder's kill grace, rather than a fresh allowance per step: the startup, the recording reporter, the command in flight, the tunnel, the file watcher, finishing the recording and the HTTP server all draw on the same one. Whatever is still stuck when it runs out is named in an `error` event with its own code instead of hanging the process, and tap exits anyway.

Three things sit outside that deadline, each bounded in its own right. The `keep-recording` question gets a few seconds of its own, because that is quit waiting for the app rather than the app waiting for quit, and no answer keeps the recording. Flushing the events already queued for standard output gets two seconds at the very end, and flushing the log lines already queued for standard error gets half a second after that. So a quit with every single part of tap stuck at once still returns in well under twenty seconds, and an ordinary quit takes microseconds.

A quit exits 0. That includes a quit that catches a request in flight, which one page still loading is enough to cause: the graceful HTTP shutdown then runs out of its share of the deadline, tap says so as a warning on standard error, and the connection goes with the process. An exit code other than 0 means tap really did fail.

Neither pipe can hold quit open. An event that cannot be queued at once is dropped (see [Standard output](#standard-output)), and so is a log line (see [Standard error](#standard-error)). A log line is advisory, and losing one is always a smaller cost than a process that will not exit.

### Standard error

Standard error is the log, for a panel in the app rather than for parsing: warnings from the deck's layouts and components, driver and approval notices, the reason quit gave up on something. The protocol is on standard output alone, and nothing on standard error is ever part of it.

At startup tap names the server's whole routing table:

```
Routes: GET /api/presentation, GET /qr, GET /ws, POST /api/execute, PUT /api/app/source, …
```

That table is what decides whether a request needs the app token, so it is the one input to a 401 or a 404 that an integrator cannot otherwise see. It carries no port, no token, no launch code and no password.

Every line tap writes in `--app` mode goes through one bounded writer. That is not a rule tap follows, it is the only thing left open to it: for the life of an `--app` run tap takes its own standard output and standard error away and points both at a pipe it drains into that writer itself. The protocol travels on a duplicate of standard output taken before the swap, which no ordinary print can name and which no child process inherits, so one JSON object per line still goes there and nothing else does. A print from anywhere in tap or in a library it uses, however it is spelled and on whatever goroutine, lands in the log. A child process tap starts, including a deck's own live code, writes into the same pipe and is bounded the same way.

The writer drops, and that is the part an integrator sees. These warnings always went to standard error, which in `--app` mode has always been the app's log pipe, so an app reading its child's log already received them. What they used to do was hold tap up once that pipe filled. Now a line that cannot be queued at once is thrown away instead, so the log is lossy exactly when it is busiest: a deck that warns on every slide produces several times a pipe buffer of warnings per parse, and most of them do not arrive. For a log panel that is the right trade, but it is loss, not new volume.

What does arrive is a count. The next line through, once the log drains again, carries `Standard error was not being read: <n> log lines were dropped, <total> in total this run.` The first number is the run that just ended; the second is everything this run of tap has lost, so a panel can say how much of the whole session is missing rather than only how much one gap cost. A burst that ends while the log is still unread has no next line to carry its count, so tap flushes a last notice when it closes the log, which is the final moment a count can be spent. There is no third pipe to report any of this on, and an event on standard output would put a diagnostic about the log into the stream the app parses.

This count is a floor, not a total, and treat it that way: the absence of a notice is not proof that nothing was lost. It only counts lines the bounded writer itself drops after they reach its queue. A line can also be lost earlier, at the pipe, before it ever gets that far, and that loss is never in the count. That happens under a heavy flood while the app is reading the log as fast as it can: a fast reader keeps the writer's queue empty, so the queue never fills and no notice is ever queued, but the pipe itself is now the slow link and lines are cut there instead, silently. Measured at a 115 MB flood of 128,000 lines with the app reading flat out: 2.8% to 4.4% of lines lost, and zero notices printed. So the quiet case is not the safe case, it is the opposite: a log panel that never sees a notice can still be missing thousands of lines, precisely because the app is keeping up well enough that the pipe, not the queue, is where the loss happens. None of this shows up at ordinary volumes. A single writer producing 4,000 lines and 324 KB delivers every line intact, and so does a run eight times busier; loss only appears under floods far beyond what tap produces in normal use.

One thing does not go through the writer: a panic. The Go runtime writes a panic message and stack trace to standard error itself, and only after it has stopped every goroutine in the process, so the goroutine draining the pipe cannot run and the trace dies in the pipe with the process. An `--app` engine that panics exits with status 2 and its log panel will usually show nothing about it; run tap from a terminal to see the trace. What is guaranteed is the exit. The pipe is non-blocking, so the runtime's write fails rather than waiting for room that nothing left alive can make, and a tap that has panicked never stays running.

### HTTP

- Every request outside the audience's own routes needs `Authorization: Bearer <token>`. The WebSocket upgrade is one of those audience routes and takes no credential, by design, so a phone remote and an audience link work (see the next bullet). A page gets the token as a cookie: the app loads its first URL with `?launch=<launch>`, and tap sets the cookie and redirects to the same URL without the code. The code works once, and only within two minutes of tap generating it, a hair before the server binds and so a hair before the ready line: it exists to cover the moment between tap starting and the app opening its first window, and it is the one tap secret that is written down in a URL. A code that is spent, expired or simply wrong is refused the same way, with `403` and one message.
- Routes are default-deny: only the audience's own `GET` routes (the deck, the presenter view, its assets, `/ws`, and the routes the audience needs to follow along) are exempt from the token. Every one of them stays exempt whether the request arrives on `127.0.0.1` or through a running tunnel, because that is what lets a phone remote and an audience link work. Every other route, including `PUT /api/app/source` and `POST /api/execute`, always needs the token, and a newly added route needs it by default too: exemption takes a deliberate change to the allow-list, not the other way around. The presenter password guards the presenter view and steering on top of this, as with `tap dev --tunnel`.
- `PUT /api/app/source` (`tap dev --app` only) takes the unsaved buffer as `{"source": "<markdown>"}` and answers with the slide list, the same object `tap slide list --json` prints. It requires the app token and goes through the same body-size and same-origin checks as every other mutating route. tap renders the buffer until the next `saved` command.
- A request that changes something must come from the same origin and send `Content-Type: application/json`.
- A request body may be at most 8 MB for `PUT /api/app/source` and 64 KB for every other route. A larger body gets 413.

## Next Steps

- [Frontmatter Options](/reference/frontmatter-options) - Configure your presentation
- [Slide Directives](/reference/slide-directives) - Per-slide settings
- [Layouts Reference](/reference/layouts-reference) - All available layouts
- [Components Reference](/reference/components-reference) - Deck-supplied React components
- [Drivers](/reference/drivers) - Live code execution configuration
