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
Options: `default`, `title`, `section`, `two-column`, `three-column`, `code-focus`, `big-stat`, `quote`, `cover`, `sidebar`, `split-media`, `blank`.

A value starting with `./` or `../` and ending in `.jsx`, `.tsx`, `.js`, or
`.ts` is a deck component file instead, which renders the whole slide. See
`skills/tap/rules/components.md`.

With no `layout:` directive, tap picks one from the content: `three-column`
or `two-column` from the slot names, `title` for a lone `h1`, `section` for
a lone `h2`, `code-focus` for a dominant code block, `quote` for a
blockquote, otherwise `default`.

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

Hex colors do not need quotes: the parser quotes a bare `#rgb`, `#rgba`,
`#rrggbb`, or `#rrggbbaa` value before handing the block to YAML. Other
`#` values still need quotes, for example `tag: "#scaling"`.

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

### tag
A short decorative metadata label on the slide. Keep it word-shaped.
```markdown
<!--
tag: "01 / Kickoff"
-->
```

### badge
A short decorative badge on the slide.
```markdown
<!--
badge: "v2.0"
-->
```

### scroll / scroll-speed
Turn a long slide into a scroll reveal. `scroll-speed` is the duration in
milliseconds (default `2000`).
```markdown
<!--
scroll: true
scroll-speed: 4000
-->
```

### steps
How many clicker presses this slide consumes, for a deck component or a
`map` fence. The directive always wins, including `steps: 0`.
```markdown
<!--
steps: 4
-->
```
**Do not set this on a slide whose component exports `steps`.** The
directive overrides the export silently, so the two drift apart. Prefer the
export.

### skip
Leaves the slide out of the talk without deleting it.
```markdown
<!-- skip: true -->

# A slide for the long version of this talk
```
A skipped slide stays in the file and keeps its number, but presenting,
slide counts, `tap build` and `tap export` leave it out. `tap dev` still
shows it, marked, when you open it directly.

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

### Monolith
- Single deployment
- Simpler to start

::right

### Microservices
- Independent scaling
- Technology flexibility
```

## Notes Comment Anywhere (Recommended)

A comment whose content starts with `notes:` is a notes comment, wherever
it sits in the slide. It never reaches the rendered HTML, and its text
becomes the slide's notes with line breaks preserved. This form never
collides with directive parsing, so prefer it for long free text. Notes
render as plain text, not markdown. A comment ends at the first `-->`, so
notes cannot contain that sequence.
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
