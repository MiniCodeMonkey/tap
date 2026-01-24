# Slide Directives

Slide directives override global settings for individual slides using YAML inside HTML comments.

## Syntax

Place directive blocks immediately after the slide separator (`---`):
```markdown
---

<!--
layout: two-column
transition: fade
fragments: true
-->

# Slide Title

Content here...
```

## Available Directives

### layout
Sets the slide layout.
```markdown
<!--
layout: two-column
-->
```
Options: `default`, `title`, `section`, `two-column`, `three-column`, `code-focus`, `big-stat`, `quote`, `cover`, `sidebar`, `split-media`, `blank`

### transition
Animation when transitioning to this slide.
```markdown
<!--
transition: zoom
-->
```
Options: `none`, `fade`, `slide`, `push`, `zoom`

### fragments
Enable/disable incremental list reveals.
```markdown
<!--
fragments: true
-->
```

### background
Background color or image.
```markdown
<!--
background: #1a1a2e
-->
```

Accepted values:
- Hex color: `#1a1a2e`
- RGB: `rgb(26, 26, 46)`
- Color name: `navy`
- Image path: `./images/bg.jpg`
- Gradient: `linear-gradient(135deg, #667eea 0%, #764ba2 100%)`

### notes
Speaker notes visible in presenter mode.
```markdown
<!--
notes: Remember to mention the demo here.
-->
```

Multi-line notes:
```markdown
<!--
notes: |
  Key talking points:
  - Performance improvements
  - Before/after comparison
-->
```

### class
Custom CSS classes for the slide.
```markdown
<!--
class: my-custom-slide highlight
-->
```

## Combining Directives

Use multiple directives in one block:
```markdown
<!--
layout: two-column
transition: slide
fragments: true
background: #1a1a2e
notes: |
  Compare the two approaches.
  Highlight that Option B scales better.
-->

# Architecture Options

::left::
### Monolith
- Single deployment
- Simpler to start

::right::
### Microservices
- Independent scaling
- Technology flexibility
```

## Inline Notes Alternative

Add notes at the end of a slide:
```markdown
# Key Results

Revenue increased 25%.

<!-- notes:
- Highlight international expansion
- Prepare for margin questions
-->
```

## Directive vs Frontmatter

| Aspect | Frontmatter | Directive |
|--------|-------------|-----------|
| Scope | All slides | Single slide |
| Location | Top of file | Start of slide |
| Syntax | YAML between `---` | YAML in `<!-- -->` |
| Overrides | Theme defaults | Frontmatter settings |
