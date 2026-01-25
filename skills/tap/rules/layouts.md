# Layouts

Layouts control how content is arranged on slides. Tap provides 11 built-in layouts.

## Specifying a Layout

Add a directive at the beginning of your slide:
```markdown
---

<!--
layout: two-column
-->

# Side by Side

Left column content.

|||

Right column content.
```

## Available Layouts

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
Two equal columns separated by `|||`.
```markdown
<!--
layout: two-column
-->

# Comparison

### Option A
- Fast execution
- Simple setup

|||

### Option B
- More features
- Better scaling
```
**Use for:** Comparisons, before/after, pros/cons.

### three-column
Three equal columns separated by `|||`.
```markdown
<!--
layout: three-column
-->

# Our Process

### Plan
Define requirements

|||

### Build
Write the code

|||

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
Large statistic or number prominently displayed.
```markdown
<!--
layout: big-stat
-->

# 3.2x
## Faster build times
```
**Use for:** Key metrics, impressive numbers, impact statements.

### quote
Stylized quotation with attribution.
```markdown
<!--
layout: quote
-->

> The best code is no code at all.

— Jeff Atwood
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
Content with a sidebar area separated by `|||`.
```markdown
<!--
layout: sidebar
-->

# Main Content
The primary focus of this slide.

|||

**Related:**
- Topic A
- Topic B
```
**Use for:** Content with references, navigation-heavy slides.

### split-media
Media and content side by side separated by `|||`.
```markdown
<!--
layout: split-media
-->

![Product screenshot](./images/product.png)

|||

# New Feature
Introducing our latest improvement.
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

| Layout | Separator | Best For |
|--------|-----------|----------|
| `title` | None | Opening slides |
| `section` | None | Part breaks |
| `two-column` | `|||` | Comparisons |
| `three-column` | `|||` (twice) | Process flows |
| `code-focus` | None | Code walkthroughs |
| `big-stat` | None | Key metrics |
| `quote` | None | Testimonials |
| `cover` | None (uses `background` directive) | Hero images |
| `sidebar` | `|||` | Reference slides |
| `split-media` | `|||` | Feature highlights |
| `blank` | None | Custom designs |
