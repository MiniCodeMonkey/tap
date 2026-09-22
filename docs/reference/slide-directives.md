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
| `two-column` | Two equal columns (`::left` and `::right` slots) |
| `three-column` | Three columns (`::left`, `::center`, and `::right` slots) |
| `code-focus` | Maximized code display |
| `big-stat` | Prominent statistic or number |
| `quote` | Styled quotation |
| `cover` | Full-screen background image |
| `sidebar` | Main content with sidebar (`::sidebar` slot) |
| `split-media` | Media and content side by side (`::media` slot) |
| `blank` | No default styling |

See [Layouts Reference](/reference/layouts-reference) for detailed specifications.

A slide with no `layout:` directive gets one picked from its content: two
or three column slots, a lone `h1`, a lone `h2`, a dominant code block, or
a blockquote each select a layout. See
[Automatic Layout Selection](/reference/layouts-reference#automatic-layout-selection).

**Component layouts:** a value that starts with `./` or `../` and ends in
`.jsx`, `.tsx`, `.js`, or `.ts` names a React component file next to the
deck, which renders the whole slide:

```markdown
<!--
layout: ./slides/RollingDeploy.jsx
-->
```

See [Components Reference](/reference/components-reference).

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

::right

### Option B
- More features
- Better scaling
```

Content before the first `::slot` marker (like the `### Option A` section above) is the default slot. A slot marker line such as `::right` starts a new named slot that runs to the next marker or the end of the slide.

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
| Default | `false` |

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

Hex colors do not need quotes. `background: #1a1a2e` and `background: "#1a1a2e"` both work: the parser quotes a bare `#rgb`, `#rgba`, `#rrggbb`, or `#rrggbbaa` value before handing the directive block to YAML, since an unquoted `#` would otherwise start a YAML comment and silently empty the value.

Only a genuine hex color is protected this way. A value such as `tag: #scaling` is not 3, 4, 6, or 8 hex digits, so it is still read as a YAML comment and still needs quotes: `tag: "#scaling"`.

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

#### Alternative: Notes Comment Anywhere in the Slide

You can also write notes as their own HTML comment anywhere in the slide, most commonly after the content:

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

A comment whose content starts with `notes:` (after optional whitespace) is a notes comment. It is removed from the rendered slide, so it never reaches the HTML, and its text (blank lines trimmed from the start and end, inner line breaks kept) becomes the slide's notes. This works even when the text after `notes:` is not valid YAML, for example when it contains a colon or starts with a quote. A notes comment inside a fenced code block is left alone as code, and one inside a named `::slot` section is still removed and still counts toward the slide's notes.

If a slide has both a `notes:` directive in its first comment block and one or more of these standalone notes comments, they are joined with a blank line, directive notes first. Several standalone notes comments on one slide are joined in document order, each separated by a blank line. A `<!-- pause -->` comment is never treated as notes.

A comment ends at the first `-->` it contains, the same rule HTML (and the goldmark renderer tap uses) follows: comments do not nest. Notes text can therefore not contain the sequence `-->`; anything after the first `-->` closes the comment and is read as ordinary slide content instead of notes.

The first directive comment can also mix real directives with free-text notes, in either order, even when the notes text is not valid YAML on its own:

```markdown
<!--
layout: big-stat
notes: Start time is 10am, don't run over.
-->
```

A line that opens with a known directive key (`layout`, `transition`, `background`, `tag`, `badge`, `fragments`, `scroll`, `scroll-speed`, `steps`) is read as that directive; the `notes:` line and everything after it up to the next such line is read as notes text. Inside that notes text, a line only counts as a directive if its value is plausible for that key: `layout`, `transition`, `fragments`, `scroll`, `scroll-speed`, and `steps` need a single word with no spaces, so a sentence like `layout: keep it simple, that's the message.` stays part of the notes instead of being read as a layout.

A notes line that begins with `background:`, `tag:`, or `badge:` is always read as a directive, since those keys take free-form values; the [trailing notes comment form](#alternative-notes-comment-anywhere-in-the-slide) avoids this question entirely and is the recommended form for long free-text notes.

---

### tag

A short decorative metadata label rendered on the slide, for example a
chapter marker.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | None |
| Overrides | None |

```markdown
<!--
tag: "01 / Kickoff"
-->
```

Keep a tag word-shaped. Some themes render it at poster size, where a
string of punctuation reads badly.

---

### badge

A short decorative badge rendered on the slide, for example a version
number.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | None |
| Overrides | None |

```markdown
<!--
badge: "v2.0"
-->
```

---

### scroll

Turns a long slide into a scroll reveal: the content scrolls up as you
advance instead of overflowing the slide.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |
| Overrides | None |

```markdown
<!--
scroll: true
-->
```

---

### scroll-speed

The scroll reveal's duration in milliseconds. Only meaningful together
with `scroll: true`.

| Property | Value |
|----------|-------|
| Type | `integer` (milliseconds) |
| Default | `2000` |
| Overrides | None |

```markdown
<!--
scroll: true
scroll-speed: 4000
-->
```

---

### steps

How many clicker presses this slide consumes before advancing. Used by
step-driven content: a deck component, or a `map` fence.

| Property | Value |
|----------|-------|
| Type | `integer` |
| Default | The component's `export const steps`, or `1` for a `map` fence, otherwise `0` |
| Overrides | A component's own `export const steps` |

```markdown
<!--
layout: ./map-slide.jsx
steps: 4
-->
```

The directive always wins, including `steps: 0`, which pins a slide to a
single press no matter what its components declare.

::: warning Do not set both
The directive overrides a component's `export const steps` **silently**, so
a component whose step count later changes keeps consuming the directive's
number, and the extra presses do nothing. Prefer the export, which keeps
the count next to the code that defines it, and use this directive only
when the slide has no component export to read (a `map` fence, or a
component you cannot edit). See
[Components Reference](/reference/components-reference#steps).
:::

---

### skip

Leaves the slide out of the talk without deleting it.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |

```markdown
<!-- skip: true -->

# A slide for the long version of this talk
```

A skipped slide is left out of presenting: the arrow keys pass over it in
the audience view and the presenter view, and slide numbers, the progress
bar and the presenter's counter leave it out. `tap build` and
`tap export pdf` leave it out of their output, and `tap export images --all`
writes no image for it.

It keeps its place and its number in the deck, so `#4` in the URL and
`tap slide list` still count it. In `tap dev`, opening it directly (with the
URL, or from the overview) shows it with a "Skipped" marker, so you can
still write and check it. `tap export images --slide 4` renders it too,
because you asked for it by number.

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

::right

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
| `tag` | string | None | Decorative metadata label |
| `badge` | string | None | Decorative badge |
| `scroll` | boolean | `false` | Scroll reveal for long content |
| `scroll-speed` | integer | `2000` | Scroll reveal duration, in milliseconds |
| `steps` | integer | Auto-detected | Clicker presses this slide consumes |
| `skip` | boolean | `false` | Leave the slide out of presenting and exports |

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
