# CLI Commands

Complete reference for Tap CLI commands. See `docs/reference/cli-commands.md` for the full docs site reference.

## tap new

Create a new presentation, interactively or non-interactively. `[deck]` is
the path of the new deck, the same as `--output`.

```bash
tap new
tap new [deck] --yes --title <title> --theme <slug> --output <file>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--title <title>` | | Presentation title (default: `"My Presentation"`) |
| `--theme <slug>` | `-t` | Theme (default: the first built-in theme) |
| `--output <file>` | `-o` | Output filename (default: derived from the title) |
| `--yes` | `-y` | Skip the wizard and write from flags and defaults |
| `--force` | | Overwrite `--output` if it already exists |
| `--json` | | Print the written deck as JSON (skips the wizard) |

With a terminal attached, `tap new` opens an interactive wizard and the
flags only pre-fill its steps. With `--yes`, or with no terminal attached
to standard input, the wizard is skipped: the file is written straight
from the flags, and only the written path is printed to standard output.
An agent working unattended should call `tap new --yes ...` rather than
write the markdown file directly.

Examples:
```bash
tap new                            # interactive wizard
tap new --theme terminal           # wizard, terminal theme pre-filled
tap new --theme keynote --output launch.md
tap new --yes --title "My Talk" --theme terminal --output talk.md   # no wizard
```

`--json`:
```json
{"ok": true, "deck": "talk.md"}
```

## tap dev

Start the development server with live reload and live code execution.

```bash
tap dev [deck]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port for the dev server (default: `3000`) |
| `--lan` | | Listen on the local network too, so a phone on the same network can open the presenter view. Without it, only this machine can connect |
| `--presenter-password <pass>` | | Password to protect the presenter view |
| `--headless` | | Run without the TUI, for testing/automation |

The server listens on this machine only, unless `--lan` opens it to the
local network. With `--lan`, any device on the network can open the deck
and run its live code blocks. The terminal then shows a `Network:`
presenter URL and a QR code for it; `--tunnel` works without `--lan` and
shows its own QR code instead. `/qr` needs `--lan`; without it the
endpoint answers 404.

Examples:
```bash
tap dev slides.md
tap dev slides.md --port 8080
tap dev slides.md --lan            # let a phone on the same network connect
```

Saving the deck updates open pages in place and keeps the current slide and step; there is no need to reload the browser.

## tap present

Serve the deck for a talk or a practice run of it. Unlike `tap dev`, it
does not reload when files change, opens the slides at launch, and leaves
out the keys that edit the deck. Every run can be recorded from launch
until you quit.

```bash
tap present [deck]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port for the server (default: `3000`) |
| `--lan` | | Listen on the local network too, so a phone on the same network can open the presenter view. Without it, only this machine can connect |
| `--no-record` | | Do not record this run |

With `--lan`, any device on the network can open the deck and run its
live code blocks.

Examples:
```bash
tap present
tap present slides.md
tap present slides.md --lan
```

## tap build

Build a production-ready static version. Static builds don't execute live code.

```bash
tap build [deck]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--output <dir>` | `-o` | Output directory (default: `dist`) |
| `--json` | | Print the result as JSON |
| `--progress json` | | progress as JSON lines on stderr |

Examples:
```bash
tap build slides.md
tap build slides.md --output ./public
```

`--json`:
```json
{"ok": true, "output": "dist", "files": 12, "bytes": 483920}
```

## tap serve

Serve a built presentation locally.

```bash
tap serve [dir]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port for the server (default: `3000`) |

## tap export pdf

Export a presentation to PDF.

```bash
tap export pdf [deck]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--output <file>` | `-o` | Output PDF file path (default: `<deck>.pdf`) |
| `--content <type>` | | Content to include: `slides`, `notes`, or `both` |
| `--json` | | Print the result as JSON |
| `--progress json` | | progress as JSON lines on stderr |

Examples:
```bash
tap export pdf slides.md
tap export pdf slides.md --output quarterly-review.pdf
tap export pdf slides.md --content both   # slides with speaker notes
```

`--json`:
```json
{"ok": true, "output": "slides.pdf", "pages": 24, "bytes": 1048576, "brokenSlides": [{"slide": 4, "message": "component blew up on purpose"}]}
```

## tap export images

Render one slide, or every slide, to a PNG. Exits with status 1 when the
slide shows an error card, which makes it a self-check.

```bash
tap export images [deck] [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--slide <n>` | | One-based slide number. Required unless `--all` |
| `--all` | | Every slide's final state, into a folder |
| `--step <k>` | | Render the slide after this many steps, from 0 (default: the final step) |
| `--fragment <k>` | | Render the slide with this many fragments shown, from 0 (default: all) |
| `--theme <slug>` | `-t` | Render with this theme instead of the deck's own |
| `--output <path>` | `-o` | Output PNG file, or folder with `--all` |
| `--width <px>` | | Viewport width (default `1920`); height follows the aspect ratio |
| `--json` | | Print the written files as JSON |
| `--progress json` | | progress as JSON lines on stderr |

Without `--step` or `--fragment`, the slide renders its final state in
print mode. On success it prints only the paths it wrote, one per line.

Examples:
```bash
tap export images deck.md --slide 12
tap export images deck.md --slide 12 --step 3
tap export images deck.md --all --output shots/
tap export images deck.md --slide 2 --output check.png || echo "slide 2 is broken"
```

`--json`:
```json
{"ok": true, "files": ["slide-012.png"]}
```

## tap slide add

Add a new slide interactively. It needs a terminal.

```bash
tap slide add [deck]
```

## tap component new

Scaffold a deck-supplied React component. See
`skills/tap/rules/components.md`.

```bash
tap component new <Name> [deck] [--inline] [--ts] [--json]
```

`--json`:
```json
{"ok": true, "files": ["slides/RollingDeploy.jsx"], "snippet": "::component RollingDeploy\n"}
```

## tap theme

```bash
tap theme list [--json]
tap theme show [slug|deck] [--json | --prompt]
```

`show` prints a theme's name, polarity, pitch, tokens, and illustration
style. `--prompt` prints a style brief to paste in front of an image model
request so illustrations match the theme. With no argument, `show` uses
the theme of the deck in the current folder.

`theme list --json`:
```json
{"ok": true, "themes": [{"slug": "terminal", "name": "Terminal", "polarity": "dark", "pitch": "..."}]}
```

## Conventions

- `[deck]` is optional. It is a deck file or a folder. With no deck, tap
  uses the only deck in the current folder, opens a picker on a terminal,
  or exits with the list of decks.
- `--output/-o`, `--theme/-t`, `--port/-p`, `--yes/-y` and `--json` mean
  the same thing on every command that has them.
- `--json` prints `{"ok": true, ...}` or `{"ok": false, "error": {"code":
  "...", "message": "..."}}`.
- Numbers are 1-based. `--step N` is the slide after N steps, `--fragment
  N` is the slide with N fragments shown, `0` is the state before the
  first one, and a flag you leave out means the final state.
- Exit codes: `0` success, `1` a problem you can fix, `2` a problem in tap
  or its environment, `130` interrupted.
- The old names `tap pdf`, `tap screenshot`, `tap add` and `tap add
  component` print the new name and exit 1.
- `--progress json` prints one JSON line per step on stderr (`{"phase":"render","done":2,"total":9}`) and a final `{"phase":"done","ok":true,...}` line with the `--json` result fields. Use it when a program needs progress; use `--json` when it only needs the result.

Errors and warnings go to standard error; a command's real output goes to
standard output.

## Quick Reference

| Command | Description |
|---------|-------------|
| `tap new [deck]` | Create a presentation (interactive wizard) |
| `tap dev [deck]` | Start dev server with live code execution |
| `tap present [deck]` | Give the talk: serve, open, and record the run |
| `tap build [deck]` | Build for production (static, no live code) |
| `tap serve [dir]` | Serve built files |
| `tap export pdf [deck]` | Export to PDF |
| `tap export images [deck]` | Render a slide to a PNG (exit 1 if broken) |
| `tap slide add [deck]` | Add a slide interactively |
| `tap component new <Name> [deck]` | Scaffold a deck component |
| `tap theme list` / `tap theme show` | Inspect a theme's tokens and style |
