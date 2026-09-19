# Getting Started with Tap

## Installation

### Homebrew (macOS/Linux)
```bash
brew install MiniCodeMonkey/tap/tap
```

### Go Install
```bash
go install github.com/MiniCodeMonkey/tap@latest
```

### Binary Download
Download from [GitHub releases](https://github.com/MiniCodeMonkey/tap/releases):
- `tap-darwin-amd64` - macOS Intel
- `tap-darwin-arm64` - macOS Apple Silicon
- `tap-linux-amd64` - Linux x64
- `tap-linux-arm64` - Linux ARM64
- `tap-windows-amd64.exe` - Windows x64

```bash
chmod +x tap-darwin-arm64
mv tap-darwin-arm64 /usr/local/bin/tap
```

## Create Your First Presentation

`tap new` opens an interactive wizard (title, theme, filename) with a
terminal attached:

```bash
tap new --output my-talk.md --theme terminal
```

With no terminal attached, or with `--yes`, it skips the wizard and writes
the file straight from the flags -- the mode to use when working
unattended:

```bash
tap new --yes --title "My Talk" --output my-talk.md --theme terminal
```

A deck is a plain markdown file. This is all it needs:
```markdown
---
title: My Talk
theme: terminal
---

# My Talk

A one-line subtitle.

---

## What we will cover

- The problem
- What we tried
- What worked
```

## Start the Dev Server

```bash
tap dev my-talk.md
```

Starts at `http://localhost:3000` with:
- Live reload on file changes
- Presenter mode at `/presenter`
- Live code execution support

Navigate with arrow keys or space. `t` cycles themes, `o` opens the
overview, `s` opens the presenter view, `f` toggles fullscreen.

## Build for Production

```bash
tap build my-talk.md
```

Generates a self-contained `dist/` folder with relative paths and bundled
fonts. Live code execution is the one thing it cannot do. Preview with:
```bash
tap serve dist
```

## Essential Workflow

1. `tap new --yes ...` (or `tap new` for the wizard) to write the markdown file
2. `tap dev <file>` - Develop with live preview
3. `tap screenshot <file> --slide <n>` - Check one slide; exit status 1 means it is broken
4. `tap build <file>` - Build for deployment
5. `tap pdf <file>` - Export to PDF
