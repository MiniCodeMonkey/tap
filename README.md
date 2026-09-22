# Tap

A markdown-based presentation tool for technical presentations with beautiful defaults, live code execution, and a developer-first experience.

## Features

- **Plain markdown.** Slides separated by `---`, named slots with `::name`
  marker lines, fragments with `<!-- pause -->`, speaker notes in an HTML
  comment.
- **12 layouts** with declared slots, from `title` and `big-stat` to
  `split-media` and `blank`.
- **21 themes**, `base` plus 20 designed ones, each built for a kind of
  talk. Press `t` to cycle them live, or pass `?theme=<slug>` in the URL.
- **No network on stage.** Fonts, the syntax highlighter, and the asciinema
  player are all bundled into the binary.
- **Live code execution** against SQLite, MySQL, PostgreSQL, shell, or any
  driver you define, in `tap dev`.
- **Deck-supplied React components.** Drop a `.jsx` file next to the deck
  and use it as a whole slide or as an inline block; tap bundles it itself,
  with no Node install. See `docs/guide/custom-components.md`.
- **Mermaid diagrams, asciinema recordings, and animated maps** as fenced
  blocks.
- **Presenter view** with notes, a timer, the next slide, and live fragment
  and step mirroring across devices.
- **Talk recording.** Press `c` in `tap dev` to record the screen and your
  microphone, with a chapter list of slide timings written beside the video.
  macOS only.
- **`tap present`.** Give the talk without live reload, and record every
  run automatically, following the projector across HDMI swaps.
- **A fixed 1920px canvas** that scales as one unit, so a deck looks the
  same on every projector.
- **Static builds and PDF export**, plus `tap export images` for checking a
  single slide from a script.

## Quick Start

```bash
# Install Tap
go install github.com/MiniCodeMonkey/tap@latest

# Create a new presentation
tap new --output my-presentation.md

# Start the dev server with hot reload
tap dev my-presentation.md
```

Open http://localhost:3000 to view your presentation. The presenter view is available at http://localhost:3000/presenter.

## Installation

### Using Go Install (Recommended)

```bash
go install github.com/MiniCodeMonkey/tap@latest
```

Requires Go 1.21 or later.

### Download Pre-built Binaries

Download the latest release for your platform from the [Releases page](https://github.com/MiniCodeMonkey/tap/releases):

- **macOS (Apple Silicon)**: `tap_darwin_arm64.tar.gz`
- **macOS (Intel)**: `tap_darwin_amd64.tar.gz`
- **Linux (x86_64)**: `tap_linux_amd64.tar.gz`
- **Linux (ARM64)**: `tap_linux_arm64.tar.gz`
- **Windows (x86_64)**: `tap_windows_amd64.zip`

Extract and add to your PATH:

```bash
# macOS/Linux
tar -xzf tap_darwin_arm64.tar.gz
sudo mv tap /usr/local/bin/

# Windows (PowerShell)
Expand-Archive tap_windows_amd64.zip -DestinationPath C:\tap
# Add C:\tap to your PATH
```

### Using Homebrew (macOS/Linux)

```bash
brew install MiniCodeMonkey/tap/tap
```

## Basic Usage

### Create a New Presentation

```bash
tap new                    # Interactive wizard
tap new -t terminal        # Pre-fill the wizard's theme step
tap new -o slides.md       # Pre-fill the output filename
```

`tap new` always runs the wizard and needs a terminal; the flags pre-fill
steps rather than skip them.

### Start Dev Server

```bash
tap dev slides.md          # Start dev server on port 3000
tap dev slides.md -p 8080  # Use custom port
tap dev slides.md --presenter-password secret  # Password-protect presenter view
```

### Build Static HTML

```bash
tap build slides.md        # Build to ./dist
tap build slides.md -o public  # Custom output directory
```

### Serve Static Build

```bash
tap serve                  # Serve ./dist on port 3000
tap serve public -p 8080   # Custom directory and port
```

### Export to PDF

```bash
tap export pdf slides.md          # Export to slides.pdf
tap export pdf slides.md -o presentation.pdf  # Custom output path
tap export pdf slides.md --content notes      # Include speaker notes
tap export pdf slides.md --content both       # Include slides and notes
```

### Add Slides Interactively

```bash
tap slide add                    # Add slide to auto-detected file
tap slide add slides.md          # Add slide to specific file
```

### Screenshot a Slide

```bash
tap export images slides.md --slide 4                 # final state of slide 4
tap export images slides.md --slide 4 --step 2        # slide 4 at presenter step 2
tap export images slides.md --all -o shots/            # every slide
```

Exits with status 1 when the slide shows an error card, so a script can
check a slide without looking at the image.

### Inspect a Theme

```bash
tap theme list                       # every built-in theme
tap theme show blueprint             # tokens and illustration style
tap theme show blueprint --prompt    # style brief for an image model
```

### Scaffold a Deck Component

```bash
tap component new RollingDeploy            # slides/RollingDeploy.jsx
tap component new LatencyDrop --inline     # components/LatencyDrop.jsx
```

## Command Reference

| Command | Description | Key Options |
|---------|-------------|-------------|
| `tap new [deck]` | Create a new presentation | `-t, --theme`, `-o, --output` |
| `tap dev [deck]` | Start dev server with hot reload | `-p, --port`, `--presenter-password`, `--lan` |
| `tap present [deck]` | Give a talk: no reload, audience view opens, optional auto-recording | `-p, --port`, `--no-record`, `--lan` |
| `tap build [deck]` | Build static HTML output | `-o, --output` |
| `tap serve [dir]` | Serve static files | `-p, --port` |
| `tap export pdf [deck]` | Export presentation to PDF | `-o, --output`, `--content` |
| `tap slide add [deck]` | Add slides interactively | - |
| `tap slide list [deck]` | List each slide, its lines, layout and errors | `--json` |
| `tap export images [deck]` | Render a slide to a PNG | `--slide`, `--all`, `--step`, `--fragment`, `-t, --theme`, `-o, --output`, `--width` |
| `tap component new <Name> [deck]` | Scaffold a deck component | `--inline`, `--ts` |
| `tap deck schema` | List every frontmatter key, type and default | `--json` |
| `tap theme list` | List every built-in theme | `--json` |
| `tap theme show [slug\|deck]` | Show a theme's tokens and style | `--json`, `--prompt` |
| `tap --version` | Show version | - |
| `tap --help` | Show help | - |

`[deck]` is optional everywhere it appears: a deck file, a deck folder, or
left out to use the only deck in the current folder. See
`docs/reference/cli-commands.md` for every flag and the `--json` shape of
each command.

## Writing Presentations

### Basic Structure

Slides are separated by `---` on its own line:

```markdown
---
title: My Presentation
theme: terminal
---

# Welcome

First slide content

---

## Second Slide

More content here
```

### Frontmatter Configuration

Configure your presentation using YAML frontmatter at the top of your file:

```yaml
---
title: My Presentation
theme: terminal          # base plus 20 designed themes, see docs/guide/themes.md
author: Your Name
date: "2024-01-15"
aspectRatio: "16:9"     # 16:9, 4:3, 16:10
transition: fade        # none, fade, slide, push, zoom
themeColors:            # optional: override individual theme colors
  accent: "#ffd447"

# Configure code execution drivers
drivers:
  shell:
    timeout: 30
  sqlite:
    connections:
      default:
        database: ":memory:"
  mysql:
    connections:
      prod:
        host: $DB_HOST
        port: 3306
        user: $DB_USER
        password: $DB_PASSWORD
        database: mydb
  python:
    command: python3
    args: ["-c"]
    timeout: 10
---
```

### Slide Directives

Add per-slide settings using HTML comments at the start of a slide:

```markdown
---

<!--
layout: two-column
transition: slide
background: "#1a1a2e"
notes: |
  Speaker notes go here.
  They can be multiline.
-->

## Slide Content
```

Add `skip: true` to a slide's directive block to leave it out of the talk
without deleting it: presenting, slide counts, `tap build` and `tap
export` leave it out, but it keeps its number and `tap dev` still shows it
when you open it directly. See `docs/reference/slide-directives.md`.

### Available Layouts

- `title` - Centered title with optional subtitle
- `section` - Large section header
- `default` - Standard content layout
- `two-column` - Side-by-side columns (`::right` marker)
- `three-column` - Three columns (`::center` and `::right` markers)
- `code-focus` - Full-width code block
- `quote` - Styled blockquote (`::attribution` marker)
- `big-stat` - Large number emphasis (`::caption`, `::figure` markers)
- `cover` - Full-bleed background image
- `sidebar` - Main content with sidebar (`::sidebar` marker)
- `split-media` - Image + text side-by-side (`::media` marker)
- `blank` - Empty canvas

### Fragments (Incremental Reveals)

Use `<!-- pause -->` to create incremental reveals:

```markdown
## Features

- First point

<!-- pause -->

- Second point (revealed on click)

<!-- pause -->

- Third point
```

### Live Code Execution

Execute code blocks during your presentation:

```markdown
```sql {driver: sqlite, connection: default}
SELECT * FROM users WHERE active = 1;
```
```

Supported built-in drivers:
- `shell` - Execute shell commands
- `sqlite` - SQLite queries
- `mysql` - MySQL queries
- `postgres` - PostgreSQL queries

Define custom drivers in frontmatter for any language.

### Image Attributes

Control image sizing and position:

```markdown
![Alt text](image.png){width=50%}
![Alt text](image.png){position=left}
![Alt text](image.png){width=75%, position=center}
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `→` `↓` `Space` `Enter` | Next slide/fragment |
| `←` `↑` `Backspace` | Previous slide/fragment |
| `Home` | First slide |
| `End` | Last slide |
| `PageDown` | Next slide/fragment |
| `PageUp` | Previous slide/fragment |
| `O` | Toggle overview |
| `S` | Open presenter view |
| `T` | Cycle themes |
| `F` | Toggle fullscreen |
| `Esc` | Exit fullscreen/overview |

## Themes

Tap ships `base` plus 20 designed themes: `terminal`, `product`, `swiss`,
`newsprint`, `zine`, `poster`, `blueprint`, `riso`, `retro-computing`,
`paperback`, `keynote`, `editorial`, `observatory`, `arcade`, `isometric`,
`ink`, `lab-notebook`, `bauhaus`, `sketch`, and `transit`.

Set one in frontmatter with `theme:`. `base` is the fallback used when a
deck names no theme or names one that does not exist. Every theme is
CSS-only and bundles its own fonts, so nothing loads from the network
during a talk.

Run `tap theme list` for the full list with each theme's polarity and the
kind of talk it was built for, or see `docs/guide/themes.md`.

## Environment Variables

Use environment variables in your configuration for secrets:

```yaml
drivers:
  mysql:
    connections:
      prod:
        password: $DB_PASSWORD
```

Tap automatically loads `.env` files from the presentation directory.

## Development

```bash
# Build from source
make build

# Run tests
make test

# Run linter
make lint

# Build for all platforms
make release
```

## License

MIT
