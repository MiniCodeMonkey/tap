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

The server listens on this machine only, unless `--lan` opens it to the
local network. With `--lan`, the terminal shows a `Network:` presenter URL
and a QR code for it. `--tunnel` works without `--lan` and shows its own
QR code instead. `/qr` needs `--lan`: without it, its network URLs would
not work, and the endpoint answers 404.

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

- **Live reload**: Changes to your markdown file are instantly reflected
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

The server listens on this machine only, unless `--lan` opens it to the
local network, the same as `tap dev`.

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

### Behavior

With neither `--step` nor `--fragment`, the slide renders its final state through print mode, exactly as `tap export pdf` would.

With either flag, the slide renders that exact presenter state, **settled**: the requested step or fragment, with every component and theme animation given its finished appearance rather than caught partway through. The step is not moved to the deck's final value the way true print mode moves it.

`--wait <ms>` skips the settling and keeps the capture live, then waits that many milliseconds before taking the shot. The clock starts once the page is **ready**, not at navigation: network idle, fonts loaded, and any animation already running finished. A short mount animation is therefore already over when the wait begins, so `--wait` suits an animation that starts on a timer, or one that runs longer than those readiness waits.

On success the command prints the path of each file written, one per line, and nothing else.

`--all` writes `slide-001.png`, `slide-002.png`, and so on into the output folder. It does not stop at the first broken slide: it tries every slide, prints the paths it did write to standard output, then prints one `slide N: <reason>` line per broken slide to standard error and exits 1. `--all` cannot be combined with `--slide`, `--step`, or `--fragment`.

It exits with status 1, and a message on standard error, on any of: a missing deck, an out-of-range slide, step, fragment, or `--wait`, an unknown theme, a deck component that fails to build, a browser that cannot start, or a rendered slide that shows a slide or component error card. A component build error fails before any image is written.

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

Add a new slide to an existing presentation interactively. It needs a terminal.

### Usage

```bash
tap slide add [deck]
```

### Examples

```bash
tap slide add              # The deck in this folder
tap slide add talk.md      # A specific deck
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

| Flag | Description |
|------|-------------|
| `--json` | Print the theme as JSON |
| `--prompt` | Print a style brief for an image model |

The argument is a theme slug, or a deck file or folder whose theme to show. With no argument, tap uses the deck in the current folder.

Without a flag, prints the theme's name, polarity, pitch, colors, fonts, motion, spacing tokens, and illustration style.

With `--prompt`, prints a plain-text style brief to paste in front of an image model request: the palette with hex values and roles, how the palette should be used, line and shape language, texture, mood, things to avoid, a type feel hint, and the canvas ratio.

### Examples

```bash
tap theme list
tap theme show terminal
tap theme show terminal --json
tap theme show blueprint --prompt
tap theme show slides.md --prompt
```

#### `--json`

```json
{"ok": true, "slug": "terminal", "name": "Terminal", "polarity": "dark", "pitch": "...", "tokens": {"...": "..."}, "illustration": {"...": "..."}, "canvas": {"ratio": "16:9", "width": 1920, "height": 1080}}
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

## Output Streams

Every command writes its real output to standard output and its errors and
warnings to standard error. A script can therefore read, for example,
`tap export images`'s written paths without filtering diagnostics out of them.

Commands exit with status 1 on failure. `tap build` fails on an unknown
layout, an undeclared slot, or a deck component that does not build;
`tap export images` also fails on an out-of-range slide, step, or fragment, an
unknown theme, or a rendered slide that shows an error card.

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
| `tap slide add [deck]` | Add a slide interactively | `tap slide add slides.md` |
| `tap component new <Name> [deck]` | Scaffold a deck component | `tap component new RollingDeploy` |
| `tap theme list` | List every built-in theme | `tap theme list --json` |
| `tap theme show [slug\|deck]` | Show a theme's tokens and style | `tap theme show blueprint --prompt` |

---

## Next Steps

- [Frontmatter Options](/reference/frontmatter-options) - Configure your presentation
- [Slide Directives](/reference/slide-directives) - Per-slide settings
- [Layouts Reference](/reference/layouts-reference) - All available layouts
- [Components Reference](/reference/components-reference) - Deck-supplied React components
- [Drivers](/reference/drivers) - Live code execution configuration
