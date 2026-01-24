# Animations & Transitions

Add motion and polish with transitions between slides and incremental reveals within slides.

## Slide Transitions

### Available Transitions

| Transition | Effect |
|------------|--------|
| `none` | Instant switch |
| `fade` | Crossfade between slides |
| `slide` | Slide horizontally |
| `push` | New slide pushes old out |
| `zoom` | Zoom in/out effect |

### Global Transition
Set in frontmatter:
```yaml
---
title: My Presentation
transition: fade
---
```

### Per-Slide Transition
Override with directive:
```markdown
---

<!--
transition: zoom
-->

# The Big Reveal

This slide zooms in for dramatic effect!
```

### Best Practices
- Use `fade` for most presentations—smooth and professional
- Use `none` for rapid-fire slides
- Use `zoom` sparingly for emphasis
- Keep transitions consistent within sections

## Fragments (Incremental Reveals)

### The Pause Directive
Create manual breakpoints with `<!-- pause -->`:
```markdown
# Three Key Points

First, we'll discuss the problem.

<!-- pause -->

Second, we'll explore solutions.

<!-- pause -->

Third, we'll choose the best approach.
```

Press Space or Right Arrow to reveal each section.

### Automatic List Fragments
Enable auto-reveal for bullet lists:

**Globally:**
```yaml
---
fragments: true
---
```

**Per-Slide:**
```markdown
<!--
fragments: true
-->

# Features

- Easy to use
- Fast performance
- Beautiful output
```

Each bullet appears one at a time.

### Combining Pause and Fragments
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

Order:
1. Heading appears
2. Each bullet appears one by one
3. Final text appears after another advance

### Disabling Fragments for One Slide
```markdown
<!--
fragments: false
-->

# Reference Slide

- Item one
- Item two
- Item three

All items appear immediately.
```

## Quick Reference

| Feature | Syntax | Scope |
|---------|--------|-------|
| Global transition | `transition: fade` in frontmatter | All slides |
| Per-slide transition | `transition: zoom` in directive | Single slide |
| Manual pause | `<!-- pause -->` | Single slide |
| Global fragments | `fragments: true` in frontmatter | All slides |
| Per-slide fragments | `fragments: true` in directive | Single slide |
