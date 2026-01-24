---
title: Slide Directives
---

# Slide Directives

Complete reference for per-slide configuration using directive blocks.

## Overview

Slide directives let you override global presentation settings for individual slides. They're placed at the start of a slide using YAML inside an HTML comment block.

```markdown
---

<!--
layout: two-column
transition: fade
fragments: true
-->

# My Slide Title

Slide content here...
```

Directives apply only to the slide where they appear, allowing precise control over each slide's behavior and appearance.

## Directive Block Syntax

### Basic Format

Directive blocks use HTML comment syntax with YAML inside:

```markdown
<!--
directive: value
another: value
-->
```

### Placement Rules

- Place directive blocks **immediately after** the slide separator (`---`)
- Put a blank line between the `---` and the directive block for readability
- The directive block must come **before** any slide content

```markdown
---

<!--
layout: title
-->

# Slide Title Comes After Directives
```

### Multi-line Values

For longer values, use YAML's pipe syntax for multi-line strings:

```markdown
<!--
notes: |
  First point to remember.
  Second important detail.
  Don't forget the demo!
-->
```

## Available Directives

### layout

Sets the slide layout, controlling how content is arranged.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `default` |
| Overrides | None (layouts are per-slide only) |

```markdown
<!--
layout: two-column
-->
```

**Available layouts:**

| Layout | Description |
|--------|-------------|
| `default` | Standard centered content |
| `title` | Large centered title slide |
| `section` | Section divider |
| `two-column` | Two equal columns (separated by `|||`) |
| `three-column` | Three columns (separated by `|||`) |
| `code-focus` | Maximized code display |
| `big-stat` | Prominent statistic or number |
| `quote` | Styled quotation |
| `cover` | Full-screen background image |
| `sidebar` | Main content with sidebar (separated by `|||`) |
| `split-media` | Media and content side by side (separated by `|||`) |
| `blank` | No default styling |

See [Layouts Reference](/reference/layouts-reference) for detailed specifications.

#### Example: Two-Column Layout

```markdown
---

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

---

### transition

Sets the animation when transitioning to this slide.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | Inherited from frontmatter |
| Overrides | `transition` in frontmatter |

```markdown
<!--
transition: zoom
-->
```

**Available transitions:**

| Transition | Effect |
|------------|--------|
| `none` | Instant switch, no animation |
| `fade` | Crossfade between slides |
| `slide` | Slide horizontally (left to right) |
| `push` | New slide pushes old slide out |
| `zoom` | Zoom in/out effect |

#### Example: Dramatic Reveal

```markdown
---
title: Product Launch
transition: fade
---

# Introduction

Standard fade transition for most slides.

---

<!--
transition: zoom
-->

# The Big Reveal

This slide zooms in for dramatic effect!

---

# Back to Normal

Returns to the default fade transition.
```

---

### fragments

Controls incremental reveals for list items on this slide.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | Inherited from frontmatter (default: `false`) |
| Overrides | `fragments` in frontmatter |

```markdown
<!--
fragments: true
-->
```

When enabled, bullet points appear one at a time as you advance.

#### Example: Enable Fragments for One Slide

```markdown
---
title: My Talk
fragments: false
---

# All at Once

- Item one
- Item two
- Item three

All items appear immediately.

---

<!--
fragments: true
-->

# Build Up

- First point
- Second point
- Third point

Items appear one by one.
```

#### Example: Disable Fragments for One Slide

```markdown
---
title: My Talk
fragments: true
---

# Step by Step

- Item one
- Item two

Items appear one at a time.

---

<!--
fragments: false
-->

# Reference Slide

- All items
- Appear together

This slide shows all content at once.
```

---

### background

Sets a background color or image for the slide.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | Theme default |
| Overrides | None |

```markdown
<!--
background: #1a1a2e
-->
```

**Accepted values:**

| Type | Example |
|------|---------|
| Hex color | `#1a1a2e` |
| RGB | `rgb(26, 26, 46)` |
| Color name | `navy` |
| Image path | `./images/bg.jpg` |
| URL | `https://example.com/image.jpg` |
| Gradient | `linear-gradient(135deg, #667eea 0%, #764ba2 100%)` |

#### Example: Colored Background

```markdown
---

<!--
background: #16213e
-->

# Dark Section

Content with a dark blue background.
```

#### Example: Image Background

```markdown
---

<!--
layout: cover
background: ./images/hero.jpg
-->

# Hero Title

Text overlaid on background image.
```

::: tip
For image backgrounds, use the `cover` layout for best results. It handles text overlay styling and ensures proper contrast.
:::

#### Example: Gradient Background

```markdown
---

<!--
background: linear-gradient(135deg, #667eea 0%, #764ba2 100%)
-->

# Gradient Slide

Modern gradient effect.
```

---

### notes

Adds speaker notes visible only in presenter mode.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | None |
| Overrides | None |

```markdown
<!--
notes: Remember to mention the demo here.
-->
```

**Multi-line notes:**

```markdown
<!--
notes: |
  Key talking points:
  - Mention the performance improvements
  - Show the before/after comparison
  - Leave time for questions
-->
```

Speaker notes appear in the presenter view (`/presenter`) alongside the current slide, next slide preview, and timer.

#### Example: Notes with Layout

```markdown
---

<!--
layout: big-stat
notes: |
  Pause here for effect.
  Let the number sink in.
  Then explain: this represents a 3x improvement over last quarter.
-->

# 3.2x

Faster than our previous release
```

#### Alternative: Inline Notes Syntax

You can also add notes at the end of a slide using inline comment syntax:

```markdown
---

# Key Results

Revenue increased 25% this quarter.

<!-- notes:
- Highlight international expansion
- Mention new product line
- Prepare for margin questions
-->
```

Both syntaxes are equivalent; choose the one that fits your workflow.

---

### class

Adds custom CSS classes to the slide for styling.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | None |
| Overrides | None |

```markdown
<!--
class: my-custom-slide highlight
-->
```

Multiple classes can be space-separated. Use this with custom CSS in your theme to create specialized slide styles.

#### Example: Custom Styling

```markdown
<!--
class: emphasis-slide
-->

# Important Point

This slide has custom styling applied.
```

In your theme's CSS:

```css
.emphasis-slide {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.emphasis-slide h1 {
  font-size: 4rem;
  text-shadow: 2px 2px 4px rgba(0, 0, 0, 0.3);
}
```

---

## Combining Directives

Use multiple directives together in a single block:

```markdown
---

<!--
layout: two-column
transition: slide
fragments: true
background: #1a1a2e
notes: |
  Compare the two approaches.
  Highlight that Option B scales better.
-->

# Architecture Options

### Monolith
- Single deployment
- Simpler to start
- Tighter coupling

|||

### Microservices
- Independent scaling
- Technology flexibility
- Operational complexity
```

## Quick Reference

| Directive | Type | Default | Description |
|-----------|------|---------|-------------|
| `layout` | string | `default` | Slide layout |
| `transition` | string | From frontmatter | Transition animation |
| `fragments` | boolean | From frontmatter | Incremental list reveals |
| `background` | string | Theme default | Background color/image |
| `notes` | string | None | Speaker notes |
| `class` | string | None | Custom CSS classes |

## Directive vs. Frontmatter

| Setting | Frontmatter | Directive |
|---------|-------------|-----------|
| Scope | All slides | Single slide |
| Location | Top of file | Start of slide |
| Syntax | YAML between `---` | YAML in `<!-- -->` |
| Overrides | Theme defaults | Frontmatter settings |

**Example: Global vs. Per-Slide**

```markdown
---
title: My Talk
transition: fade
fragments: false
---

# Slide 1

Uses global settings: fade transition, no fragments.

---

<!--
transition: zoom
fragments: true
-->

# Slide 2

Overrides: zoom transition, fragments enabled.

---

# Slide 3

Back to global settings: fade transition, no fragments.
```

## Next Steps

- [Frontmatter Options](/reference/frontmatter-options) - Global presentation settings
- [Layouts Reference](/reference/layouts-reference) - Detailed layout specifications
- [Animations & Transitions](/guide/animations-transitions) - More on transitions and fragments
- [Presenter Mode](/guide/presenter-mode) - Using speaker notes effectively
