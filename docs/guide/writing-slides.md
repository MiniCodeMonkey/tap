---
title: Writing Slides
---

# Writing Slides

Learn the fundamentals of authoring presentations in Tap. This guide covers frontmatter configuration, slide structure, markdown syntax, local directives, and speaker notes.

## Frontmatter Basics

Every Tap presentation begins with YAML frontmatter enclosed in triple dashes. This defines global settings that apply to your entire presentation.

```yaml
---
title: My Presentation
theme: paper
author: Jane Developer
date: 2024-01-15
---
```

### Common Frontmatter Options

| Option | Type | Description |
|--------|------|-------------|
| `title` | string | The presentation title (shown in browser tab) |
| `theme` | string | Visual theme: `paper`, `noir`, `aurora`, `phosphor`, `poster` |
| `author` | string | Author name for metadata |
| `date` | string | Presentation date |
| `aspectRatio` | string | Slide aspect ratio (default: `16/9`) |
| `transition` | string | Default slide transition: `none`, `fade`, `slide`, `push`, `zoom` |

See [Frontmatter Options](/reference/frontmatter-options) for the complete reference.

## Slide Separators

Separate individual slides using three dashes (`---`) on their own line. Leave blank lines before and after the separator for clarity:

```markdown
---
title: My Talk
theme: paper
---

# Welcome

This is the first slide.

---

# Agenda

This is the second slide.

---

# Deep Dive

This is the third slide.
```

::: tip
The first `---` block is frontmatter, not a slide separator. Your first slide content comes after the closing frontmatter dashes.
:::

## Markdown Syntax

Tap supports standard markdown syntax with some presentation-focused enhancements.

### Headings

Use headings to structure your slides:

```markdown
# Main Title (H1)

## Section Title (H2)

### Subsection (H3)
```

### Text Formatting

```markdown
**Bold text** for emphasis

*Italic text* for subtle emphasis

`inline code` for technical terms

~~Strikethrough~~ for corrections
```

### Lists

Both ordered and unordered lists are supported:

```markdown
- First bullet point
- Second bullet point
  - Nested item
  - Another nested item
- Third bullet point

1. First numbered item
2. Second numbered item
3. Third numbered item
```

### Links and Images

```markdown
[Link text](https://example.com)

![Alt text](./images/diagram.png)
```

See [Images & Media](/guide/images-media) for advanced image options.

### Code Blocks

Fenced code blocks with syntax highlighting:

````markdown
```javascript
function greet(name) {
  return `Hello, ${name}!`;
}
```
````

See [Code Blocks](/guide/code-blocks) for line highlighting, diffs, and multi-step reveals.

### Blockquotes

```markdown
> "The best way to predict the future is to invent it."
> — Alan Kay
```

### Tables

```markdown
| Feature | Supported |
|---------|-----------|
| Markdown | Yes |
| Live Code | Yes |
| Themes | Yes |
```

## Local Directives

Override global settings for individual slides using local directives. These are YAML blocks inside HTML comments, placed at the start of a slide:

```markdown
---

<!--
layout: two-column
transition: fade
-->

# This Slide Uses Two Columns

Content here will use the two-column layout with a fade transition.
```

### Available Directives

| Directive | Description |
|-----------|-------------|
| `layout` | Slide layout (e.g., `title`, `two-column`, `code-focus`) |
| `transition` | Transition for this slide (overrides global) |
| `background` | Background color or image |
| `fragments` | Enable incremental reveals (`true`/`false`) |
| `notes` | Speaker notes (alternative to `<!-- notes: -->` syntax) |

See [Slide Directives](/reference/slide-directives) for the complete reference.

### Example: Mixed Layouts

````markdown
---
title: Product Launch
theme: aurora
---

<!--
layout: title
-->

# Introducing Our Product

The future of developer tools

---

<!--
layout: two-column
-->

# Key Features

- Fast and efficient
- Easy to use
- Well documented

|||

- Open source
- Cross-platform
- Extensible

---

<!--
layout: code-focus
-->

# Quick Start

```bash
npm install awesome-tool
```

---

<!--
layout: big-stat
-->

# 10x

Faster than the competition
````

## Speaker Notes

Add notes visible only in presenter mode using the special `notes` syntax. These won't appear on your slides but will be visible when you're presenting.

### Inline Notes

Add notes at the end of a slide using an HTML comment:

```markdown
---

# Quarterly Results

Revenue increased by 25% this quarter.

<!-- notes:
- Mention the new product line contribution
- Highlight international expansion
- Q&A: Be prepared for margin questions
-->
```

### Notes in Directives

Alternatively, include notes in your slide directive block:

```markdown
---

<!--
layout: big-stat
notes: |
  Pause for effect here.
  Let the number sink in before continuing.
-->

# 1 Million

Active users reached this milestone
```

### Viewing Notes

Speaker notes appear in presenter mode. Start your presentation and press `S` or navigate to `/presenter` to open the presenter view with:

- Current slide
- Next slide preview
- Speaker notes
- Presentation timer

See [Presenter Mode](/guide/presenter-mode) for more details.

## Best Practices

### Keep Slides Focused

Each slide should convey one main idea. If you find yourself cramming content, split it into multiple slides.

### Use Consistent Structure

Maintain a predictable rhythm:
- Title slide
- Agenda/overview
- Content sections
- Summary/call to action

### Leverage Layouts

Don't default to plain slides. Use layouts like `two-column`, `quote`, and `big-stat` to add visual variety.

### Write Notes Liberally

Even if you know your content well, speaker notes help you:
- Stay on track during nerves
- Remember specific data points
- Hand off presentations to colleagues

## Next Steps

- [Layouts](/guide/layouts) - Explore all available slide layouts
- [Themes](/guide/themes) - Customize your presentation's appearance
- [Animations & Transitions](/guide/animations-transitions) - Add motion to your slides
- [Code Blocks](/guide/code-blocks) - Advanced code presentation features
