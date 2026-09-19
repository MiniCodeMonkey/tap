---
title: A Tour of Tap's Themes
theme: swiss
author: Tap Presentations
date: 2026-01-25
aspectRatio: "16:9"
transition: fade
---

<!-- This deck looks the same in every theme; only the frontmatter theme
     changes how it renders. While presenting, press "t" to cycle through
     all 21 themes live and see this exact content redrawn each time. -->

# A Tour of Tap's Themes

One deck, twenty-one looks. Press `t` to cycle.

---

<!-- layout: section -->

# Same Content, Different Room

---

## Why Themes Matter

A slide deck is read from the back of a room, not a desk.

- The room decides light or dark
- The talk decides the personality
- The content never has to change

---

<!--
layout: two-column
-->

## Two Ways to Pick

### By the room

- Bright room, weak projector: pick a light theme
- Dark room, good contrast: a dark theme can carry more

::right

### By the talk

- Infrastructure talk: `terminal`
- Launch day: `product` or `keynote`
- Scrappy lightning talk: `zine`

---

<!--
layout: big-stat
-->

# 21

::caption

Themes ship with Tap, all CSS-only, all switchable live

---

<!--
layout: quote
-->

> The best theme is the one nobody notices, because the content was clear.

::attribution

A conference organizer, probably

---

<!-- layout: code-focus -->

```bash {2}
# Try it yourself
tap dev examples/theme-tour.md
# then press "t" a few times
```

---

## Next Steps

- Read the pitch for each theme in `docs/guide/themes.md`
- Force one with `?theme=<slug>` in the URL
- Scaffold a new deck already themed: `tap new --theme terminal`
