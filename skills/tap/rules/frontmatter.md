# Frontmatter Options

Frontmatter is YAML at the start of your file, enclosed in `---`. Settings apply globally unless overridden by slide directives.

## Presentation Metadata

```yaml
---
title: Quarterly Business Review    # Browser tab, PDF title
author: Jane Developer              # Metadata
date: 2024-01-15                    # Version tracking
---
```

## Visual Appearance

### theme
```yaml
theme: paper  # Options: paper, noir, aurora, phosphor, poster
```

| Theme | Description |
|-------|-------------|
| `paper` | Ultra-clean, premium design with warm accents |
| `noir` | Cinematic, sophisticated with gold highlights |
| `aurora` | Vibrant gradient mesh with glassmorphism |
| `phosphor` | CRT aesthetic with phosphor green glow |
| `poster` | Bold graphic design with thick borders |

### aspectRatio
```yaml
aspectRatio: 16:9  # Options: 16:9 (default), 4:3, 16:10
```

## Animations

### transition
```yaml
transition: fade  # Options: none, fade, slide, push, zoom
```

| Transition | Effect |
|------------|--------|
| `none` | Instant switch |
| `fade` | Crossfade |
| `slide` | Horizontal slide |
| `push` | Push old slide out |
| `zoom` | Zoom effect |

### fragments
```yaml
fragments: true  # Auto-reveal list items one at a time
```

## Code Display

### codeTheme
```yaml
codeTheme: github-dark  # Shiki theme for syntax highlighting
```

Popular themes: `github-dark`, `github-light`, `nord`, `dracula`, `one-dark-pro`, `monokai`

### codeFontSize
```yaml
codeFontSize: 14px  # Adjust for readability
```

## Live Code Drivers

```yaml
drivers:
  sqlite:
    connections:
      demo:
        path: ./data/demo.db
  postgres:
    connections:
      analytics:
        host: localhost
        database: analytics
        user: $PGUSER
        password: $PGPASSWORD
  shell:
    timeout: 30
```

## Complete Example

```yaml
---
title: Database Architecture Deep Dive
author: Jane Developer
date: 2024-03-15
theme: phosphor
aspectRatio: 16:9
transition: fade
fragments: true
codeTheme: github-dark
codeFontSize: 14px
drivers:
  sqlite:
    connections:
      demo:
        path: ./demo.db
  postgres:
    connections:
      analytics:
        host: localhost
        database: analytics
        user: $PGUSER
        password: $PGPASSWORD
---
```

## Quick Reference

| Option | Type | Default |
|--------|------|---------|
| `title` | string | File name |
| `author` | string | None |
| `theme` | string | `paper` |
| `aspectRatio` | string | `16:9` |
| `transition` | string | `fade` |
| `fragments` | boolean | `false` |
| `codeTheme` | string | Theme default |
| `codeFontSize` | string | `16px` |
| `drivers` | object | None |
