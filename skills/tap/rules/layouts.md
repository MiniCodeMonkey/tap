# Layouts

Layouts control how content is arranged on slides. Tap provides 12 built-in layouts.

## Specifying a Layout

Add a directive at the beginning of your slide:
```markdown
---

<!--
layout: two-column
-->

# Side by Side

Left column content.

::right

Right column content.
```

Content before the first `::slot` marker is the `default` slot; a marker
line like `::right` starts a new slot that runs to the next marker or the
end of the slide. A slot marker is `::` followed by a lowercase name, on a
line of its own. A duplicate slot name on one slide is a parse error, and
`tap build` fails on a slot the layout does not declare.

With no `layout:` directive, tap picks one from the content:
`three-column` or `two-column` from the slot names, `title` for a lone
`h1` with an optional short subtitle, `section` for a lone `h2`,
`code-focus` for a code block that is more than half the slide, `quote`
for a blockquote, otherwise `default`. So `::left` and `::right` alone
already give a two-column slide.

## Available Layouts

### default
Centers headings and body text. The fallback when nothing more specific
fits, and what an unknown layout name renders as.
```markdown
<!--
layout: default
-->

# My Slide Title
Body content, centered.
```
**Use for:** General-purpose slides.

### title
Full-screen title slide with large, centered text.
```markdown
<!--
layout: title
-->

# Welcome to My Talk
## A subtitle goes here
```
**Use for:** Opening slides, major section introductions.

### section
Section divider with prominent heading.
```markdown
<!--
layout: section
-->

# Part 2: The Solution
```
**Use for:** Separating major parts of your presentation.

### two-column
Two equal columns. `default` is a header spanning both; `::right` starts the right column.
```markdown
<!--
layout: two-column
-->

# Comparison

### Option A
- Fast execution
- Simple setup

::right

### Option B
- More features
- Better scaling
```
**Use for:** Comparisons, before/after, pros/cons.

### three-column
Three equal columns. `::center` and `::right` start the second and third.
```markdown
<!--
layout: three-column
-->

# Our Process

### Plan
Define requirements

::center

### Build
Write the code

::right

### Ship
Deploy to production
```
**Use for:** Process flows, multiple options, feature sets.

### code-focus
Maximized code display with reduced padding.
```markdown
<!--
layout: code-focus
-->

```python
def calculate_metrics(data):
    return {
        "mean": sum(data) / len(data),
        "max": max(data),
        "min": min(data)
    }
```
```
**Use for:** Code walkthroughs, technical deep-dives.

### big-stat
Large statistic or number prominently displayed. `::caption` adds a supporting line, `::figure` a small chart or image.
```markdown
<!--
layout: big-stat
-->

# 3.2x

::caption

Faster build times
```
**Use for:** Key metrics, impressive numbers, impact statements.

### quote
Stylized quotation with an `::attribution` slot for the source.
```markdown
<!--
layout: quote
-->

> The best code is no code at all.

::attribution

Jeff Atwood
```
**Use for:** Customer testimonials, famous quotes.

### cover
Full-screen background image with overlaid text.
```markdown
<!--
layout: cover
background: ./images/hero.jpg
-->

# Bold Statement
## On a beautiful background
```
**Use for:** Hero images, dramatic statements.

### sidebar
Content with a sidebar area. `::sidebar` starts the sidebar content.
```markdown
<!--
layout: sidebar
-->

# Main Content
The primary focus of this slide.

::sidebar

**Related:**
- Topic A
- Topic B
```
**Use for:** Content with references, navigation-heavy slides.

### split-media
Media and content side by side. `::media` starts the media slot.
```markdown
<!--
layout: split-media
-->

# New Feature
Introducing our latest improvement.

::media

![Product screenshot](./images/product.png)
```
**Use for:** Product demos, feature highlights.

### blank
Empty layout with no default styling.
```markdown
<!--
layout: blank
-->

<div style="display: flex; justify-content: center; height: 100%;">
  Custom HTML content
</div>
```
**Use for:** Custom designs, embedded content.

## Layout Reference

| Layout | Slots | Best For |
|--------|-------|----------|
| `default` | `default` | General-purpose slides |
| `title` | `default` | Opening slides |
| `section` | `default` | Part breaks |
| `two-column` | `default`, `left`, `right` | Comparisons |
| `three-column` | `default`, `left`, `center`, `right` | Process flows |
| `code-focus` | `default` | Code walkthroughs |
| `big-stat` | `default`, `caption`, `figure` | Key metrics |
| `quote` | `default`, `attribution` | Testimonials |
| `cover` | `default` (uses `background` directive) | Hero images |
| `sidebar` | `default`, `sidebar` | Reference slides |
| `split-media` | `default`, `media` | Feature highlights |
| `blank` | `default` | Custom designs |

## Component Layouts

A `layout:` value that starts with `./` or `../` and ends in `.jsx`,
`.tsx`, `.js`, or `.ts` is a React component file next to the deck, which
renders the whole slide and may use any slot name:

```markdown
<!--
layout: ./slides/RollingDeploy.jsx
-->

# Zero-downtime deploys
```

See `skills/tap/rules/components.md`.
