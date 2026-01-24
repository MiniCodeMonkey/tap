# Getting Started with Tap

## Installation

### Homebrew (macOS/Linux)
```bash
brew install tap-slides
```

### Go Install
```bash
go install github.com/tap-slides/tap@latest
```

### Binary Download
Download from [GitHub releases](https://github.com/tap-slides/tap/releases):
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

```bash
tap new my-talk
```

Creates `my-talk.md`:
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

## Start the Dev Server

```bash
tap dev my-talk.md
```

Starts at `http://localhost:3000` with:
- Live reload on file changes
- Presenter mode at `/presenter`
- Live code execution support

Navigate with arrow keys or space.

## Build for Production

```bash
tap build my-talk.md
```

Generates optimized HTML/CSS/JS in `dist/`. Preview with:
```bash
tap serve dist
```

## Essential Workflow

1. `tap new <name>` - Create presentation
2. `tap dev <file>` - Develop with live preview
3. `tap build <file>` - Build for deployment
4. `tap pdf <file>` - Export to PDF
