---
name: tap
description: Create markdown presentations with live code execution
version: 2.0.0
metadata:
  author: tap-sh
  tags: presentations, slides, markdown, live-code
globs:
  - "**/*.md"
---

# Tap - Presentations for Developers

Tap is a CLI tool that transforms markdown files into beautiful, interactive presentations with live code execution. Decks are written mostly by LLMs: write plain markdown, keep slides short, and let a theme and layout do the visual work.

## When to Use This Skill

Use Tap when building:
- Technical presentations with code demos
- Database query demonstrations with live results
- CLI tool tutorials and walkthroughs
- Workshop materials with executable examples
- Conference talks with code walkthroughs

## Quick Start

```bash
tap new                            # interactive wizard: title, theme, filename (needs a terminal)
tap new --theme terminal -o my-talk.md   # same wizard, with those steps pre-filled
tap new --yes --title "My Talk" --theme terminal --output my-talk.md   # no wizard, scriptable
tap dev my-talk.md                 # start the dev server (live code execution works here)
tap build my-talk.md               # build a static site (no live code execution)
tap export pdf my-talk.md                 # export to PDF
tap export images my-talk.md --slide 4 --output check.png   # render one slide, exit 1 if it is broken
tap theme list                     # every built-in theme
tap theme show terminal --prompt   # style brief for an image model
```

With a terminal attached, `tap new` opens an interactive wizard. An
unattended agent should pass `--yes` instead (or rely on it automatically:
`tap new` behaves as `--yes` when standard input is not a terminal),
which writes the file straight from `--title`, `--theme`, and `--output`
with no wizard.

Check your own work by rendering it: `tap export images` exits with status 1
when a slide shows an error card, and `tap build` exits 1 on any component
build error. Neither needs a browser window.

## Basic Slide Structure

```markdown
---
title: My Presentation
theme: terminal
---

# First Slide

Content here.

---

# Second Slide

More content.
```

Slides are separated by `---` on its own line. Frontmatter (the first `---`-delimited YAML block) sets presentation-wide defaults: `title`, `theme`, `author`, `date`, `aspectRatio`, `transition`, `fragments`, `slideNumbers`. Per-slide directives (an HTML comment with YAML, placed right after the slide separator) override those for one slide. See `docs/reference/frontmatter-options.md` and `docs/reference/slide-directives.md` for the complete option lists.

Every slide renders on a fixed 1920px-wide canvas that scales as one unit to fit any screen, so a deck looks the same on every projector, laptop, or presenter panel; themes are tuned for the default 16:9 aspect ratio.

## Slots

A layout with more than one content area uses named slots. Everything before the first `::slotname` marker is the `default` slot; a marker line on its own starts a new slot that runs to the next marker or the end of the slide. There is no column separator character anymore (an older `|||` syntax is gone).

```markdown
<!-- layout: big-stat -->

# 3.2x

::caption

Faster than our previous release

::figure

![Quarterly trend](trend.png)
```

## The 12 Layouts

Pick a layout by what the slide needs to say, not for variety's sake.

| Layout | Slots | Use it for |
|--------|-------|------------|
| `default` | `default` | General-purpose slides that don't fit anything more specific |
| `title` | `default` | The opening slide, or a section's title slide |
| `section` | `default` | A divider between major parts of the talk |
| `two-column` | `default`, `left`, `right` | Comparisons, before/after, pros/cons |
| `three-column` | `default`, `left`, `center`, `right` | Process flows, three options, feature sets |
| `code-focus` | `default` | A code block that should fill the slide |
| `big-stat` | `default`, `caption`, `figure` | One number the audience should remember |
| `quote` | `default`, `attribution` | A testimonial or a quote worth pausing on |
| `cover` | `default` (+ `background:` directive) | A hero image behind a short line of text |
| `sidebar` | `default`, `sidebar` | Main content plus a short aside |
| `split-media` | `default`, `media` | A screenshot or diagram next to explanatory text |
| `blank` | `default` | Fully custom HTML, no default styling |

Set the layout with a directive:

```markdown
<!-- layout: two-column -->

## Comparison

### Option A
- Fast execution
- Simple setup

::right

### Option B
- More features
- Better scaling
```

## The 21 Themes

Every theme is CSS-only: pick one by frontmatter, no build step. `base` is the plain fallback used when no theme is set or a theme name doesn't exist. Pick a theme by two questions: **the room** (a bright room or a weak projector washes out dark themes, so favor a `light`-polarity one; a dark room can carry a `dark`-polarity theme's contrast) and **the talk** (match the pitch below to what you're actually presenting).

| Theme | Polarity | Pitch |
|-------|----------|-------|
| `base` | light | A plain, readable default theme with no strong identity. |
| `terminal` | dark | Infrastructure and live-coding talks: one long tmux session. |
| `product` | light | A launch-day talk for a developer tool. |
| `swiss` | light | An argument-driven engineering talk, Zurich concert poster style. |
| `newsprint` | light | A war story or postmortem told as front-page news. |
| `zine` | light | An opinionated, scrappy talk, cut and photocopied the night before. |
| `poster` | light | A loud, fast, opinionated talk in the Takahashi style. |
| `blueprint` | dark | Architecture and systems talks as sheets from the drawing set. |
| `riso` | light | A warm community-conference talk, a two-ink print off the drum. |
| `retro-computing` | light | A nostalgic systems talk told from a 1990 desktop. |
| `paperback` | light | A story-led talk with the warmth of a mid-century paperback series. |
| `keynote` | dark | A big-room launch talk, one idea at a time on a dark stage. |
| `editorial` | light | A story-led talk like a long-read magazine feature. |
| `observatory` | dark | A data-heavy infrastructure talk, charted like a night sky. |
| `arcade` | dark | A high-energy war-story talk framed as a game you can win. |
| `isometric` | light | A friendly architecture walkthrough with infrastructure as the cast. |
| `ink` | light | A calm, reflective talk about hard-won lessons. |
| `lab-notebook` | light | An evidence-first engineering talk, every claim a numbered figure. |
| `bauhaus` | light | A bold talk about first principles: circle, triangle, square. |
| `sketch` | light | An explainer talk that builds an idea like a whiteboard drawing. |
| `transit` | light | An architecture talk that walks the audience like a metro map. |

```yaml
---
theme: terminal
---
```

While presenting, `t` cycles themes live and `?theme=<slug>` in the URL forces one. `tap new --theme <slug>` scaffolds a deck with a theme already chosen.

## Fragments (Incremental Reveals)

Add `<!-- pause -->` on its own line inside a slide to split it into steps that arrive one at a time:

```markdown
# Key Results

Revenue increased 25% this quarter.

<!-- pause -->

International expansion is the biggest driver.
```

Set `fragments: true` in frontmatter or a slide directive to have bullet lists reveal one item at a time automatically instead.

## Line Highlighting

Add `{...}` after the language in a fenced code block to highlight specific lines:

````markdown
```go {3-4}
func connect() {
    config := loadConfig()
    conn := createConnection(config)  // highlighted
    conn.Verify()                      // highlighted
    return conn
}
```
````

`{5}`, `{1,3,5}`, `{1-3}`, and `{1-3,7,9-11}` are all valid.

## Speaker Notes

The recommended form for free-text notes is a trailing HTML comment anywhere in the slide, most often at the end:

```markdown
# Key Results

Revenue increased 25% this quarter.

<!-- notes:
- Highlight international expansion
- Mention new product line
-->
```

This form never collides with directive parsing, unlike a `notes:` line inside the directive block, which is also supported but requires care around keys that look like directives (see `docs/reference/slide-directives.md`).

## Tag and Badge

`tag` and `badge` are small decorative metadata labels rendered on the slide, set as directives:

```markdown
<!--
tag: "01 · Kickoff"
badge: "v2.0"
-->
```

Keep a `tag` to a real word or short phrase; a tag rendered at poster size on a `poster`-style theme reads badly if it's not actually word-shaped.

## Readability Rules for Projected Slides

A slide is read from the back of a room in a few seconds, not read closely like a document:

- One idea per slide. If a slide needs "and" to describe its point, split it.
- Few words. A sentence-length headline, not a paragraph.
- About 80 characters per code line, and no more than about 12 lines of code on one slide; use `code-focus` and let the code block breathe, or split the example across slides.
- At most 4 bullets per list. A fifth bullet is usually a sign the slide should split.
- A sentence headline, not a noun phrase: "Requests dropped 40% after the cache fix," not "Cache Fix Results."

## Rich Content

- **Mermaid diagrams** - fenced ` ```mermaid ` blocks render flowcharts, sequence diagrams, ER diagrams, and more, themed to match the slide's theme. See `docs/guide/mermaid-diagrams.md`.
- **Live code execution** - a fenced code block with `{driver: sqlite, connection: demo}` (or `mysql`, `postgres`, `shell`) runs against a real backend in `tap dev`; static builds don't execute code. See `docs/guide/live-code-execution.md` and `docs/reference/drivers.md`.
- **Asciinema recordings** - ` ```asciinema {src: "./demo.cast", autoPlay: true} ``` ` plays a terminal recording inline. See `docs/guide/asciinema.md`.
- **Maps** - animated map slides for geographic data. See `docs/guide/map-animations.md`.
- **Deck components** - a React component from a file next to the deck, as a whole slide (`layout: ./slides/Thing.jsx`) or as an inline ` ```component ./charts/Thing.jsx ` fence with JSON props. Tap bundles it itself, gives it the slide's steps and the theme's tokens, and renders it in place. Use one for a picture that moves or a diagram that builds up; markdown still holds the words. Two rules worth knowing before you write one: `theme.accent` is a fill color, so text and thin lines on the slide background use `theme.accentText`; and any timer or looping animation must be gated on `useActive()`, because the presenter view and the overview mount the same component. See `skills/tap/rules/components.md`.

## Rule Index

Detailed rules live under `skills/tap/rules/`:

1. **getting-started** - Installation and first presentation
2. **writing-slides** - Markdown syntax, slide separators, speaker notes
3. **frontmatter** - Global config options (title, theme, transitions)
4. **layouts** - All 12 layouts with slot markers
5. **slide-directives** - Per-slide HTML comment syntax
6. **animations** - Transitions and fragment reveals
7. **code-blocks** - Syntax highlighting, line highlighting, diffs
8. **live-code** - Drivers for SQLite, MySQL, PostgreSQL, shell
9. **themes** - The 21 themes, their tokens, and customization
10. **cli** - tap new/dev/present/build/serve/export/slide/component/theme commands
11. **best-practices** - Presentation design tips
12. **mermaid** - Mermaid diagram support (flowcharts, sequence, ER, etc.)
13. **ai-images** - AI image generation from prompts
14. **asciinema** - Asciinema terminal recording playback
15. **map-animations** - Animated map slides
16. **components** - Deck-supplied React components, `tap component new`, `tap export images`
