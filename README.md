# Tap

A markdown-based presentation tool for technical presentations with beautiful defaults, live code execution, and a developer-first experience.

## Quick Start

```bash
# Install Tap
go install github.com/tapsh/tap@latest

# Create a new presentation
tap new my-presentation

# Start the dev server with hot reload
tap dev my-presentation.md
```

Open http://localhost:3000 to view your presentation. The presenter view is available at http://localhost:3000/presenter.

## Installation

### Using Go Install (Recommended)

```bash
go install github.com/tapsh/tap@latest
```

Requires Go 1.21 or later.

### Download Pre-built Binaries

Download the latest release for your platform from the [Releases page](https://github.com/tapsh/tap/releases):

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
brew install tapsh/tap/tap
```

## Basic Usage

### Create a New Presentation

```bash
tap new                    # Interactive wizard
tap new -t gradient        # Specify theme
tap new -o slides.md       # Specify output file
```

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
tap pdf slides.md          # Export to slides.pdf
tap pdf slides.md -o presentation.pdf  # Custom output path
tap pdf slides.md --content notes      # Include speaker notes
tap pdf slides.md --content both       # Include slides and notes
```

### Add Slides Interactively

```bash
tap add                    # Add slide to auto-detected file
tap add slides.md          # Add slide to specific file
```

## Command Reference

| Command | Description | Key Options |
|---------|-------------|-------------|
| `tap new` | Create a new presentation | `-t, --theme`, `-o, --output` |
| `tap dev <file>` | Start dev server with hot reload | `-p, --port`, `--presenter-password` |
| `tap build <file>` | Build static HTML output | `-o, --output` |
| `tap serve [dir]` | Serve static files | `-p, --port` |
| `tap pdf <file>` | Export presentation to PDF | `-o, --output`, `--content` |
| `tap add [file]` | Add slides interactively | - |
| `tap --version` | Show version | - |
| `tap --help` | Show help | - |

## Writing Presentations

### Basic Structure

Slides are separated by `---` on its own line:

```markdown
---
title: My Presentation
theme: minimal
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
theme: minimal          # minimal, gradient, terminal, brutalist, keynote
author: Your Name
date: "2024-01-15"
aspectRatio: "16:9"     # 16:9, 4:3, 16:10
transition: fade        # none, fade, slide, push, zoom
codeTheme: github-dark
fragments: true

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

### Available Layouts

- `title` - Centered title with optional subtitle
- `section` - Large section header
- `default` - Standard content layout
- `two-column` - Side-by-side columns (use `|||` separator)
- `three-column` - Three columns (use `|||` separators)
- `code-focus` - Full-width code block
- `quote` - Styled blockquote
- `big-stat` - Large number emphasis
- `cover` - Full-bleed background image
- `sidebar` - Main content with sidebar
- `split-media` - Image + text side-by-side
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
| `O` | Toggle overview |
| `S` | Open presenter view |
| `F` | Toggle fullscreen |
| `Esc` | Exit fullscreen/overview |

## Themes

Tap includes five built-in themes:

- **minimal** - Clean Apple-style aesthetics (default)
- **gradient** - Modern colorful gradients with glassmorphism
- **terminal** - Hacker aesthetic with CRT effects
- **brutalist** - Bold, geometric, high contrast
- **keynote** - Professional with subtle shadows

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
