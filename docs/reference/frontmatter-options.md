---
title: Frontmatter Options
---

# Frontmatter Options

Complete reference for all frontmatter configuration options in Tap presentations.

## Overview

Frontmatter is YAML configuration at the start of your presentation file, enclosed in triple dashes (`---`). These settings apply globally to your entire presentation unless overridden by slide directives.

```yaml
---
title: My Presentation
theme: paper
author: Jane Developer
transition: fade
---
```

## Presentation Metadata

### title

The presentation title, displayed in the browser tab and used for PDF exports.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | File name |
| Required | No |

```yaml
---
title: Quarterly Business Review
---
```

### author

Author name for presentation metadata.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | None |
| Required | No |

```yaml
---
author: Jane Developer
---
```

### date

Presentation date, useful for version tracking and PDF metadata.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | None |
| Required | No |

```yaml
---
date: 2024-01-15
---
```

## Visual Appearance

### theme

The visual theme applied to all slides. Themes control typography, colors, animations, transitions, and spacing.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `paper` |
| Required | No |
| Options | `paper`, `noir`, `aurora`, `phosphor`, `poster` |

```yaml
---
theme: phosphor
---
```

**Available themes:**

| Theme | Description |
|-------|-------------|
| `paper` | Ultra-clean, premium design with warm accents |
| `noir` | Cinematic, sophisticated with gold highlights |
| `aurora` | Vibrant gradient mesh with glassmorphism |
| `phosphor` | CRT aesthetic with phosphor green glow |
| `poster` | Bold graphic design with thick borders |

See [Themes](/guide/themes) for detailed descriptions and examples.

### aspectRatio

Slide aspect ratio. Defines the width-to-height ratio of your slides.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `16:9` |
| Required | No |
| Options | `16:9`, `4:3`, `16:10` |

```yaml
---
aspectRatio: 4:3
---
```

**Common aspect ratios:**

| Ratio | Use Case |
|-------|----------|
| `16:9` | Modern widescreen (default) |
| `4:3` | Traditional/legacy projectors |
| `16:10` | Widescreen laptops/displays |

::: tip
Most modern projectors and displays use 16:9. Use 4:3 only if you know your venue has older equipment.
:::

## Animations and Transitions

### transition

Default slide transition animation applied when advancing between slides.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `fade` |
| Required | No |
| Options | `none`, `fade`, `slide`, `push`, `zoom` |

```yaml
---
transition: slide
---
```

**Transition types:**

| Transition | Effect |
|------------|--------|
| `none` | Instant switch, no animation |
| `fade` | Crossfade between slides |
| `slide` | Slide horizontally (left to right) |
| `push` | New slide pushes old slide out |
| `zoom` | Zoom in/out effect |

Individual slides can override this using the `transition` directive. See [Animations & Transitions](/guide/animations-transitions).

### fragments

Enable automatic fragment reveals for list items. When enabled, bullet points appear one at a time as you advance.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |
| Required | No |

```yaml
---
fragments: true
---
```

When `true`, all bullet lists in the presentation will reveal incrementally. Individual slides can override this with the `fragments` directive.

See [Animations & Transitions](/guide/animations-transitions) for more on fragments and the `<!-- pause -->` directive.

## Code Display

### codeTheme

Syntax highlighting theme for code blocks. Uses Shiki themes.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | Theme-dependent |
| Required | No |

```yaml
---
codeTheme: github-dark
---
```

**Popular code themes:**

| Theme | Style |
|-------|-------|
| `github-dark` | GitHub's dark mode colors |
| `github-light` | GitHub's light mode colors |
| `nord` | Arctic, bluish color palette |
| `dracula` | Popular dark theme |
| `one-dark-pro` | Atom One Dark colors |
| `monokai` | Classic dark theme |
| `min-light` | Minimal light theme |

::: tip
Each presentation theme sets a sensible default code theme. Only override if you want a specific look.
:::

### codeFontSize

Font size for code blocks. Adjust for readability at presentation distance.

| Property | Value |
|----------|-------|
| Type | `string` (CSS value) |
| Default | `16px` |
| Required | No |

```yaml
---
codeFontSize: 14px
---
```

**Recommended sizes:**

| Size | Use Case |
|------|----------|
| `18px` | Large venue, few lines of code |
| `16px` | Default, standard presentations |
| `14px` | More code on screen |
| `12px` | Dense code, close viewing |

## Live Code Execution

### drivers

Configure live code execution drivers. Each driver connects to a different backend for running code during presentations.

| Property | Value |
|----------|-------|
| Type | `object` |
| Default | None |
| Required | No (only for live code execution) |

```yaml
---
drivers:
  sqlite:
    database: ./data/demo.db
  postgres:
    host: localhost
    port: 5432
    database: analytics
    user: $PGUSER
    password: $PGPASSWORD
  shell:
    cwd: ./scripts
    timeout: 30
---
```

**Available drivers:**

| Driver | Description |
|--------|-------------|
| `sqlite` | SQLite database queries |
| `mysql` | MySQL/MariaDB database queries |
| `postgres` | PostgreSQL database queries |
| `shell` | Shell/Bash command execution |

::: warning
Live code execution only works with `tap dev`. Static builds (`tap build`) do not execute code.
:::

**Environment variables:** Values starting with `$` are replaced with environment variables. Never hardcode passwords in your files.

See [Drivers Reference](/reference/drivers) for complete driver configuration options.

## Complete Example

Here's a comprehensive frontmatter example using multiple options:

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
    database: ./demo.db
  postgres:
    host: localhost
    database: analytics
    user: $PGUSER
    password: $PGPASSWORD
    timeout: 30
---
```

## Quick Reference

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `title` | string | File name | Presentation title |
| `author` | string | None | Author name |
| `date` | string | None | Presentation date |
| `theme` | string | `minimal` | Visual theme |
| `aspectRatio` | string | `16:9` | Slide aspect ratio |
| `transition` | string | `fade` | Default slide transition |
| `fragments` | boolean | `false` | Auto-reveal list items |
| `codeTheme` | string | Theme default | Syntax highlighting theme |
| `codeFontSize` | string | `16px` | Code block font size |
| `drivers` | object | None | Live code execution config |

## Next Steps

- [Slide Directives](/reference/slide-directives) - Per-slide configuration options
- [Themes](/guide/themes) - Detailed theme descriptions
- [Drivers Reference](/reference/drivers) - Complete driver configuration
- [Animations & Transitions](/guide/animations-transitions) - Transition and fragment options
