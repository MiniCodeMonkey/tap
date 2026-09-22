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
- Use `fade` for most presentations, it's smooth and professional
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

Each `<!-- pause -->` marker is one reveal: the slide above takes two
presses before it advances to the next slide. The content before the first
marker is visible as soon as the slide appears.

### Automatic List Fragments

Enable automatic fragments for a slide's bullet lists by setting
`fragments: true` in that slide's directive block. Fragments are set per
slide; there is no deck-wide frontmatter equivalent.

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

You can use both techniques on the same slide:

```markdown
<!--
fragments: true
-->

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

## Steps

A deck component or a `map` fence consumes clicker presses of its own,
called steps. They behave like fragments from the audience's side: an
advance key moves through the slide's steps before moving on. Set the
count with the `steps:` directive, or let a component declare it with
`export const steps = N`. See
[Custom Components](/guide/custom-components) and
[Map Animations](/guide/map-animations).

## Print Mode

`?print=true`, PDF export, and every thumbnail render a slide in its final
state: every fragment revealed, every step at its last value, and no
animation. A deck component must do the same; see
[Print mode and thumbnails](/guide/custom-components#print-mode-and-thumbnails).

## Animation Timing

Themes control animation timing and easing, each to match its own
personality: `terminal` favors instant, blunt transitions, `keynote` favors
slower, more deliberate ones, `sketch` favors slightly playful timing, and
so on. See [Themes](/guide/themes) for the full list.

## Quick Reference

| Feature | Syntax | Scope |
|---------|--------|-------|
| Global transition | `transition: fade` in frontmatter | All slides |
| Per-slide transition | `transition: zoom` in directive | Single slide |
| Manual pause | `<!-- pause -->` | Single slide |
| Fragments | `fragments: true` in directive | Single slide |
| Steps for a component or map | `steps: 4` in directive | Single slide |
| Load at a fragment | `?fragment=2` in the URL | Single load |
| Load at a step | `?step=3` in the URL | Single load |

## Next Steps

- Learn about [Themes](/guide/themes) which control animation styling
- Explore [Writing Slides](/guide/writing-slides) for more markdown features
- See [Presenter Mode](/guide/presenter-mode) to practice your timing
