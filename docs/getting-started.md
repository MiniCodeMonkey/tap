---
title: Getting Started
---

# Getting Started

Get up and running with Tap in minutes.

## Install Tap

Choose your preferred installation method:

### Homebrew (macOS/Linux)

```bash
brew install MiniCodeMonkey/tap/tap
```

### Go Install

If you have Go installed:

```bash
go install github.com/tap-slides/tap@latest
```

### Binary Download

Download the latest release for your platform from the [GitHub releases page](https://github.com/tap-slides/tap/releases).

Available binaries:
- `tap-darwin-amd64` - macOS Intel
- `tap-darwin-arm64` - macOS Apple Silicon
- `tap-linux-amd64` - Linux x64
- `tap-linux-arm64` - Linux ARM64
- `tap-windows-amd64.exe` - Windows x64

After downloading, make the binary executable and move it to your PATH:

```bash
chmod +x tap-darwin-arm64
mv tap-darwin-arm64 /usr/local/bin/tap
```

## Create Your First Presentation

Use `tap new` to scaffold a new presentation:

```bash
tap new my-talk
```

This creates a new file `my-talk.md` with a basic template:

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

Launch the development server to preview your slides:

```bash
tap dev my-talk.md
```

This starts a local server (typically at `http://localhost:3000`) with:
- Live reload on file changes
- Presenter mode at `/presenter`
- Live code execution support

Use arrow keys or space to navigate between slides.

## Basic Slide Syntax

### Frontmatter

Every presentation starts with YAML frontmatter defining global settings:

```yaml
---
title: My Presentation
theme: paper
author: Your Name
date: 2024-01-15
---
```

### Slide Separators

Separate slides with three dashes on their own line:

```markdown
# First Slide

Content for the first slide.

---

# Second Slide

Content for the second slide.

---

# Third Slide

And so on...
```

### Slide Directives

Add per-slide settings using HTML comment blocks:

```markdown
---

<!--
layout: two-column
transition: fade
-->

# This Slide Has Custom Settings

Content here...
```

## Build for Production

When you're ready to share your presentation, build it as a static site:

```bash
tap build my-talk.md
```

This generates optimized HTML/CSS/JS in the `dist/` directory. You can deploy this to any static hosting service like Netlify, Vercel, or GitHub Pages.

To preview the built version locally:

```bash
tap serve dist
```

## Next Steps

Now that you have the basics, explore the guides to unlock Tap's full potential:

- [Writing Slides](/guide/writing-slides) - Deep dive into markdown syntax and speaker notes
- [Layouts](/guide/layouts) - Use different slide layouts for variety
- [Themes](/guide/themes) - Customize the look and feel
- [Code Blocks](/guide/code-blocks) - Syntax highlighting and line highlighting
- [Live Code Execution](/guide/live-code-execution) - Run SQL, shell commands, and more
- [Presenter Mode](/guide/presenter-mode) - Use speaker notes and timer during presentations
