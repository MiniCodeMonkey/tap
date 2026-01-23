---
title: Themes
---

# Themes

Themes control the visual appearance of your presentation, including typography, colors, animations, and transitions. Tap comes with five built-in themes designed for different presentation styles.

## Setting a Theme

Set the theme in your presentation's frontmatter:

```yaml
---
theme: minimal
---
```

The theme applies to all slides in your presentation.

## Built-in Themes

### Minimal

Clean and spacious with ample whitespace. Uses a neutral color palette with subtle accents.

```yaml
---
theme: minimal
---
```

**Best for:** Professional presentations, corporate settings, content-heavy slides where readability is paramount.

**Characteristics:**
- Light background with dark text
- Sans-serif typography
- Subtle animations
- High contrast for accessibility

### Gradient

Modern and colorful with smooth gradient backgrounds. Eye-catching without being distracting.

```yaml
---
theme: gradient
---
```

**Best for:** Product launches, marketing presentations, creative projects, startup pitches.

**Characteristics:**
- Dynamic gradient backgrounds
- Bold, modern typography
- Smooth color transitions
- Vibrant accent colors

### Terminal

Hacker aesthetic with a dark terminal-inspired design. Monospace fonts and green-on-black styling.

```yaml
---
theme: terminal
---
```

**Best for:** Technical talks, developer conferences, security presentations, demos with code.

**Characteristics:**
- Dark background (near black)
- Monospace typography throughout
- Green/amber accent colors
- Minimal animations
- Retro terminal feel

### Brutalist

Bold and geometric with strong visual impact. High contrast and unconventional layouts.

```yaml
---
theme: brutalist
---
```

**Best for:** Design talks, creative showcases, presentations that want to stand out, artistic projects.

**Characteristics:**
- High contrast black and white
- Bold, heavy typography
- Sharp edges, no rounded corners
- Stark, impactful visual style
- Unconventional spacing

### Keynote

Professional and polished with a classic presentation feel. Familiar and comfortable for audiences.

```yaml
---
theme: keynote
---
```

**Best for:** Business presentations, executive briefings, educational content, formal settings.

**Characteristics:**
- Clean, professional aesthetic
- Balanced typography hierarchy
- Subtle gradients and shadows
- Smooth, elegant transitions
- Traditional slide feel

## What Themes Control

Each theme defines:

| Aspect | Description |
|--------|-------------|
| **Typography** | Font families, sizes, weights, and line heights for headings, body text, and code |
| **Colors** | Background colors, text colors, accent colors, and syntax highlighting palette |
| **Animations** | How elements appear on slides (fade, slide, bounce, etc.) |
| **Transitions** | How slides transition between each other (fade, push, slide, zoom) |
| **Spacing** | Padding, margins, and overall layout density |
| **Code styling** | Code block appearance, syntax highlighting colors, and font sizing |

## Theme Reference Table

| Theme | Vibe | Background | Typography |
|-------|------|------------|------------|
| `minimal` | Clean, spacious | Light (#ffffff) | Sans-serif |
| `gradient` | Modern, colorful | Gradient | Bold sans-serif |
| `terminal` | Hacker aesthetic | Dark (#0a0a0a) | Monospace |
| `brutalist` | Bold, geometric | High contrast | Heavy sans-serif |
| `keynote` | Professional, polished | Light/subtle | Classic sans-serif |

## Customizing Themes

::: tip Coming Soon
Custom theme support is planned for a future release. You'll be able to:

- Override theme variables (colors, fonts, spacing)
- Create entirely custom themes
- Share themes with the community

For now, choose the built-in theme that best matches your presentation style.
:::

## Next Steps

- Learn about [Animations & Transitions](/guide/animations-transitions) to fine-tune slide effects
- Explore [Layouts](/guide/layouts) to structure your content
- See [Code Blocks](/guide/code-blocks) for syntax highlighting options
