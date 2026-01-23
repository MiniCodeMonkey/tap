---
title: Layouts
---

# Layouts

Layouts control how content is arranged on your slides. Tap provides a variety of built-in layouts to suit different presentation needs.

## Default Layout Behavior

If you don't specify a layout, Tap uses a default content layout that centers your content vertically and horizontally. This works well for most slides with headings and body text.

```markdown
# My Slide Title

This content uses the default layout.
It will be centered on the slide.
```

## Specifying a Layout

To use a specific layout, add a directive block at the beginning of your slide using an HTML comment with YAML:

```markdown
---

<!--
layout: two-column
-->

# Side by Side

::left::
Content for the left column.

::right::
Content for the right column.
```

The `layout` directive tells Tap which layout to apply to that slide.

## Available Layouts

Tap includes 11 built-in layouts:

### title

A full-screen title slide with large, centered text. Perfect for the opening slide or major section breaks.

```markdown
---

<!--
layout: title
-->

# Welcome to My Talk
## A subtitle goes here
```

**When to use:** Opening slides, major section introductions.

### section

A section divider slide with prominent heading. Similar to title but styled as an interstitial break.

```markdown
---

<!--
layout: section
-->

# Part 2: The Solution
```

**When to use:** Separating major parts of your presentation.

### two-column

Split the slide into two equal columns. Use `::left::` and `::right::` markers to place content.

```markdown
---

<!--
layout: two-column
-->

# Comparison

::left::
### Option A
- Fast execution
- Simple setup

::right::
### Option B
- More features
- Better scaling
```

**When to use:** Comparisons, before/after, pros/cons.

### three-column

Split the slide into three equal columns. Use `::left::`, `::center::`, and `::right::` markers.

```markdown
---

<!--
layout: three-column
-->

# Our Process

::left::
### Plan
Define requirements

::center::
### Build
Write the code

::right::
### Ship
Deploy to production
```

**When to use:** Process flows, multiple options, feature sets.

### code-focus

Optimized for showing code with maximum screen real estate. Reduces padding and uses the full slide area.

```markdown
---

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

**When to use:** Code walkthroughs, technical deep-dives.

### big-stat

Display a large statistic or number prominently. Perfect for impactful data points.

```markdown
---

<!--
layout: big-stat
-->

# 3.2x
## Faster build times
```

**When to use:** Key metrics, impressive numbers, impact statements.

### quote

Stylized layout for quotations with attribution support.

```markdown
---

<!--
layout: quote
-->

> The best code is no code at all.

— Jeff Atwood
```

**When to use:** Customer testimonials, famous quotes, key statements.

### cover

Full-screen background image with overlaid text. Specify the background image in the directive.

```markdown
---

<!--
layout: cover
background: ./images/hero.jpg
-->

# Bold Statement
## On a beautiful background
```

**When to use:** Hero images, dramatic statements, visual storytelling.

### sidebar

Content with a sidebar area for notes, navigation, or supplementary information.

```markdown
---

<!--
layout: sidebar
-->

::main::
# Main Content
The primary focus of this slide.

::sidebar::
**Related:**
- Topic A
- Topic B
```

**When to use:** Content with references, navigation-heavy slides.

### split-media

Split layout with media on one side and content on the other. Great for images with explanatory text.

```markdown
---

<!--
layout: split-media
-->

::media::
![Product screenshot](./images/product.png)

::content::
# New Feature
Introducing our latest improvement that makes everything faster.
```

**When to use:** Product demos, feature highlights, image explanations.

### blank

Completely empty layout with no default styling. Full creative control.

```markdown
---

<!--
layout: blank
-->

<div style="display: flex; justify-content: center; align-items: center; height: 100%;">
  Custom HTML content
</div>
```

**When to use:** Custom designs, complex layouts, embedded content.

## Layout Reference Table

| Layout | Description | Best For |
|--------|-------------|----------|
| `title` | Large centered text | Opening slides |
| `section` | Section divider | Part breaks |
| `two-column` | Two equal columns | Comparisons |
| `three-column` | Three equal columns | Process flows |
| `code-focus` | Maximum code space | Code walkthroughs |
| `big-stat` | Prominent number | Key metrics |
| `quote` | Styled quotation | Testimonials |
| `cover` | Background image | Hero images |
| `sidebar` | Content + sidebar | Reference slides |
| `split-media` | Media + content | Feature highlights |
| `blank` | No styling | Custom designs |

## Next Steps

- Learn about [Themes](/guide/themes) to style your layouts
- Add [Animations & Transitions](/guide/animations-transitions) for polish
- See the [Layouts Reference](/reference/layouts-reference) for detailed specifications
