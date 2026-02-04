---
title: Demo Day
---

# Demo Day

A product demo template perfect for showcasing features to stakeholders.

## Overview

This example demonstrates how to structure a product demo with:

- Problem/solution narrative structure
- Live feature demonstrations
- Metrics and impact slides
- Call to action and next steps

## Features Used

- **Theme**: `aurora` for modern, engaging visuals
- **Layouts**: `title`, `big-stat`, `split-media`, `section`
- **Live Code**: Shell driver for live CLI demos
- **Transitions**: `fade` for smooth flow between sections

## Source

````markdown
---
title: Introducing Tap
theme: aurora
author: Product Team
date: 2026-01-24
aspectRatio: "16:9"
transition: fade
drivers:
  shell:
    timeout: 30
---

# Introducing Tap

Developer presentations, reimagined

---

<!--
layout: section
-->

## The Problem

Death by PowerPoint

---

## Current Pain Points

<!-- pause -->

- **Context switching** — Leave your IDE to update slides

<!-- pause -->

- **Outdated screenshots** — Code examples get stale

<!-- pause -->

- **No interactivity** — Can't demo live code safely

<!-- pause -->

- **Version control nightmare** — Binary files don't diff

---

<!--
layout: big-stat
-->

## 73%

of developers avoid creating presentations because of tooling friction

---

<!--
layout: section
-->

## The Solution

Write slides in Markdown, run code live

---

## How It Works

<!-- pause -->

1. **Write Markdown** — Use your favorite editor

<!-- pause -->

2. **Add frontmatter** — Configure themes and drivers

<!-- pause -->

3. **Run `tap dev`** — Hot reload as you edit

<!-- pause -->

4. **Present** — Execute code blocks live

---

<!--
layout: two-column
-->

## Before & After

|||

### Before (PowerPoint)

- Export code as images
- Manually update screenshots
- Hope nothing changed
- Pray the demo gods are kind

|||

### After (Tap)

- Code lives in markdown
- Always up to date
- Execute live on stage
- Git-friendly diffs

---

## Live Demo

```bash {driver: shell}
echo "Hello from Tap!"
date
whoami
```

---

<!--
layout: big-stat
-->

## 10x

faster to create and maintain technical presentations

---

## Customer Feedback

<!-- pause -->

> "Finally, a presentation tool that thinks like a developer."
> — Senior Engineer, Acme Corp

<!-- pause -->

> "Our team's technical presentations went from dreaded to delightful."
> — Engineering Manager, StartupCo

---

<!--
layout: section
-->

## Next Steps

---

## Getting Started

<!-- pause -->

1. **Install**: `go install github.com/MiniCodeMonkey/tap@latest`

<!-- pause -->

2. **Create**: `tap new my-talk`

<!-- pause -->

3. **Develop**: `tap dev my-talk.md`

<!-- pause -->

4. **Present**: Share the URL or build static files

---

<!--
layout: title
-->

# Ready to Try?

tap.sh
````

---

::: tip
Demo Day presentations work best when you lead with the problem, show the solution in action, and end with clear next steps.
:::
