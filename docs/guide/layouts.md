---
title: Layouts
---

# Layouts

Layouts control how content is arranged on your slides. Tap provides a variety of built-in layouts to suit different presentation needs.

`tap slide add --layout <name>` appends a slide in any of the 12 layouts below straight from the command line, no wizard needed; add `--print` to see its template without writing anything. See [CLI Commands](/reference/cli-commands#tap-slide-add).

## Automatic Layout Behavior

A slide with no `layout:` directive gets one picked from its content:

1. `three-column` when the slide has `left`, `center`, and `right` slots
2. `two-column` when it has `left` and `right` slots
3. `title` when it holds only an `h1`, with an optional short subtitle
4. `section` when it holds only an `h2`
5. `code-focus` when a single code block is more than half the content
6. `quote` when a blockquote is the main content
7. `default` for everything else

So this slide is a `title` slide without saying so:

```markdown
# My Slide Title

A one-line subtitle.
```

Set the directive when you want a layout the content would not produce on
its own, such as `big-stat`, `cover`, `sidebar`, or `split-media`.

## Specifying a Layout

To use a specific layout, add a directive block at the beginning of your slide using an HTML comment with YAML:

```markdown
---

<!--
layout: two-column
-->

# Side by Side

Content for the left column.

::right

Content for the right column.
```

The `layout` directive tells Tap which layout to apply to that slide. A
layout with named slots reads content before the first `::slot` marker as
the `default` slot; a marker line like `::right` starts a new slot that
runs to the next marker or the end of the slide. See
[Slide Directives](/reference/slide-directives) for the full marker syntax.

## Available Layouts

Tap includes 12 built-in layouts:

### default

Centers headings and body text vertically and horizontally. This is what
you get when you don't specify a `layout` directive at all, but you can
also name it explicitly.

```markdown
---

<!--
layout: default
-->

# My Slide Title
Body content, centered.
```

**When to use:** General-purpose slides that don't fit a more specific layout.

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

Split the slide into two equal columns. The default slot is a header spanning both columns; `::right` starts the right column.

```markdown
---

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

**When to use:** Comparisons, before/after, pros/cons.

### three-column

Split the slide into three equal columns. Use `::center` and `::right` to start the second and third columns.

```markdown
---

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

**When to use:** Process flows, multiple options, feature sets.

### code-focus

Optimized for showing code with maximum screen real estate. Reduces padding and uses the full slide area.

````markdown
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

````

**When to use:** Code walkthroughs, technical deep-dives.

### big-stat

Display a large statistic or number prominently. Add `::caption` for a supporting line and `::figure` for a small chart or image.

```markdown
---

<!--
layout: big-stat
-->

# 3.2x

::caption

Faster build times
```

**When to use:** Key metrics, impressive numbers, impact statements.

### quote

Stylized layout for quotations, with an `::attribution` slot for the source.

```markdown
---

<!--
layout: quote
-->

> The best code is no code at all.

::attribution

Jeff Atwood
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

Content with a sidebar area for notes, navigation, or supplementary information. Use `::sidebar` to start the sidebar content.

```markdown
---

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

**When to use:** Content with references, navigation-heavy slides.

### split-media

Split layout with media on one side and content on the other. Great for images with explanatory text. Use `::media` to start the media slot.

```markdown
---

<!--
layout: split-media
-->

# New Feature
Introducing our latest improvement that makes everything faster.

::media

![Product screenshot](./images/product.png)
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
| `default` | Centered heading and body | General-purpose slides |
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

## Component Layouts

A `layout:` value that starts with `./` or `../` and ends in `.jsx`,
`.tsx`, `.js`, or `.ts` is a React component file next to the deck, which
renders the whole slide and can use any slot name it likes:

```markdown
<!--
layout: ./slides/RollingDeploy.jsx
-->

# Zero-downtime deploys
```

See [Custom Components](/guide/custom-components).

## Next Steps

- Learn about [Themes](/guide/themes) to style your layouts
- Add [Animations & Transitions](/guide/animations-transitions) for polish
- See the [Layouts Reference](/reference/layouts-reference) for detailed specifications
- Build your own slide with [Custom Components](/guide/custom-components)
