---
title: Animations & Transitions
---

# Animations & Transitions

Add motion and polish to your slides with transitions between slides and incremental reveals within slides.

## Slide Transitions

Transitions control how slides animate when moving from one to the next. Tap supports five transition types.

### Available Transitions

| Transition | Effect |
|------------|--------|
| `none` | Instant switch, no animation |
| `fade` | Crossfade between slides |
| `slide` | Slide horizontally (left to right) |
| `push` | New slide pushes old slide out |
| `zoom` | Zoom in/out effect |

### Setting Transitions Globally

Set a default transition for all slides in your frontmatter:

```yaml
---
title: My Presentation
transition: fade
---
```

All slides will use this transition unless overridden.

### Setting Transitions Per-Slide

Override the global transition for a specific slide using a directive block:

```markdown
---
title: My Presentation
transition: fade
---

# Slide One

This uses the default fade transition.

---

<!--
transition: zoom
-->

# Slide Two

This slide zooms in for dramatic effect.

---

# Slide Three

Back to the default fade transition.
```

::: tip Best Practices
- Use `fade` for most presentations—it's smooth and professional
- Use `none` for rapid-fire slides or when you want instant switches
- Use `zoom` sparingly for emphasis on key slides
- Keep transitions consistent within sections for a polished feel
:::

## Fragments (Incremental Reveals)

Fragments let you reveal content step-by-step within a single slide, perfect for building up ideas or keeping your audience focused.

### The Pause Directive

Use `<!-- pause -->` to create manual breakpoints in your slide:

```markdown
# Three Key Points

First, we'll discuss the problem.

<!-- pause -->

Second, we'll explore solutions.

<!-- pause -->

Third, we'll choose the best approach.
```

Each `<!-- pause -->` creates a new fragment. Press the next key (Space or Right Arrow) to reveal each section.

### Automatic List Fragments

Enable automatic fragments for bullet lists by setting `fragments: true`:

#### Globally in Frontmatter

```yaml
---
title: My Presentation
fragments: true
---
```

#### Per-Slide in Directive Block

```markdown
<!--
fragments: true
-->

# Features

- Easy to use
- Fast performance
- Beautiful output
```

With `fragments: true`, each list item appears one at a time as you advance.

### Combining Pause and List Fragments

You can use both techniques in the same presentation:

```markdown
---
title: Product Launch
fragments: true
---

# Why Our Product?

Key benefits:

- Saves time
- Reduces costs
- Improves quality

<!-- pause -->

**Ready to get started?**
```

In this example:
1. The heading appears
2. Each bullet appears one by one (from `fragments: true`)
3. The final text appears after another advance (from `<!-- pause -->`)

### Disabling Fragments for a Slide

If you've enabled fragments globally but want a specific slide to show all content at once:

```markdown
<!--
fragments: false
-->

# Reference Slide

- Item one
- Item two
- Item three

All items appear immediately on this slide.
```

## Animation Timing

Themes control animation timing and easing. Each built-in theme has its own animation style:

| Theme | Animation Style |
|-------|-----------------|
| `minimal` | Subtle, quick fades |
| `gradient` | Smooth, flowing animations |
| `terminal` | Minimal/instant transitions |
| `brutalist` | Sharp, sudden reveals |
| `keynote` | Elegant, professional timing |

## Quick Reference

| Feature | Syntax | Scope |
|---------|--------|-------|
| Global transition | `transition: fade` in frontmatter | All slides |
| Per-slide transition | `transition: zoom` in directive | Single slide |
| Manual pause | `<!-- pause -->` | Single slide |
| Global fragments | `fragments: true` in frontmatter | All slides |
| Per-slide fragments | `fragments: true` in directive | Single slide |

## Next Steps

- Learn about [Themes](/guide/themes) which control animation styling
- Explore [Writing Slides](/guide/writing-slides) for more markdown features
- See [Presenter Mode](/guide/presenter-mode) to practice your timing
