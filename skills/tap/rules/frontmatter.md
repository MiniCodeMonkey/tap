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
theme: terminal  # base plus 20 designed themes; unknown names fall back to base
```

See `skills/tap/rules/themes.md` for the full list of 21 themes with pitch and polarity.

### aspectRatio
```yaml
aspectRatio: 16:9  # Options: 16:9 (default), 4:3, 16:10
```

### slideNumbers
```yaml
slideNumbers: false  # Hide the slide number the theme draws (default: true)
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

There is no frontmatter option for fragments; auto-revealing list items is set per slide with the `fragments` directive (see `skills/tap/rules/slide-directives.md`).

## Theme Customization

### themeColors
```yaml
themeColors:          # Keys: background, text, muted, accent, codeBg
  accent: "#ffd447"
```

### customTheme
```yaml
customTheme: "./my-theme.css"  # Loaded after the built-in theme's CSS
```

## Code Display

Code colors and code size come from the active theme. There is no
frontmatter option for either. Override the theme's custom properties
with `customTheme` if you need different code colors.

## Live Code Drivers

Required for live code: every driver a block uses must be a key here. `shell: {}` declares a driver with no settings.

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
        user: ${PGUSER}
        password: ${PGPASSWORD}
  shell:
    timeout: 30
```

## Complete Example

```yaml
---
title: Database Architecture Deep Dive
author: Jane Developer
date: 2024-03-15
theme: blueprint
aspectRatio: 16:9
transition: fade
themeColors:
  accent: "#ffd447"
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
        user: ${PGUSER}
        password: ${PGPASSWORD}
---
```

## Quick Reference

| Option | Type | Default |
|--------|------|---------|
| `title` | string | File name |
| `author` | string | None |
| `date` | string | None |
| `theme` | string | `base` |
| `aspectRatio` | string | `16:9` |
| `slideNumbers` | boolean | `true` |
| `transition` | string | `fade` |
| `themeColors` | object | None |
| `customTheme` | string | None |
| `drivers` | object | None |
