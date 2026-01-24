---
title: Getting Started with Tap
theme: paper
author: Your Name
date: 2026-01-24
aspectRatio: "16:9"
transition: fade
---

# Getting Started with Tap

A simple presentation to show you the basics

---

## What is Tap?

Tap is a markdown-based presentation tool designed for developers.

- Write slides in markdown
- Beautiful themes out of the box
- Live code execution
- Hot reload during development

---

## Slide Basics

Slides are separated by `---` horizontal rules.

Each slide can contain:

- **Headers** for titles
- Regular paragraphs
- Lists like this one
- Code blocks
- Images
- And more!

---

## Code Blocks

You can include syntax-highlighted code:

```javascript
function greet(name) {
  return `Hello, ${name}!`;
}

console.log(greet('World'));
```

---

## Using Fragments

<!-- pause -->

Fragments let you reveal content incrementally.

<!-- pause -->

Each `<!-- pause -->` marker creates a new fragment.

<!-- pause -->

Press Space or Enter to reveal the next fragment.

---

<!--
layout: two-column
-->

## Two Column Layout

Use the two-column layout for side-by-side content.

|||

- Left column content
- Lists work great here
- Easy to compare

|||

- Right column content
- Perfect for comparisons
- Or code + explanation

---

<!--
layout: quote
-->

> The best way to predict the future is to invent it.

Alan Kay

---

## Images

You can include images with standard markdown:

![Placeholder](https://via.placeholder.com/400x200)

Images can be sized with `{width=50%}` attributes.

---

<!--
layout: section
-->

## Section Breaks

Use section layouts for chapter headers

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Arrow Right/Down | Next slide |
| Arrow Left/Up | Previous slide |
| Space/Enter | Next fragment or slide |
| Home | First slide |
| End | Last slide |
| O | Toggle overview |
| S | Open presenter view |
| F | Toggle fullscreen |

---

<!--
layout: title
-->

# Thank You!

Start creating your own presentations with `tap new`
