---
title: Layouts Reference
---

# Layouts Reference

Quick reference for all available slide layouts in Tap.

## Layout Summary

| Layout | Description | When to Use |
|--------|-------------|-------------|
| `default` | Standard content layout | Anything that does not fit a more specific layout |
| `title` | Large centered text for opening slides | Opening slides, major section introductions |
| `section` | Section divider with prominent heading | Separating major parts of your presentation |
| `two-column` | Two equal columns (`::left`, `::right`) | Comparisons, before/after, pros/cons |
| `three-column` | Three equal columns (`::left`, `::center`, `::right`) | Process flows, multiple options, feature sets |
| `code-focus` | Maximum screen space for code blocks | Code walkthroughs, technical deep-dives |
| `big-stat` | Prominent display for statistics (`::caption`, `::figure`) | Key metrics, impressive numbers, impact statements |
| `quote` | Stylized quotation with attribution (`::attribution`) | Customer testimonials, famous quotes, key statements |
| `cover` | Full-screen background image with text overlay | Hero images, dramatic statements, visual storytelling |
| `sidebar` | Content area with sidebar for supplementary info (`::sidebar`) | Content with references, navigation-heavy slides |
| `split-media` | Media on one side, content on the other (`::media`) | Product demos, feature highlights, image explanations |
| `blank` | No default styling, full creative control | Custom designs, complex layouts, embedded content |

Content before the first `::slot` marker is always the `default` slot; a
marker line like `::right` starts a new named slot that runs to the next
marker or the end of the slide. See [Slide Directives](/reference/slide-directives)
for the full marker syntax.

## Automatic Layout Selection

A slide with no `layout:` directive gets one picked from its content, in
this order:

1. `three-column` when the slide has `left`, `center`, and `right` slots
2. `two-column` when it has `left` and `right` slots
3. `title` when it holds only an `h1`, with an optional short subtitle
4. `section` when it holds only an `h2`
5. `code-focus` when a single code block is more than half the content
6. `quote` when a blockquote is the main content
7. `default` for everything else

Writing `::left` and `::right` is therefore enough to get a two-column
slide. Set the directive when you want a layout the content would not
produce on its own, such as `big-stat` or `split-media`.

## Validation

`tap build` fails with exit status 1, and prints to standard error, when a
slide names a layout that does not exist, or uses a slot the layout does
not declare:

```
error: deck.md: slide 4: unknown layout "bogus" (valid layouts: big-stat, blank, code-focus, cover, default, quote, section, sidebar, split-media, three-column, title, two-column)
error: deck.md: slide 7: layout "quote" has no slot "caption" (valid slots: attribution, default)
```

## Layout Details

### default

Standard content layout. Renders the `default` slot with the theme's normal
typography. This is what a slide gets when nothing more specific fits, and
what an unknown layout name falls back to at render time so a deck never
goes blank on stage.

| Property | Value |
|----------|-------|
| **Slots** | `default` only |
| **Best for** | Anything that does not fit a more specific layout |

### title

Full-screen title slide with large, centered text.

| Property | Value |
|----------|-------|
| **Slots** | `default` only (uses heading hierarchy) |
| **Best for** | Opening slides, major section introductions |

### section

Section divider slide with prominent heading. Similar to title but styled as an interstitial break.

| Property | Value |
|----------|-------|
| **Slots** | `default` only (uses heading hierarchy) |
| **Best for** | Separating major parts of your presentation |

### two-column

Split the slide into two equal columns. The `default` slot spans both columns as a header.

| Property | Value |
|----------|-------|
| **Slots** | `default` (header), `left`, `right` |
| **Best for** | Comparisons, before/after, pros/cons |

```markdown
<!-- layout: two-column -->

## Comparison

### Option A
- Fast execution
- Simple setup

::right

### Option B
- More features
- Better scaling
```

### three-column

Split the slide into three equal columns. The `default` slot spans all three columns as a header.

| Property | Value |
|----------|-------|
| **Slots** | `default` (header), `left`, `center`, `right` |
| **Best for** | Process flows, multiple options, feature sets |

```markdown
<!-- layout: three-column -->

## Pipeline

### Ingest
Collect raw events.

::center

### Transform
Normalize and enrich.

::right

### Serve
Query-ready output.
```

### code-focus

Optimized for showing code with maximum screen real estate. Reduces padding and uses the full slide area.

| Property | Value |
|----------|-------|
| **Slots** | `default` only (code block takes full space) |
| **Best for** | Code walkthroughs, technical deep-dives |

### big-stat

Display a large statistic or number prominently, with an optional caption and supporting figure.

| Property | Value |
|----------|-------|
| **Slots** | `default` (the number/statement), `caption`, `figure` |
| **Best for** | Key metrics, impressive numbers, impact statements |

```markdown
<!-- layout: big-stat -->

# 3.2x

::caption

Faster than our previous release

::figure

![Quarterly trend](trend.png)
```

### quote

Stylized layout for quotations with attribution support.

| Property | Value |
|----------|-------|
| **Slots** | `default` (the blockquote), `attribution` |
| **Best for** | Customer testimonials, famous quotes, key statements |

```markdown
<!-- layout: quote -->

> Simplicity is the ultimate sophistication.

::attribution

Leonardo da Vinci
```

### cover

Full-screen background image with overlaid text.

| Property | Value |
|----------|-------|
| **Slots** | `default` only (content overlays the background) |
| **Directive options** | `background: <path>` |
| **Best for** | Hero images, dramatic statements, visual storytelling |

### sidebar

Content with a sidebar area for notes, navigation, or supplementary information.

| Property | Value |
|----------|-------|
| **Slots** | `default` (main content), `sidebar` |
| **Best for** | Content with references, navigation-heavy slides |

```markdown
<!-- layout: sidebar -->

## Release Notes

Everything shipping in this version.

::sidebar

### Also in this release
- Bug fixes
- Performance tuning
```

### split-media

Split layout with media on one side and content on the other.

| Property | Value |
|----------|-------|
| **Slots** | `default` (content), `media` |
| **Best for** | Product demos, feature highlights, image explanations |

```markdown
<!-- layout: split-media -->

## New Dashboard

Redesigned for at-a-glance status.

::media

![Dashboard screenshot](dashboard.png)
```

### blank

Completely empty layout with no default styling. Full creative control.

| Property | Value |
|----------|-------|
| **Slots** | `default` only |
| **Best for** | Custom designs, complex layouts, embedded content |

## Component Layouts

A `layout:` value that starts with `./` or `../` and ends in `.jsx`,
`.tsx`, `.js`, or `.ts` is not a layout name: it is a path to a React
component file next to the deck, which renders the whole slide.

```markdown
<!--
layout: ./slides/RollingDeploy.jsx
-->

# Zero-downtime deploys
```

A component layout accepts any slot name, because only the component knows
which ones it renders. See
[Components Reference](/reference/components-reference).

## See Also

- [Layouts Guide](/guide/layouts) - In-depth guide with code examples
- [Slide Directives](/reference/slide-directives) - How to apply layouts to slides and the full slot marker syntax
- [Components Reference](/reference/components-reference) - Component layouts and inline components
