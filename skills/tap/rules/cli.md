# CLI Commands

Complete reference for Tap CLI commands.

## tap new

Create a new presentation.

```bash
tap new [name]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--theme <name>` | `-t` | Theme (default: `paper`) |
| `--template <type>` | | Template: `blank`, `demo`, `talk` |
| `--dir <path>` | `-d` | Directory (default: current) |
| `--force` | `-f` | Overwrite existing file |

Examples:
```bash
tap new my-talk
tap new quarterly-review --theme noir
tap new demo --template demo
tap new slides --dir ./presentations
```

## tap dev

Start development server with live reload.

```bash
tap dev <file>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port (default: `3000`) |
| `--host <ip>` | | Host (default: `localhost`) |
| `--open` | `-o` | Open browser |
| `--no-live-reload` | | Disable live reload |
| `--password <pass>` | | Presenter mode password |
| `--qr` | | Display QR code |

Examples:
```bash
tap dev slides.md
tap dev slides.md --port 8080 --open
tap dev slides.md --host 0.0.0.0  # Network access
tap dev slides.md --password secret123
```

URLs:
- `http://localhost:3000` - Audience view
- `http://localhost:3000/presenter` - Presenter view

## tap build

Build production-ready static version.

```bash
tap build <file>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--out <dir>` | `-o` | Output directory (default: `dist/`) |
| `--base <path>` | `-b` | Base path for deployment |
| `--minify` | `-m` | Enable minification |
| `--watch` | `-w` | Watch for changes |

Examples:
```bash
tap build slides.md
tap build slides.md --out ./public
tap build slides.md --base /my-repo/  # GitHub Pages
tap build slides.md --minify
```

**Note:** Static builds don't execute live code.

## tap serve

Serve built presentation locally.

```bash
tap serve [dir]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--port <number>` | `-p` | Port (default: `8080`) |
| `--host <ip>` | | Host (default: `localhost`) |
| `--open` | `-o` | Open browser |
| `--cors` | | Enable CORS |

Examples:
```bash
tap serve
tap serve ./public
tap serve dist --port 3000 --open
```

## tap pdf

Export to PDF.

```bash
tap pdf <file>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--out <file>` | `-o` | Output filename |
| `--format <type>` | `-f` | Format: `slides`, `notes`, `both` |
| `--paper <size>` | | Size: `letter`, `a4`, `16:9`, `4:3` |
| `--margin <px>` | `-m` | Page margins |
| `--quality <level>` | `-q` | Quality: `low`, `medium`, `high` |

Examples:
```bash
tap pdf slides.md
tap pdf slides.md --out quarterly-review.pdf
tap pdf slides.md --format both  # Slides with notes
tap pdf slides.md --format notes  # Speaker script
tap pdf slides.md --paper a4 --margin 20
```

## tap add

Add slide or asset to existing presentation.

```bash
tap add [file]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--layout <name>` | `-l` | Layout for new slide |
| `--position <n>` | `-p` | Insert position |
| `--title <text>` | `-t` | Slide title |
| `--image <path>` | `-i` | Add image asset |
| `--interactive` | | Interactive mode |

Examples:
```bash
tap add slides.md
tap add slides.md --layout two-column
tap add slides.md --position 3 --title "New Section"
tap add slides.md --image ./photo.png
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--help` | Show help |
| `--version` | Show version |
| `--verbose` | Verbose output |
| `--quiet` | Suppress output |
| `--no-color` | Disable colors |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `TAP_PORT` | Default dev port |
| `TAP_HOST` | Default dev host |
| `TAP_THEME` | Default theme |
| `NO_COLOR` | Disable colors |

## Quick Reference

| Command | Description |
|---------|-------------|
| `tap new [name]` | Create presentation |
| `tap dev <file>` | Start dev server |
| `tap build <file>` | Build for production |
| `tap serve [dir]` | Serve built files |
| `tap pdf <file>` | Export to PDF |
| `tap add [file]` | Add slide/asset |
