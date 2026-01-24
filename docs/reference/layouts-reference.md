---
title: Layouts Reference
---

# Layouts Reference

Quick reference for all available slide layouts in Tap.

## Layout Summary

| Layout | Description | When to Use |
|--------|-------------|-------------|
| `title` | Large centered text for opening slides | Opening slides, major section introductions |
| `section` | Section divider with prominent heading | Separating major parts of your presentation |
| `two-column` | Two equal columns separated by `|||` | Comparisons, before/after, pros/cons |
| `three-column` | Three equal columns separated by `|||` | Process flows, multiple options, feature sets |
| `code-focus` | Maximum screen space for code blocks | Code walkthroughs, technical deep-dives |
| `big-stat` | Prominent display for statistics | Key metrics, impressive numbers, impact statements |
| `quote` | Stylized quotation with attribution | Customer testimonials, famous quotes, key statements |
| `cover` | Full-screen background image with text overlay | Hero images, dramatic statements, visual storytelling |
| `sidebar` | Content area with sidebar for supplementary info | Content with references, navigation-heavy slides |
| `split-media` | Media on one side, content on the other | Product demos, feature highlights, image explanations |
| `blank` | No default styling, full creative control | Custom designs, complex layouts, embedded content |

## Layout Details

### title

Full-screen title slide with large, centered text.

| Property | Value |
|----------|-------|
| **Slot markers** | None (uses heading hierarchy) |
| **Best for** | Opening slides, major section introductions |

### section

Section divider slide with prominent heading. Similar to title but styled as an interstitial break.

| Property | Value |
|----------|-------|
| **Slot markers** | None (uses heading hierarchy) |
| **Best for** | Separating major parts of your presentation |

### two-column

Split the slide into two equal columns.

| Property | Value |
|----------|-------|
| **Separator** | `|||` |
| **Best for** | Comparisons, before/after, pros/cons |

### three-column

Split the slide into three equal columns.

| Property | Value |
|----------|-------|
| **Separator** | `|||` (used twice) |
| **Best for** | Process flows, multiple options, feature sets |

### code-focus

Optimized for showing code with maximum screen real estate. Reduces padding and uses the full slide area.

| Property | Value |
|----------|-------|
| **Slot markers** | None (code block takes full space) |
| **Best for** | Code walkthroughs, technical deep-dives |

### big-stat

Display a large statistic or number prominently.

| Property | Value |
|----------|-------|
| **Slot markers** | None (uses heading hierarchy) |
| **Best for** | Key metrics, impressive numbers, impact statements |

### quote

Stylized layout for quotations with attribution support.

| Property | Value |
|----------|-------|
| **Slot markers** | None (uses blockquote syntax) |
| **Best for** | Customer testimonials, famous quotes, key statements |

### cover

Full-screen background image with overlaid text.

| Property | Value |
|----------|-------|
| **Slot markers** | None (content overlays background) |
| **Directive options** | `background: <path>` |
| **Best for** | Hero images, dramatic statements, visual storytelling |

### sidebar

Content with a sidebar area for notes, navigation, or supplementary information.

| Property | Value |
|----------|-------|
| **Separator** | `|||` (main content first, then sidebar) |
| **Best for** | Content with references, navigation-heavy slides |

### split-media

Split layout with media on one side and content on the other.

| Property | Value |
|----------|-------|
| **Separator** | `|||` (media and content in either order) |
| **Best for** | Product demos, feature highlights, image explanations |

### blank

Completely empty layout with no default styling. Full creative control.

| Property | Value |
|----------|-------|
| **Slot markers** | None |
| **Best for** | Custom designs, complex layouts, embedded content |

## Column Separator Reference

Multi-column layouts use `|||` as a separator between content sections:

| Layout | Usage |
|--------|-------|
| `two-column` | `content ||| content` |
| `three-column` | `content ||| content ||| content` |
| `sidebar` | `main content ||| sidebar content` |
| `split-media` | `media ||| content` (or reverse) |

See the [Layouts Guide](/guide/layouts) for detailed examples with code snippets.

## See Also

- [Layouts Guide](/guide/layouts) — In-depth guide with code examples
- [Slide Directives](/reference/slide-directives) — How to apply layouts to slides
