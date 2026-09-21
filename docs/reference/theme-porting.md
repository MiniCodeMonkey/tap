# Porting a theme mockup into Tap

This is the guide for turning one of the 20 approved theme mockups
(`.superpowers/brainstorm/themes-v3/<slug>.theme.html`, git-ignored) into a
real Tap theme: `frontend/src/lib/themes/<slug>/theme.css` and `theme.json`.
It was written while porting `terminal`, the worked example checked in
alongside this guide. Read `frontend/src/lib/themes/terminal/theme.css` next
to this document; it demonstrates every technique described below.

You will not need to run `npm install`. Every font family the 20 themes use
is already installed (see the font table at the end). You will not need to
touch any file outside your own `frontend/src/lib/themes/<slug>/` folder and
its snapshot folder.

## 1. Folder layout

```
frontend/src/lib/themes/<slug>/
  theme.css
  theme.json
```

Nothing else. `theme.json` shape (`mermaid` optional, everything else
required):

```json
{
	"name": "Terminal",
	"polarity": "dark",
	"pitch": "One sentence: what kind of talk this is for.",
	"illustration": {
		"medium": "flat vector technical drawing, white-ink-on-blue blueprint diagram",
		"line": "3 to 4px even strokes, drafting-table precision",
		"shapes": "rectangles and circles, grid-aligned, no organic curves",
		"texture": "faint graph-paper grid in the background",
		"palette_use": "deep blue dominant, white line work, one yellow accent for a callout",
		"mood": "precise, engineered, under-construction",
		"avoid": "gradients, photos, rounded corners, decorative flourish"
	},
	"mermaid": {
		"themeVariables": { "fontFamily": "JetBrains Mono", "primaryColor": "#...", "...": "..." },
		"quietStyle": "fill:#...,stroke:#...,color:#...",
		"curve": "basis"
	}
}
```

`name`, `polarity`, and `pitch` are already decided for you: copy them
verbatim from your theme's entry in `internal/themes/themes.json` (the
single source of truth Go, the CLI, and the frontend all read). Don't
invent new wording.

`illustration` is seven short phrases describing how an illustration should
look next to this theme. `tap theme show <slug> --prompt` turns them into a
style brief for an image model, so write them from your theme's own CSS and
identity: short noun phrases, no sentences, no em dashes.

### Generated tokens

`internal/themes/tokens.json` is generated from every theme's root token
block by `frontend/scripts/extract-theme-tokens.mjs`, and Go embeds it so
`tap theme show` can print your tokens without a browser. After changing a
token, from `frontend/`:

```bash
npm run tokens         # regenerate and write internal/themes/tokens.json
npm run tokens:check   # fail if it is out of date
```

Commit the regenerated file. A Go test and `tokens:check` both guard it.

## 2. The selector rename

The mockup scopes every rule under `.theme-<slug>`. The real app has no such
class. Scope every rule under `[data-theme="<slug>"]` instead:

```css
/* mockup */
.theme-terminal .slide { ... }

/* real theme.css */
[data-theme='terminal'] .slide[data-layout] { ... }
```

Note the `[data-layout]` addition: see the next section for why it matters.
Every `@keyframes` name still needs the `<slug>-` prefix the brief required
of the mockup; keep it.

## 3. Markup differences between the mockup and the real app

The mockup's `deck.html` is a close approximation of the real markup, but
not identical. These are the differences that matter for a port, discovered
while porting `terminal`:

### 3.1 Two elements share the class `slide`, scope past the outer one

`SlideCanvas` renders `<div class="slide-container">` (the letterbox area
around the canvas, always black outside it, never a theme root itself) and
inside it `<div class="slide" data-theme="..." style="width:1920px; ...">`
as the 1920x1080 scaling wrapper, with no `data-layout`, `data-index`, or
`data-total` attribute. The *real* rendered slide, the one with those
attributes, that the mockup's `.slide` selector was written against, is
nested one level deeper: `<div class="slide" data-layout="..." data-index="N" data-total="M">`.
The outer wrapper always lays out at its full inline size (`flex: none`
keeps it from shrinking as a flex child of `.slide-container`); only a
`scale()` transform, driven by the container's actual measured size, ever
makes it look smaller. That is what makes the same theme CSS produce the
same layout at 1920x1080 and inside a 250px-wide presenter thumbnail alike.

If you write `[data-theme='x'] .slide { ... }` the way the mockup does,
you style **both** elements. For plain declarations (background, color,
font) this is harmless duplication. It is not harmless for anything that
reads `attr(data-index)`/`attr(data-layout)` in `content`, or that
positions a `::before`/`::after` pseudo-element with an explicit size: the
outer wrapper gets its own copy of that pseudo-element, with empty
`attr()` values, rendered at the same coordinates as the real one. For
example, a `content: ' ' attr(data-index) ':' attr(data-layout) '*'` status
bar shows up twice, once correct and once reading `” :*"`.

**Fix: scope every such rule to `.slide[data-layout]`**, not bare `.slide`.
`theme.css`'s status bar rules do this throughout. `data-theme` itself lives
on the scaling wrapper (the inner `.slide`), matching the interface the
loader and `SlideCanvas` already implement: you never need to set it
yourself.

**Slide numbers must honor `data-slide-numbers="off"`.** A deck with
`slideNumbers: false` in its frontmatter gets that attribute on the same
inner `.slide`. A theme that prints `attr(data-index)` or `attr(data-total)`
ends its `theme.css` with a rule that removes the number, usually
`[data-theme='x'] .slide[data-layout][data-slide-numbers='off']::after { content: none; }`.
Put it last, so it outranks layout-specific rules for the same
pseudo-element, and exclude any layout that reuses that pseudo-element for
a decoration instead of the number.

### 3.2 `.slide-content` is unpadded by default; overriding its box needs `width`/`height: auto` too

Base's `layouts.css` sets `.slide-content { position: relative; width: 100%;
height: 100%; }`. If your theme repositions it, for example `terminal`'s
prompt-line design needs `.slide-content` inset by a left/right/top/bottom
margin, you must write:

```css
[data-theme='terminal'] .slide-content {
	position: absolute;
	width: auto;
	height: auto; /* <-- required, see below */
	left: 96px;
	right: 96px;
	top: 84px;
	bottom: 120px;
}
```

Leaving out `width: auto` (and `height: auto`) is a real bug that is easy to
write by accident: CSS resolves each *property* through the cascade
independently across every matching rule, not per matching *rule*. Since
your `[data-theme='x'] .slide-content` selector doesn't set `width`, the
cascade still finds base's `.slide-content { width: 100% }` and applies it,
even though your selector has higher specificity, that only matters for
properties both rules set. An absolutely positioned box with an explicit
`width` *and* both `left` and `right` set is over-constrained; the browser
keeps `width` and `left`, and silently drops `right`. The result: your inset
box renders at full slide width, starting from `left`, and overflows past
the right edge by exactly the margin you meant to reserve. This produced
dozens of false-looking overflow failures during the `terminal` port before
the cause was found. Check for it first if a broad set of elements overflow
the right or bottom edge by a consistent amount.

The same applies to any other base-styled element you reposition with
`position: absolute` plus insets: explicitly set `width`/`height` (usually to
`auto`) rather than assuming an unset property falls back to nothing.

### 3.3 `box-sizing: border-box` is already global, don't add your own

`app.css` sets `box-sizing: border-box` on `.slide-container` and everything
inside it. You do not need `* { box-sizing: border-box }` in your theme.
(This was a real, separate bug fixed as part of the theme infrastructure
work: before the fix, a rule as ordinary as `.layout-x { width: 100%;
padding: 5rem }` rendered `2 * padding` wider than its container. If you see
that pattern of overflow and you're working from a copy of this repo
without the fix, that's the cause.)

### 3.4 Fragment classes

The mockup deck marks every list item `class="fragment fragment-visible"`.
The real app does carry a `fragment` class: the parser writes
`class="fragment fragment-hidden"` on every fragment element, an
auto-fragmented `<li>`, or the `<div>` a `<!-- pause -->` wraps around the
content that follows it, and `Slot.tsx` then toggles `fragment-hidden` and
`fragment-visible` on it directly as the presenter steps through. Style
`.fragment-hidden`/`.fragment-visible` the way the mockup styled its combo
classes. For a rule that should apply to every fragment regardless of its
visibility state, match `:where(.fragment)` rather than `.fragment`:
`:where()` contributes no specificity, so it won't fight your more specific
hidden/visible rules.

Because a `<!-- pause -->` wraps the block that follows it in its own
`class="fragment ..."` div, a selector written against the mockup's flat
markup can miss fragmented content. `.slot > ul` matches a list that opens
a slot directly, but not one a `<!-- pause -->` pushed into a wrapper div of
its own. Cover both:

```css
[data-theme='<slug>'] .slot > ul,
[data-theme='<slug>'] .slot > .fragment > ul {
	...
}
```

### 3.5 Slot elements carry an extra class

A slot renders as `<div class="slot slot-content slot-<name>">`, not just
`class="slot slot-<name>"`. This doesn't require any change to your
selectors (`.slot > h2`, `.slot-default`, etc. all still match), just don't
be surprised to see `slot-content` in the DOM.

### 3.6 Print mode and entrance animations

PDF export (`?print=true`) sets `data-print="true"` on the scaling wrapper
(the inner `.slide`), the same element that carries `data-theme`. A global rule in `app.css` then
turns off every descendant's `animation` and `transition`, including
pseudo-elements, under it. That means your theme's animated elements freeze
wherever `animation: none` catches them mid-keyframe, not at a smoothly
finished state. Make each animated element's *natural*, un-animated state
(its plain CSS, with no animation running) the state you want it to look
like in print. An entrance animation should grow toward its final size,
scaling up to `1`, and never overshoot it and settle back down: a
mid-overshoot frame is a plausible freeze point, and an unfinished-looking
smaller element is far more forgiving to land on than a too-large one.

### 3.7 Four layouts and several elements the mockup never covered

The mockup's fixed deck exercises 8 layouts and 11 slides. The real deck
(`testdata/themes.md`, the kitchen-sink deck every check runs against) has
all 12 layouts and every markdown element a theme must survive. You are
responsible for designing, in your theme's own voice, everything the mockup
didn't show you:

- **Layouts**: `three-column` (`slot-left`, `slot-center`, `slot-right` around
  a `slot-default` header, like `two-column` with one more pane),
  `cover` (single `slot-default`, meant to fill the frame: the mockup's
  `split-media` image treatment is a good starting point for its mood, but
  cover has no image slot of its own), `sidebar` (`slot-default` +
  `slot-sidebar`), `blank` (single `slot-default`, no chrome expectations,
  used for custom HTML and bare code fences like the map slide).
- **Nested lists** (`ul` inside `li`): give them a quieter treatment than
  the top-level list marker, meaning a lighter font-weight or a more muted
  color, never a smaller font-size: the 40px body minimum (section 6)
  applies to nested list items too.
- **Links** (`a`): needs its own color and hover state; the mockup never
  includes one.
- **Inline `code`** outside of `pre` (in `p` and `li`): the mockup covers
  this narrowly (`.slot > p code`); make sure it also works inside nested
  list items and other slots.
- **`h3`–`h6`**: the mockup only styles `h2` (slot headings) and layout-specific
  `h1`/`h3` uses. Give every heading level a size relationship that holds up.
  Also see section 6 on `font-weight`/`letter-spacing` for headings.
- **An image on a plain `default` slot** (not `split-media`): give it a
  treatment consistent with your `split-media`/cover image treatment.
- **Code blocks without highlighted lines**: `pre.shiki` without
  `.has-highlighted`, make sure your code chrome doesn't assume a
  highlighted line exists.
- **Badges** (`.slide-badge`, top-right by default) alongside tags
  (`.slide-tag`): the mockup only used `.slide-tag`.
- **Live code block chrome** (`.live-code-block`, with a `.run-button`, a
  `.result-container`, and so on, see `rich-blocks.css`): styled through
  the token bridge (section 4) by default; override specific pieces if the
  theme calls for it, the way `terminal.css` restyles `.run-button`.
- **The scroll layout** (`scroll: true` in frontmatter): content inside
  `.scroll-content` is allowed, expected, to be taller than the slide; the
  slide clips it (`.slide { overflow: hidden }`) and the presenter scrolls it
  into view. Don't fight this; just make sure nothing overflows
  *horizontally*.
- **Mermaid diagrams** (`.mermaid-diagram`, not `.mermaid`): mermaid renders
  a diagram's labels as `<p>` elements inside the SVG's `foreignObject`s, so
  a theme's `.slot p` safety-net rule (section 6) leaks into them unless the
  theme resets font-size, and anything else it doesn't want inherited, on
  `.mermaid-diagram p` specifically. Diagram text is excluded from the
  minimum-size check, so this reset can, and should, be far smaller than
  40px.
- **Asciinema** (`.asciinema-player-wrapper`): the player and its CSS are
  bundled (no CDN, no network access needed), loaded as a lazy chunk the
  first time a slide has an asciinema block. Keep any styling you add to
  the wrapper minimal and generic (border, background) rather than
  assuming its internal DOM.
- **The map slide** (`.map-slide`): excluded from the minimum-size check
  (MapLibre renders its own labels) and from the overflow check's element
  list, but still worth a glance: it's excluded from checks, not from
  "should look intentional." `.map-slide` picks up `position: relative` from
  MapLibre's own setup, and `.map-content-overlay` (the slot content
  stacked on top of the map) has `z-index: 10`; keep that in mind if your
  theme adds its own positioned or z-indexed elements to a map slide.

## 4. Bridge your tokens to the base theme's token names

The design spec's token set (`--bg --fg --muted --accent --accent-text
--accent-2 --surface --status-ok --status-warn --status-error
--font-display --font-body --font-mono --ease --dur --space-unit --radius
--stroke-width`) is what your theme thinks in. All of them are required: a
deck component reads them with `useTheme()` to color and size itself in
your theme's terms, so pick values that match the rest of your design
rather than copying another theme's.

Four of them are newer and easy to miss:

| Token | Rule |
|-------|------|
| `--status-ok` | A healthy or passing state, used as a **fill**. At least **3:1** against `--bg` |
| `--status-warn` | A warning state, used as a fill. At least **3:1** against `--bg` |
| `--status-error` | A failed state, used as a fill. At least **3:1** against `--bg` |
| `--accent-2` | A real second accent if your theme has one; otherwise set it equal to `--accent-text` |

Two rules apply to the status three, and the check suite enforces both:

1. **Each is at least 3:1 against `--bg`**, which is what makes a filled
   status dot or bar visible at all.
2. **Any two of the three differ by a contrast ratio of at least 1.35.**
   Hue alone is not enough: a colorblind viewer, or a washed-out projector,
   flattens red against green. Step them apart in lightness so the three
   stay distinguishable as light, mid, and dark as well as as colors.

Picking a saturated red, amber, and green straight off the wheel usually
fails the second rule, since they can land at nearly the same luminance.
Darken one and lighten another until the pairwise ratios clear 1.35. For
reference, `terminal` settled on ok `#23b561`, warn `#bd7100`, error
`#d60600`.

These are fills, not text colors: a component paints a label on one with
`textOn()`. Keep them recognisable as ok, warning, and error rather than
three shades of your accent. It is **not** what most of the app's shared CSS reads.
`layouts.css`, `prose.css`, `rich-blocks.css`, and `ui-components.css`, the
files that style the live code block, the map slide, `.slide-badge`,
`.slide-tag`, and generic prose, all read the *base* theme's token names:
`--color-bg`, `--color-text`, `--color-muted`, `--color-accent`,
`--color-code-bg`, `--color-code-text`, `--color-code-inline-bg`,
`--color-border`, `--color-surface`, `--color-surface-elevated`,
`--color-link`, and `--font-sans`.

Set both, at your theme's `[data-theme="<slug>"]` scope, with the base names
aliased from your own:

```css
[data-theme='terminal'] {
	--bg: #0f1a16;
	--fg: #dcebe0;
	/* ...the rest of your own tokens... */

	--color-bg: var(--bg);
	--color-text: var(--fg);
	--color-muted: var(--muted);
	--color-accent: var(--accent);
	--color-code-bg: var(--surface);
	--color-code-text: var(--fg);
	--color-code-inline-bg: rgba(220, 235, 224, 0.12); /* pick your own translucent inline-code bg */
	--color-border: var(--muted);
	--color-surface: var(--surface);
	--color-surface-elevated: var(--surface);
	--color-link: var(--accent-2); /* or --accent-text, if you have no secondary accent */
	--font-sans: var(--font-body);
}
```

Do this once and the live code block, map slide, badges, and tags all
follow your theme automatically, with no further selectors needed. You only
need to write your own overrides for the pieces that should look distinctly
like your theme rather than the shared default (`terminal.css` restyles
`.run-button`'s color and the code container's border, and leaves the rest
to the bridge).

Also set the Shiki CSS-variable tokens the same way, at the same scope:
Shiki reads these directly, there is no separate bundled-theme choice to
make:

```css
--shiki-foreground: var(--fg);
--shiki-background: var(--surface);
--shiki-token-comment: var(--muted);
--shiki-token-string: var(--accent-2);
--shiki-token-string-expression: var(--accent-2);
--shiki-token-keyword: var(--accent-text);
--shiki-token-function: var(--fg);
--shiki-token-constant: var(--accent-text);
--shiki-token-parameter: var(--fg);
--shiki-token-punctuation: var(--fg);
--shiki-token-link: var(--accent-2);
```

At most two syntax colors plus the text color, per the brief: don't invent
a six-color palette here even though there are many `--shiki-token-*`
variables to fill in.

## 5. Mermaid

`theme.json`'s `mermaid.themeVariables` values must be **literal colors**,
never `var(...)`: mermaid's theming engine parses each one to derive
shades and throws on a string it can't parse as a color. `fontFamily` must
be a single bare family name (`"JetBrains Mono"`, not a comma-separated
stack): mermaid measures label text in the wrong font, and silently, if you
give it a stack or quotes it doesn't expect.

Set `nodeTextColor` explicitly. It does not derive from `textColor` the way
you might expect from mermaid's own `base` theme defaults; leaving it out
can leave node labels in mermaid's default text color instead of yours.

`quietStyle` is a mermaid `classDef` declaration string (the same
comma-separated syntax mermaid's own `classDef` line uses, e.g.
`"fill:#0f1a16,stroke:#9db8a8,color:#dcebe0"`). The app appends it to a
rendered diagram's source as `classDef quiet <your string>`, so a diagram
gets the quiet treatment by assigning the `quiet` class to specific nodes in
its own source, mermaid-style: `class ServerNode quiet` or the shorthand
`ServerNode:::quiet`. If a diagram doesn't use the `quiet` class, your
`quietStyle` has no visible effect on it, which is correct: it's meant for
background/app-server nodes a diagram author opts into, not a global style.

## 6. Minimum sizes are real space, budget for them

The checks enforce (see `frontend/e2e-themes/checks.ts`,
`MINIMUM_FONT_SIZES`): body text (`p`/`li` as direct slot content, and
table text, `td`/`th`) at least 40px, code (`pre code`) at least 36px, and
everything else at least 24px (the hard floor). `h1`/`h2` have their own,
separate bar: 60px or larger is a *recommendation*, not a requirement - a
theme that runs a short or deliberately modest headline under 60px (still
clearing the 24px floor) gets a warning in the test output, not a failing
check. These are still large: a 40px `p`/`li` at your `--font-body` is
often taller and wider than you'd guess from the mockup's shorter,
carefully-fitted mockup copy. At 36px, a typical monospace font (about
0.6em average glyph advance width) fits roughly 80 characters across the
1728px safe code width (the 1920px canvas minus a 96px margin on each
side) - budget code samples accordingly.

Three consequences worth planning for up front, all hit during the
`terminal` port:

- **Narrow columns overflow vertically before they overflow horizontally.**
  A ~590px-wide column (roughly the left pane of a `1fr 1fr` two-column
  layout) at 40px in a display/monospace font still wraps a longer
  sentence into several lines, taller than most rows have room for. Give a
  column that needs to hold real body copy noticeably more than half the
  available width (`terminal`'s `split-media` moved from `0.82fr 1fr` to
  `1.1fr 1fr` to fix exactly this), and keep the kitchen-sink deck's copy
  for cramped layouts (`two-column`, `three-column`, `sidebar`,
  `split-media`) on the short side.
- **Headings inherit `font-weight: 800` and `letter-spacing: -0.03em` from
  base.** `layouts.css` sets both on layout headings (for example
  `.layout-title .slot-content h1`). A single-weight display font rendered
  at `font-weight: 800` is browser-synthesized faux bold, which usually
  looks worse than the font's real weight. Override both at
  `.slide[data-layout]` specificity, the same specificity base uses, if
  your display font has no real 800 weight or doesn't want base's tight
  tracking.
- **Write a "safety net" rule near the top of your file, not the end.** You
  will not remember to set an explicit font size on every slot/layout
  combination the kitchen-sink deck exercises. Add it right after your
  token bridge (section 4):

  ```css
  [data-theme='<slug>'] .slot p,
  [data-theme='<slug>'] .slot li {
  	font-size: 40px;
  	line-height: 1.35; /* pick something readable for your font */
  }
  [data-theme='<slug>'] pre code {
  	font-size: 36px;
  }
  [data-theme='<slug>'] .slot td,
  [data-theme='<slug>'] .slot th {
  	font-size: 40px;
  }
  ```

  Without this, anything you didn't explicitly size falls back to base's
  much smaller default typography (its own `p` is nowhere near 40px), and
  the check fails on slots you never even looked at. Put it early rather
  than last: CSS only breaks a specificity tie by source order, and your
  more specific layout rules (`.layout-x .slot p`, for example) still win
  regardless of where the safety net sits in the file. But if any later
  rule in your own file happens to match at the *same* specificity as the
  safety net (another plain `.slot p` selector, say), whichever one comes
  last in the file wins; placing the safety net last would let it silently
  override a same-specificity rule you wrote on purpose.

## 7. Heading length classes

Every rendered `h1`, `h2`, and `h3` carries a `data-length` attribute:
`"short"` (up to 24 characters), `"medium"` (25-60), or `"long"` (over 60).
It's computed server-side, in runes, from the heading's visible text after
inline markup (bold, code spans, links) is stripped away - see
`internal/parser/heading_length.go`.

The problem it solves: every theme's title type has to be sized for a
worst-case, near-60-character title, which makes a genuinely short title
(a one- or two-word talk name) look small and unbalanced by comparison.
`data-length` gives a theme a hook to scale a short heading up:

```css
[data-theme='<slug>'] .layout-title .slot-content h1 {
	font-size: 7rem;
}

[data-theme='<slug>'] .layout-title .slot-content h1[data-length='short'] {
	font-size: 8.5rem;
}
```

Size for `long` first (your base rule, unqualified by `data-length`, the
worst case you already have to design for), then layer `[data-length='medium']`
and `[data-length='short']` overrides on top that scale up from there. Keep
the steps modest: a `short` title is still one of several headings the
theme has to keep in a consistent type scale, not a special case that
breaks it.

### Blockquote length classes

Blockquotes carry the same `data-length` attribute headings do, on their
own rune-count thresholds:

| Class | Rune count |
|-------|-----------|
| `short` | up to 80 |
| `medium` | 81 to 180 |
| `long` | above 180 |

Thirteen of the built-in themes step the quote size down for a `long`
quote. Size for `long` first, then scale `short` and `medium` up, the same
way you do for headings:

```css
[data-theme='<slug>'] .slot blockquote[data-length='short'] {
	font-size: 3.4rem;
}
```

## 8. Fonts

The frontend build emits **`.woff2` only**. The `@fontsource` packages ship
`.woff` as well, and the build strips those `src` entries and deletes the
files, since every browser tap supports has read `.woff2` for years. Do not
add a `.woff` fallback by hand; it will be removed.

Every family the 20 themes use is already installed as an `@fontsource`
package (or `@fontsource-variable`, for the one family with no static
package). Import only the Latin-subset files for the weights you actually
use, at the top of `theme.css`:

```css
@import '@fontsource/jetbrains-mono/latin-500.css';
@import '@fontsource/jetbrains-mono/latin-700.css';
```

No Google Fonts `<link>` at runtime, self-hosted only, per the design spec.
Check the family's row below for its package name and the weights that
actually exist; not every family ships every 100–900 weight, and importing
a weight file that doesn't exist is a silent no-op (the browser falls back
to the nearest weight it has, which may not be what your CSS asked for).

| Family (as named in the mockups) | Package | Available Latin weights |
| --- | --- | --- |
| Jersey 10 | `@fontsource/jersey-10` | 400 |
| Lexend | `@fontsource/lexend` | 100, 200, 300, 400, 500, 600, 700, 800, 900 |
| Red Hat Mono | `@fontsource/red-hat-mono` | 300, 400, 500, 600, 700 |
| Tiny5 | `@fontsource/tiny5` | 400 |
| Outfit | `@fontsource/outfit` | 100, 200, 300, 400, 500, 600, 700, 800, 900 |
| Space Mono | `@fontsource/space-mono` | 400, 700 |
| IBM Plex Mono | `@fontsource/ibm-plex-mono` | 100, 200, 300, 400, 500, 600, 700 |
| Saira Condensed | `@fontsource/saira-condensed` | 100, 200, 300, 400, 500, 600, 700, 800, 900 |
| Saira Semi Condensed | `@fontsource/saira-semi-condensed` | 100, 200, 300, 400, 500, 600, 700, 800, 900 |
| Fraunces | `@fontsource/fraunces` | 100, 200, 300, 400, 500, 600, 700, 800, 900 |
| Newsreader | `@fontsource/newsreader` | 200, 300, 400, 500, 600, 700, 800 |
| DM Mono | `@fontsource/dm-mono` | 300, 400, 500 |
| Shippori Mincho | `@fontsource/shippori-mincho` | 400, 500, 600, 700, 800 |
| Zen Kaku Gothic New | `@fontsource/zen-kaku-gothic-new` | 300, 400, 500, 700, 900 |
| Fredoka | `@fontsource/fredoka` | 300, 400, 500, 600, 700 |
| JetBrains Mono | `@fontsource/jetbrains-mono` | 100, 200, 300, 400, 500, 600, 700, 800 (already a project dependency) |
| Nunito | `@fontsource/nunito` | 200, 300, 400, 500, 600, 700, 800, 900 |
| Geist | `@fontsource/geist` | 100, 200, 300, 400, 500, 600, 700, 800, 900 |
| Geist Mono | `@fontsource/geist-mono` | 100, 200, 300, 400, 500, 600, 700, 800, 900 |
| STIX Two Text | `@fontsource/stix-two-text` | 400, 500, 600, 700 |
| Besley | `@fontsource/besley` | 400, 500, 600, 700, 800, 900 |
| UnifrakturCook | `@fontsource/unifrakturcook` (no hyphen: `@fontsource/unifraktur-cook` does not exist) | 700 only |
| Alegreya Sans | `@fontsource/alegreya-sans` | 100, 300, 400, 500, 700, 800, 900 |
| Courier Prime | `@fontsource/courier-prime` | 400, 700 |
| Jost | `@fontsource/jost` | 100, 200, 300, 400, 500, 600, 700, 800, 900 |
| Libre Baskerville | `@fontsource/libre-baskerville` | 400, 500, 700 (no 600) |
| Anton | `@fontsource/anton` | 400 only |
| Archivo Narrow | `@fontsource/archivo-narrow` | 400, 500, 600, 700 |
| Inconsolata | `@fontsource/inconsolata` | 200, 300, 400, 500, 600, 700, 800, 900 |
| Pixelify Sans | `@fontsource/pixelify-sans` | 400, 500, 600, 700 |
| Bagel Fat One | `@fontsource/bagel-fat-one` | 400 only |
| Bricolage Grotesque | `@fontsource/bricolage-grotesque` | 200, 300, 400, 500, 600, 700, 800 |
| Fira Mono | `@fontsource/fira-mono` | 400, 500, 700 |
| Patrick Hand | `@fontsource/patrick-hand` | 400 only |
| Permanent Marker | `@fontsource/permanent-marker` | 400 only |
| Archivo | `@fontsource/archivo` | 100, 200, 300, 400, 500, 600, 700, 800, 900 |
| Overpass | `@fontsource/overpass` | 100, 200, 300, 400, 500, 600, 700, 800, 900 |
| Alfa Slab One | `@fontsource/alfa-slab-one` | 400 only |
| Archivo Black | `@fontsource/archivo-black` | 400 only |
| Special Elite | `@fontsource/special-elite` | 400 only |

Import line pattern for any weight in the table above:
`@import '@fontsource/<package>/latin-<weight>.css';` (append `-italic`
before `.css` for an italic cut, where the package ships one; check
`node_modules/@fontsource/<package>/` if you need one and aren't sure).

## 9. Running your two private servers

You'll be given a Go port and a Vite port. Start both from the repo root and
`frontend/` respectively:

```bash
# from the repo root
go run ./cmd/tap dev testdata/themes.md --port <goPort> --headless

# from frontend/, in another terminal
TAP_API_PORT=<goPort> npx vite --port <vitePort>
```

`vite.config.ts`'s dev server proxies `/api`, `/ws`, and `/local` to
`TAP_API_PORT` (default 3000), so the Vite server on `<vitePort>` serves the
live React app, including images and asciinema casts a slide references
under `/local/...`, against the real backend without a CORS problem. Open
`http://localhost:<vitePort>/?theme=<slug>#<n>` in a browser to look at any
slide; add `&print=true` to match exactly what the checks and snapshots see
(no connection indicator, all fragments/steps shown at once).

Go through every slide of `testdata/themes.md` at full size before you run
the checks. The checks catch overflow, contrast, and minimum size, but not
"this looks weak" or "this diagram is too small," which only a human eye
catches.

## 10. Running the checks

From `frontend/`:

```bash
THEME=<slug> BASE_URL=http://localhost:<vitePort> npx playwright test -c playwright.themes.config.ts
```

Add `--update-snapshots` the first time, to create your theme's visual
baseline (written to `frontend/e2e-themes/__snapshots__/<slug>/`). Re-run
without that flag afterward: a snapshot diff is a real signal that
something changed, so don't reflexively re-run with `--update-snapshots`
once a baseline exists; look at the diff first.

The suite runs, per slide of the kitchen-sink deck: overflow (no text
element extends past the slide, except vertically inside a scroll slide's
`.scroll-content`), contrast (`--fg` and `--accent-text` at least 7:1
against `--bg`, `--muted` at least 4.5:1), occluded or clipped text, and
minimum text size (section 6), and the contrast of `--status-ok`,
`--status-warn`, and `--status-error` against `--bg` (at least 3:1 each,
since a component uses them as fills).

One check runs **live**, not in print mode: an entrance-animation check per
theme, asserting that an element which animates in never settles hidden. A
theme whose entrance keyframes end at `opacity: 0`, or that leaves a
transform parked off-slide, passes every print-mode check and still shows
the audience a blank slide; this is the check that catches it.

It also runs, once per theme: an isolation
check (switches from the
adjacent theme in `themes.json` order into yours with the `t` key, in one
page, and asserts the resulting computed styles equal a fresh page load of
your theme directly, which catches CSS that leaked in from the previous
theme's stylesheet, or that your theme failed to override from it) and
takes one screenshot per slide.

The occluded-or-clipped-text check
(`findOccludedOrClippedText`) catches two defects the overflow check can't
see, because both leave an element's own box entirely inside the slide:
text hidden under opaque chrome (a status bar, a page-number band) and text
cut off by an ancestor with `overflow` other than `visible`. It flags an
element (`h1`-`h6`, `p`, `li`, `td`, `th`, `blockquote`, `pre`,
`figcaption`, `dt`, `dd`) when either its own rendered text extends past
the visible box of a clipping ancestor by more than 2px (a scroll slide's
own vertical clip of `.scroll-content` is expected and exempt, matching the
overflow check's own scroll exemption; only horizontal clipping there is a
bug), or when more than 20% of sample points across its text is covered by
another element with an opaque background, an image, or a canvas. A
decorative overlay with `pointer-events: none` (scanlines, a vignette)
never falsely reports as chrome: the check uses `elementFromPoint`, which
already skips those elements the same way a real cursor would.

`THEME` selects a single theme; leave it unset to run every theme with a
folder (useful for `base` and `terminal`, not useful for you mid-port, since
your slug's folder won't exist until you've written `theme.css` and
`theme.json`).

When all checks pass and your snapshots look right in the report, stop both
servers. Per your task's instructions, do not commit, report what you
found.

## 11. Checklist before you report

- [ ] All 12 layouts styled: `title`, `section`, `default`, `two-column`,
      `three-column`, `code-focus`, `big-stat`, `quote`, `cover`, `sidebar`,
      `split-media`, `blank`.
- [ ] Nested lists, links, inline code, `h3`–`h6`, an image on a plain
      `default` slot, a table, code with and without highlighted lines, a
      badge and a tag, live code block chrome, the scroll layout, a mermaid
      diagram (including a `quiet`-classed node), the map slide, asciinema.
- [ ] `theme.json`'s `name`/`polarity`/`pitch` match `internal/themes/themes.json`.
- [ ] Contrast, overflow, and minimum-size checks pass (`readability:
      contrast`, `overflow and minimum text size, every slide`).
- [ ] Isolation check passes.
- [ ] Snapshots written and look right in a browser (not just "the checks
      are green," actually look at the deck).

## More lessons from the 20 ports

A handful of things that only became clear after porting all 20 themes,
past what the sections above already cover:

- **Wrap the whole reset selector in `:where()`, including `.slide`.** The
  margin/padding reset in section 5 keeps its specificity at zero by
  wrapping the tag list in `:where(...)`. Wrap the entire selector after
  the `[data-theme]` prefix, not just the tag list: `[data-theme='x']
  :where(.slide[data-layout] h1, .slide[data-layout] h2, ...)`, or
  `[data-theme='x'] :where(.slide[data-layout]) h1, ...`. Leaving `.slide`
  outside `:where()` still gives the rule enough specificity to fight your
  own layout-specific overrides later in the file.
- **Never use `:not()` to exclude something from the minimum-size safety
  net.** `:not()`'s specificity is that of its most specific argument, so
  `.slot p:not(.caption)` is *more* specific than the plain `.slot p` rules
  your layout overrides use, and now the exclusion wins fights it has no
  business winning. Make the exception itself more specific instead (target
  `.layout-x .slot p` directly) rather than trying to carve a hole in the
  safety net with `:not()`.
- **Give every grid child an explicit `grid-column` and `grid-row`.** A
  layout like `two-column` or `big-stat` places content with CSS grid; an
  implicit placement that happens to look right at one viewport silently
  breaks the moment your theme changes a slot's order or display type.
  Set both explicitly on anything you place in a grid.
- **`--shiki-foreground` must contrast with the code panel's background.**
  The theme suite's code contrast check reads every highlighted code
  block's actual rendered colors, including the plain, unclassified token
  color that falls back to `--shiki-foreground`. A theme that points
  `--shiki-foreground` at a color close to `--shiki-background` (or to the
  slide background, if the code panel doesn't have its own) fails that
  check even though every *classified* token might look fine.
- **A theme-local `.slide-content { height: auto }` starves downstream
  `flex: 1` layouts.** Several layouts rely on `.slide-content` filling its
  parent's height so a `flex: 1` child inside can size against something
  real. Overriding `.slide-content` to `height: auto` for one layout, for
  whatever reason, quietly breaks every other layout that shares the selector.
- **Give the `pre` itself an explicit `font-size`.** The live code block
  nests a `pre` one level deeper than a plain slot does, so a safety net
  written against `.slot pre code` alone can miss it; set `font-size` on
  the `pre` element too.
- **Base colors the big-stat number with the accent.** `layouts.css` sets
  `.layout-big-stat .slot-default h1 { color: var(--color-accent) }`. A
  theme whose accent is a highlighter color (a bright yellow or lime meant
  for small accents, not a giant number) needs to set that color itself
  rather than let the big-stat number inherit it.
- **Chromium drops `filter` when `mix-blend-mode` sits on the same
  element.** If you need both a filter (grayscale, contrast, brightness)
  and a blend mode on the same image, apply them to two nested elements
  instead of stacking both properties on one.
- **Screenshots taken before `document.fonts.ready` show a fallback font
  and a wrong layout.** Any manual check outside the test suite (the suite
  itself already waits) should await `document.fonts.ready` before reading
  computed styles or taking a screenshot; a slide measured against the
  fallback font reports the wrong overflow and the wrong minimum size.
- **The asciinema player is fitted by shared styles.** `rich-blocks.css`
  already sizes and scales the terminal to its wrapper; a theme should
  style only the frame around it (border, background, shadow), not reach
  into the player's own internals.
- **A slide's `background` directive is an inline style.** It beats the
  theme's slide background because inline styles win over any class
  selector's specificity. A theme with a `cover`-style layout that expects
  its own background should keep text on its own panel rather than assume
  the theme's background shows through.
- **A code box that is too small for its content must wrap, not shrink.**
  Set `white-space: pre-wrap; overflow-wrap: anywhere` and never shrink the
  code below the 36px minimum from section 6.
- **A theme may reserve chrome room with padding on `.slide-content`.**
  Scroll slides measure the content box, so that padding is already
  accounted for and needs no separate compensation.
- **Real tags are words, so never set `.slide-tag` at poster size.** A
  `tag` directive holds a short phrase, not a display headline; size it to
  read as a label, even in a theme whose other type is huge.

## Component slides

A slide can use a deck-supplied React component as its whole layout
(`data-layout="component"`). `LayoutComponent` renders it through
`DeckComponent` (`frontend/src/lib/components/DeckComponent.tsx`), which
wraps the component's own markup in `<div class="deck-component-root">` -
there is no `layout-component` wrapper class the way every other layout gets
`.layout-<name>`. The component decides for itself which slots it renders
through `Slot`, typically a `slot-title` heading plus a `slot-default`/other
body slot, alongside its own divs, SVG, or canvases.

**`.deck-component-root` is not unique to the whole-slide form.** An *inline*
component - a deck component referenced from inside a normal slide's
markdown, mounted at a `<div class="deck-component" data-component-index>`
placeholder `Slide.tsx` portals into - uses the exact same
`deck-component-root` class, nested inside a `.slot` on whatever ordinary
layout (`default`, `two-column`, ...) the slide actually uses. Only the
whole-slide form is ever a direct child of `.slide-content` (or of
`.scroll-content`, when `scroll: true`); an inline one is always nested
inside a `.slot`, several levels deeper. **Every rule below must be scoped
to `.slide[data-layout='component'] > .slide-content > .deck-component-root`
(plus the `> .slide-content > .scroll-content >` variant for a scrolling
whole-slide component) - never to a bare `.deck-component-root` or
`[data-theme='<slug>'] .deck-component-root` selector**, which also matches
every inline component on every other layout in the deck. A theme that gets
this wrong doesn't fail its own component slide; it corrupts every *other*
slide in the deck that happens to use an inline component - a stray prompt
line, an extra 80px inset, or a component stretched to the slide's full
height, none of which show up by looking at the component slide alone.

**The rule for a theme author:** give `component` the same padding as
`default`, and never impose a grid or a centering flex on it. A component
commonly fills its own root with an inline `position: absolute; inset: 0`
style (no stylesheet rule can override an inline style), so:

- `layouts.css` gives the whole-slide root a shared baseline, scoped the
  way described above: `position: relative` (so an inset-0 child's
  containing block is this box, not whatever ancestor happens to be
  positioned), `width`/`height: 100%`, a `display: flex; flex-direction:
  column` container, and `border: var(--slide-padding) solid transparent`
  for the padding.
- **Use `border`, not `padding`, to reserve space for an inset-0 child.**
  An absolutely positioned box's containing block is the padding *edge* of
  its positioned ancestor, so real `padding` sits inside that edge and an
  `inset: 0` child paints right over it. A transparent border reserves the
  same visual margin in a way the containing-block calculation actually
  honors. If your theme insets `.slide-content` itself (the terminal/
  blueprint/sketch pattern) and gives the layout wrapper `padding: 0`,
  give the whole-slide root the matching `border: 0` too - otherwise it
  silently keeps the shared 5rem/6rem border on top of your own inset,
  double-padding component slides only.
- **Set every `border-*-width` you touch explicitly; don't rely on a
  shorthand that only covers one side.** `border-top: 72px solid
  transparent` sets only the top-side longhands - the left/right/bottom
  widths still fall through to whatever lower-specificity rule set them
  (the shared 80px default, most often), so the slide ends up inset by
  your reserve *plus* that leftover 80px on the sides you meant to leave
  alone. Write `border-width: 72px 0 0 0; border-style: solid; border-color:
  transparent;` instead when only one side needs a nonstandard value.
- **A theme-scoped cancelling selector needs to match the shared rule's
  specificity, not just its class.** The shared rule above is scoped
  through two chained classes and an attribute selector
  (`.slide[data-layout='component'] > .slide-content >
  .deck-component-root`); a theme rule meant to override it - for example
  a generic `[data-theme='x'] .slide-content > div` used to zero every
  layout wrapper's padding at once - can lose that fight on specificity
  even though it's theme-scoped, if its own selector shape is simpler.
  Give the override the same selector shape, scoped by theme, rather than
  trusting a looser selector that happened to work before this shared rule
  existed.
- Where your theme styles `[class^='layout-']` as a selector (the
  padding/flex reset most themes use for their `.slide-content`-inset
  pattern), add the scoped whole-slide-root selector to the same list: it
  doesn't match the `[class^='layout-']` attribute selector on its own.
- Where a theme reserves chrome space with an in-flow sibling inside the
  layout wrapper (terminal's blank prompt line), that flow
  sibling doesn't push an `inset: 0` child down the way it pushes a normal
  flow child down. Reserve the same space with a border (or, for a
  fixed-position chrome element like a page-number roundel that isn't
  inside the wrapper at all, extra `border-*-width` on the whole-slide root
  matching the clearance the heading rule gives content) and draw the
  decoration as an absolutely positioned overlay inside that reserved
  strip instead of a flow element.
- **Don't assume your default layout's whole inset comes from one place.**
  A theme can build up its `default` heading's real position from several
  layers at once - an outer `.slide[data-layout]` padding, a
  `[class^='layout-']`/`.layout-<name>` padding the theme never actually
  zeroes out (so the *shared* base default keeps applying, unmodified),
  and a heading-specific margin on top of both - and a component only
  needs to reserve exactly the sum. Measure the real thing
  (`getBoundingClientRect()` of the default heading vs. the whole-slide
  component's content, per the safe-area table below) rather than reading
  one rule and assuming it's the whole story.
- Never style bare `div`, `svg`, `span`, etc. inside the component root -
  that's the component's own markup. Only `.slot`/heading/body rules that
  already apply generically (`.slide[data-layout] h1`, `.slot-content p`,
  and so on) should reach into a component slide, the same way they reach
  into every other layout.

### Safe areas

The usable area of a `component` slide on the 1920x1080 canvas, after the
port above, matching each theme's own `default` layout. Measured with
`getBoundingClientRect()` (Playwright, against a running dev server) of
each theme's `default`-layout heading versus the whole-slide component's
own rendered content edges - not eyeballed off a screenshot. Chrome notes
call out fixed decoration a component's content must still clear (page
numbers, roundels, running heads); tag/badge-only extra clearance (shown
only when a `tag`/`badge` directive is set) is omitted since slide 3 of
the example deck carries neither.

| theme | left | right | top | bottom | chrome |
| --- | --- | --- | --- | --- | --- |
| base | 96 | 96 | 96 | 96 | none |
| arcade | 176 | 176 | 256 | 230 | HUD bars top/bottom, counted in the totals |
| bauhaus | 176 | 176 | 176 | 176 | outer slide padding (96) plus the shared unmodified layout padding (80), counted in the totals |
| blueprint | 96 | 96 | 92 | 176 | title block, bottom-right corner |
| editorial | 96 | 96 | 190 | 96 | running head/foot rules |
| ink | 176 | 140 | 120 | 110 | none major |
| isometric | 96 | 96 | 96 | 96 | none major |
| keynote | 176 | 176 | 176 | 176 | outer slide padding (96) plus the shared unmodified layout padding (80), counted in the totals (tag adds +84 top when set) |
| lab-notebook | 192 | 96 | 196 | 96 | margin rule/binding on the left |
| newsprint | 96 | 96 | 164 | 84 | running head band at top |
| observatory | 176 | 176 | 256 | 176 | sky/tick-ruler chrome, counted in the totals |
| paperback | 96 | 96 | 96 | 140 | counter band at bottom (the `default` heading itself bleeds past this box by design - a banner treatment with a negative margin - so it is not a useful comparison point for this theme specifically) |
| poster | 96 | 96 | 96 | 96 | none major |
| product | 96 | 96 | 96 | 96 | nav row (tag adds +100 top when set) |
| retro-computing | 164 | 172 | 228 | 144 | HUD corner marks, counted in the totals |
| riso | 96 | 96 | 96 | 96 | `.layout-default` sets its own 96px padding directly, not through the shared default (tag adds +114 top when set) |
| sketch | 96 | 96 | 130 | 110 | none major |
| swiss | 96 | 96 | 186 | 96 | top rule + page number band |
| terminal | 96 | 96 | 156 | 120 | blank prompt line, reserved in the top figure |
| transit | 352 | 176 | 176 | 176 | roundel (slide-number circle) plus the shared unmodified layout padding, both reserved in the left figure |
| zine | 96 | 96 | 96 | 96 | none (tag adds +44 top when set) |

Values are total px inset from the canvas edge (a theme's own `.slide[data-layout]`
or `.slide-content` padding plus any extra `.deck-component-root` reserves on
top of it), not just the `.deck-component-root` rule's own numbers. `bauhaus`,
`keynote`, `retro-computing`, `riso`, and `transit` looked deceptively simple
from their `.deck-component-root` rule alone; each turned out to need a real
measurement against the theme's own `default` layout to get right, since a
theme can add its base inset through an outer `.slide[data-layout]` padding,
an un-zeroed shared `.layout-<name>` padding, or a heading-specific margin
rather than only through `.slide-content` or the layout wrapper's own box -
don't infer the safe area from the component's CSS alone.
