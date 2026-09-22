---
title: Frontmatter Options
---

# Frontmatter Options

Complete reference for all frontmatter configuration options in Tap presentations.

## Overview

Frontmatter is YAML configuration at the start of your presentation file, enclosed in triple dashes (`---`). These settings apply globally to your entire presentation unless overridden by slide directives.

`tap deck schema` lists every key on this page with its type, default and allowed values, and `tap deck schema --json` prints them for editors and tools.

```yaml
---
title: My Presentation
theme: terminal
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

The visual theme applied to all slides. Themes control typography, colors, animations, transitions, and spacing. An unknown theme name falls back to `base` with a warning.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `base` |
| Required | No |

```yaml
---
theme: terminal
---
```

**Available themes:**

| Theme | Polarity | Pitch |
|-------|----------|-------|
| `base` | light | A plain, readable default theme with no strong identity, used when a deck names no theme or names one that doesn't exist. |
| `terminal` | dark | For infrastructure and live-coding talks where the audience lives in a shell and the deck should feel like one long tmux session. |
| `product` | light | A launch-day talk for a developer tool, where every slide should feel like the landing page of something you want to install. |
| `swiss` | light | An argument-driven engineering talk set like a Zurich concert poster: grid, grotesk, one red. |
| `newsprint` | light | A war story or postmortem told as front-page news, where every slide is a headline and every number is a front-page figure. |
| `zine` | light | An opinionated, scrappy talk with a point to argue: cut, pasted, and photocopied the night before, and proud of it. |
| `poster` | light | For a loud, fast, opinionated talk in the Takahashi style, where every slide is a gig poster and the words do all the work. |
| `blueprint` | dark | For architecture and systems talks where every slide is a sheet from the drawing set and the diagrams are the argument. |
| `riso` | light | A warm community-conference talk that should feel like a two-ink print pulled off the drum that morning. |
| `retro-computing` | light | A nostalgic, slightly mischievous systems talk told from a 1990 desktop, where every idea opens in its own window. |
| `paperback` | light | A story-led conference talk that wants the warmth and quiet authority of a mid-century paperback series. |
| `keynote` | dark | A big-room launch talk where one idea at a time lands on a dark stage, lit from above. |
| `editorial` | light | A story-led talk told like a long-read magazine feature, with drop caps, pull quotes, and figures. |
| `observatory` | dark | A data-heavy infrastructure talk, charted like a night sky: systems as bodies, readings taken through a reticle. |
| `arcade` | dark | A high-energy war-story talk framed as a game you can win, built to stay legible on a washed-out projector. |
| `isometric` | light | A friendly architecture walkthrough where the infrastructure itself is the cast: code, tables, and numbers become chunky little solids on a warm floor. |
| `ink` | light | A calm, reflective talk about hard-won lessons, where each slide holds one thought and a lot of silence around it. |
| `lab-notebook` | light | An evidence-first engineering talk where every claim arrives as a numbered figure and the speaker's red pen does the arguing. |
| `bauhaus` | light | A bold, opinionated talk about first principles, where every idea is reduced to a circle, a triangle, or a square. |
| `sketch` | light | For explainer talks that build an idea step by step, as if the speaker were drawing it on a whiteboard in front of you. |
| `transit` | light | An architecture or infrastructure talk that walks the audience through a system the way a metro map walks a city. |

Press `t` during a presentation to cycle themes live, or force one with the `?theme=<slug>` query parameter. See [Themes](/guide/themes) for the full guide and `docs/reference/theme-porting.md` if you want to design a new one.

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

Every slide renders on a fixed 1920px-wide canvas (1920 x 1920/aspectRatio
px) and that whole canvas scales as one unit to fit the screen it's shown
on, so a deck looks the same on every projector, laptop, or presenter
panel. Themes are tuned against the 16:9 canvas.

### slideNumbers

Whether the theme draws a slide number on each slide. Most themes show one,
for example `3/12` in the terminal theme's status bar or `Page 3 of 12` in
newsprint. Set it to `false` to leave the numbers out.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `true` |
| Required | No |

```yaml
---
slideNumbers: false
---
```

The setting applies to the audience view, the presenter view previews,
screenshots, and PDF export. The presenter view's own slide counter stays.
Where a theme builds a decoration around the number, the decoration stays
and only the number goes: transit keeps an empty station marker, blueprint
keeps the drawing row of its title block, and terminal keeps the window tab
on the left of its status bar. In zine, the "N PAGES!" burst on the title
slide also goes, because it shows the slide count.

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

Code colors and code text size come from the active theme, not from
frontmatter. Every theme sets the Shiki CSS variables its stylesheet
defines, so syntax highlighting always matches the deck's theme. To
change code colors, override the theme's custom properties with
`customTheme`, or pick a different theme.

## Theme Customization

### themeColors

Override a handful of the active theme's colors without writing a whole
theme.

| Property | Value |
|----------|-------|
| Type | `object` |
| Default | None |
| Required | No |

```yaml
---
theme: terminal
themeColors:
  accent: "#ff0000"
  background: "#0d0d0d"
---
```

Accepted keys: `background`, `text`, `muted`, `accent`, `codeBg`.

### customTheme

Path to your own CSS file, loaded after the built-in theme's CSS so it can
override any of the theme's custom properties or rules.

| Property | Value |
|----------|-------|
| Type | `string` (path, relative to the deck) |
| Default | None |
| Required | No |

```yaml
---
theme: base
customTheme: "./my-theme.css"
---
```

See [Creating Themes](/reference/theme-porting) for the full set of custom
properties a theme defines and the selector conventions to follow.

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
theme: blueprint
aspectRatio: 16:9
transition: fade
fragments: true
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
| `theme` | string | `base` | Visual theme |
| `aspectRatio` | string | `16:9` | Slide aspect ratio |
| `slideNumbers` | boolean | `true` | Show the theme's slide numbers |
| `transition` | string | `fade` | Default slide transition |
| `fragments` | boolean | `false` | Auto-reveal list items |
| `themeColors` | object | None | Override individual theme colors |
| `customTheme` | string | None | Path to your own CSS file |
| `drivers` | object | None | Live code execution config |

## Next Steps

- [Slide Directives](/reference/slide-directives) - Per-slide configuration options
- [Themes](/guide/themes) - Detailed theme descriptions
- [Drivers Reference](/reference/drivers) - Complete driver configuration
- [Animations & Transitions](/guide/animations-transitions) - Transition and fragment options
