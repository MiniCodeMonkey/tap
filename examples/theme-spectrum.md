---
title: Spectrum Theme Showcase
theme: spectrum
author: Tap Presentations
date: 2026-02-24
aspectRatio: "16:9"
transition: fade
---

<!--
layout: title
tag: "keynote"
badge: "2026"
-->

# Spectrum Theme

Gradient-forward modern design

---

## Design Philosophy

The Spectrum theme embodies:

- **Vibrant** — Indigo-to-pink gradient as signature accent
- **Modern** — SaaS-inspired card layouts and soft shadows
- **Rounded** — 14px radius throughout for approachable feel
- **Layered** — Depth via shadows and surface elevation

---

## Typography

<!-- pause -->

**Sora** geometric sans-serif throughout

<!-- pause -->

Modern variable font with excellent weight range:
- Display: Weight 800, ultra-tight tracking
- Body: Weight 400, generous line-height
- Mono: Fira Code with ligature support

---

## Code Blocks

```python
from dataclasses import dataclass
from typing import list

@dataclass
class Presentation:
    title: str
    slides: list[Slide]
    theme: str = "spectrum"

def render(pres: Presentation) -> str:
    return f"Rendering {len(pres.slides)} slides"
```

Rounded 14px corners with gradient filename badges.

---

<!--
layout: two-column
-->

## Two Columns

Balanced content presentation

|||

### Features
- Gradient text headings
- Card-style list items
- Smooth animations

|||

### Stack
- Svelte 5 frontend
- Go backend
- WebSocket sync

---

<!--
layout: quote
-->

> Design is not just what it looks like and feels like. Design is how it works.

— Steve Jobs

---

<!--
layout: section
-->

## Visual Identity

Where gradients meet functionality

---

## Lists and Tables

Card-style list items with gradient badges:
- Real-time collaboration
- Theme customization
- Markdown authoring
- PDF export

| Gradient | Start | End |
|----------|-------|-----|
| Primary | Indigo | Pink |
| Cool | Blue | Purple |
| Warm | Orange | Rose |

---

<!--
layout: big-stat
-->

## 60fps

smooth gradient animations

---

<!--
layout: title
tag: "thank you"
-->

# Spectrum Theme

A full spectrum of possibilities
