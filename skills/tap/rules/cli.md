# CLI Commands

Complete reference for Tap CLI commands. See `docs/reference/cli-commands.md` for the full docs site reference.

## tap new

Create a new presentation with an interactive wizard.

```bash
tap new
```

| Flag | Short | Description |
|------|-------|-------------|
| `--theme <slug>` | `-t` | Pre-fill the wizard's theme step |
| `--output <file>` | `-o` | Pre-fill the output filename |

The wizard always runs and needs a terminal, so an agent working
unattended should write the markdown file directly rather than call
`tap new`.

Examples:
```bash
tap new                            # interactive wizard
tap new --theme terminal           # wizard, terminal theme pre-filled
tap new --theme keynote --output launch.md
```

## tap dev

Start the development server with live reload and live code execution.

```bash
tap dev [file]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port for the dev server (default: `3000`) |
| `--presenter-password <pass>` | | Password to protect the presenter view |
| `--headless` | | Run without the TUI, for testing/automation |

Examples:
```bash
tap dev slides.md
tap dev slides.md --port 8080
```

## tap build

Build a production-ready static version. Static builds don't execute live code.

```bash
tap build <file>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--output <dir>` | `-o` | Output directory (default: `dist`) |

Examples:
```bash
tap build slides.md
tap build slides.md --output ./public
```

## tap serve

Serve a built presentation locally.

```bash
tap serve [dir]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port for the server (default: `3000`) |

## tap pdf

Export a presentation to PDF.

```bash
tap pdf <file>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--output <file>` | `-o` | Output PDF file path (default: `<input>.pdf`) |
| `--content <type>` | | Content to include: `slides`, `notes`, or `both` |

Examples:
```bash
tap pdf slides.md
tap pdf slides.md --output quarterly-review.pdf
tap pdf slides.md --content both   # slides with speaker notes
```

## tap screenshot

Render one slide, or every slide, to a PNG. Exits with status 1 when the
slide shows an error card, which makes it a self-check.

```bash
tap screenshot <file> [flags]
```

| Flag | Description |
|------|-------------|
| `--slide <n>` | One-based slide number. Required unless `--all` |
| `--all` | Every slide's final state, into a folder |
| `--step <k>` | Render that presenter step, without print mode |
| `--fragment <k>` | Render with fragments revealed through index `k`, without print mode |
| `--theme <slug>` | Render with this theme instead of the deck's own |
| `--out <path>` | Output PNG file, or folder with `--all` |
| `--width <px>` | Viewport width (default `1920`); height follows the aspect ratio |

Without `--step` or `--fragment`, the slide renders its final state in
print mode. On success it prints only the paths it wrote, one per line.

Examples:
```bash
tap screenshot deck.md --slide 12
tap screenshot deck.md --slide 12 --step 3
tap screenshot deck.md --all --out shots/
tap screenshot deck.md --slide 2 --out check.png || echo "slide 2 is broken"
```

## tap theme

```bash
tap theme list [--json]
tap theme show <slug> [--json | --prompt]
tap theme show --deck <file> [--prompt]
```

`show` prints a theme's name, polarity, pitch, tokens, and illustration
style. `--prompt` prints a style brief to paste in front of an image model
request so illustrations match the theme.

## tap add

Add a new slide interactively.

```bash
tap add [file]
```

## tap add component

Scaffold a deck-supplied React component. See
`skills/tap/rules/components.md`.

```bash
tap add component <Name> [--inline] [--ts] [--deck <file>]
```

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Enable verbose output |

Errors and warnings go to standard error; a command's real output goes to
standard output.

## Quick Reference

| Command | Description |
|---------|-------------|
| `tap new` | Create a presentation (interactive wizard) |
| `tap dev [file]` | Start dev server with live code execution |
| `tap build <file>` | Build for production (static, no live code) |
| `tap serve [dir]` | Serve built files |
| `tap pdf <file>` | Export to PDF |
| `tap screenshot <file>` | Render a slide to a PNG (exit 1 if broken) |
| `tap add [file]` | Add a slide interactively |
| `tap add component <Name>` | Scaffold a deck component |
| `tap theme list` / `tap theme show` | Inspect a theme's tokens and style |
