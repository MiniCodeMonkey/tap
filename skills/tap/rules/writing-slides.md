# Writing Slides

## Frontmatter

Every presentation starts with YAML frontmatter:
```yaml
---
title: My Presentation
theme: paper
author: Jane Developer
date: 2024-01-15
---
```

## Slide Separators

Separate slides with `---` on its own line:
```markdown
---
title: My Talk
theme: paper
---

# Welcome

First slide content.

---

# Agenda

Second slide content.

---

# Deep Dive

Third slide content.
```

The first `---` block is frontmatter. Slides start after the closing frontmatter.

## Markdown Syntax

### Headings
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
```markdown
- First bullet point
- Second bullet point
  - Nested item

1. First numbered item
2. Second numbered item
```

### Links and Images
```markdown
[Link text](https://example.com)
![Alt text](./images/diagram.png)
```

### Code Blocks
````markdown
```javascript
function greet(name) {
  return `Hello, ${name}!`;
}
```
````

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
```

## Speaker Notes

### Inline Notes
Add at the end of a slide:
```markdown
# Quarterly Results

Revenue increased by 25%.

<!-- notes:
- Mention new product line
- Highlight international expansion
-->
```

### Notes in Directives
```markdown
<!--
layout: big-stat
notes: |
  Pause for effect here.
  Let the number sink in.
-->

# 1 Million

Active users reached
```

### Viewing Notes
Press `S` or navigate to `/presenter` to open presenter view with:
- Current slide
- Next slide preview
- Speaker notes
- Presentation timer
