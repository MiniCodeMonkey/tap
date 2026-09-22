---
title: Themes
---

# Themes

Themes control the visual appearance of your presentation: typography,
colors, animations, transitions, and spacing. Tap ships `base` plus 20
designed themes, each built for a particular kind of talk.

## Setting a Theme

Set the theme in your presentation's frontmatter:

```yaml
---
theme: terminal
---
```

An unknown theme name falls back to `base` with a warning, so a deck
written against an older theme name still builds.

## Switching Themes Live

While presenting, press `t` to cycle through every installed theme without
touching the frontmatter, handy for trying a room's projector against a few
options before you start. To force a specific theme for a link or a
recording, add `?theme=<slug>` to the URL - it beats the deck's frontmatter.

## Picking a Theme

Two questions narrow it down fast:

- **The room.** A bright room or a projector with poor black levels washes
  out dark themes; pick a `light`-polarity theme. A dark room lets a
  `dark`-polarity theme's contrast do more work.
- **The talk.** Each theme below has a one-line pitch describing the kind of
  talk it was built for - match the pitch to your talk, not just the vibe.

## Built-in Themes

| Theme | Polarity | Pitch |
|-------|----------|-------|
| `base` | light | A plain, readable default theme with no strong identity, used when a deck names no theme or names one that doesn't exist. |
| `terminal` | dark | For infrastructure and live-coding talks where the audience lives in a shell and the deck should feel like one long tmux session. |
| `product` | light | A launch-day talk for a developer tool, where every slide should feel like the landing page of something you want to install. |
| `swiss` | light | An argument-driven engineering talk set like a Zurich concert poster: grid, grotesk, one red. |
| `newsprint` | light | A war story or postmortem told as front-page news, where every slide is a headline and every number is a front-page figure. |
| `zine` | light | An opinionated, scrappy talk with a point to argue: cut, pasted, and photocopied the night before, and proud of it. |
| `poster` | light | For a loud, fast, opinionated talk in the Takahashi style, where every slide is a gig poster and the words do all the work. |
| `blueprint` | dark | For architecture and systems talks where every slide is a sheet from the drawing set and the diagrams are the argument. |
| `riso` | light | A warm community-conference talk that should feel like a two-ink print pulled off the drum that morning. |
| `retro-computing` | light | A nostalgic, slightly mischievous systems talk told from a 1990 desktop, where every idea opens in its own window. |
| `paperback` | light | A story-led conference talk that wants the warmth and quiet authority of a mid-century paperback series. |
| `keynote` | dark | A big-room launch talk where one idea at a time lands on a dark stage, lit from above. |
| `editorial` | light | A story-led talk told like a long-read magazine feature, with drop caps, pull quotes, and figures. |
| `observatory` | dark | A data-heavy infrastructure talk, charted like a night sky: systems as bodies, readings taken through a reticle. |
| `arcade` | dark | A high-energy war-story talk framed as a game you can win, built to stay legible on a washed-out projector. |
| `isometric` | light | A friendly architecture walkthrough where the infrastructure itself is the cast: code, tables, and numbers become chunky little solids on a warm floor. |
| `ink` | light | A calm, reflective talk about hard-won lessons, where each slide holds one thought and a lot of silence around it. |
| `lab-notebook` | light | An evidence-first engineering talk where every claim arrives as a numbered figure and the speaker's red pen does the arguing. |
| `bauhaus` | light | A bold, opinionated talk about first principles, where every idea is reduced to a circle, a triangle, or a square. |
| `sketch` | light | For explainer talks that build an idea step by step, as if the speaker were drawing it on a whiteboard in front of you. |
| `transit` | light | An architecture or infrastructure talk that walks the audience through a system the way a metro map walks a city. |

## Reading a Theme From the Command Line

Two commands expose everything a theme declares, for a person writing a
component or an LLM writing a deck.

```bash
tap theme list              # slug, name, polarity, and pitch for all 21
tap theme list --json
tap theme show blueprint    # tokens and illustration style
tap theme show blueprint --json
tap theme show slides.md   # the theme that deck's frontmatter names
```

## Theme Tokens

Every theme defines the same set of CSS custom properties on its root
block. They are what a custom stylesheet, a deck component, or a
hand-written theme should build on, instead of hard-coded colors:

| Token | Meaning |
|-------|---------|
| `--bg` | Slide background |
| `--fg` | Body text |
| `--muted` | Secondary text and inactive states |
| `--accent` | The one highlight color, used as a **fill** |
| `--accent-text` | The accent in a form that is readable as **text** on `--bg` |
| `--accent-2` | A second fill; equals `--accent-text` in themes with no second accent |
| `--surface` | Panels, cards, and code backgrounds |
| `--status-ok` | A healthy or passing state, as a fill |
| `--status-warn` | A warning state, as a fill |
| `--status-error` | A failed state, as a fill |
| `--font-display` | Heading family |
| `--font-body` | Body family |
| `--font-mono` | Code family |
| `--ease` | Animation easing |
| `--dur` | Animation duration |
| `--space-unit` | The theme's base spacing step |
| `--radius` | Corner radius |
| `--stroke-width` | Border and line weight |

The three status colors are each readable as a fill at a contrast of at
least 3:1 against `--bg`, and any two of them differ by a contrast ratio of
at least 1.35, so they are stepped apart by lightness and not only by hue.
A filled dot, bar, or badge in one of them is visible in every theme.

They carry meaning, so use them only where the color says something: a
health state, a passed or failed check, a threshold crossed. Something
merely inactive stays `--muted`. And **never signal status by color
alone**: pair it with a label, an icon, or a shape, so it still reads for a
colorblind viewer and on a washed-out projector. The luminance separation
exists to keep that pairing legible when the hue is lost.

Themes also set the Shiki CSS variables, so code colors follow the theme.

`tap theme show <slug>` prints the current values:

```
$ tap theme show blueprint
Blueprint (blueprint)
  Polarity: dark
  Pitch: For architecture and systems talks where every slide is a sheet from the drawing set and the diagrams are the argument.

  Colors:
    background (bg): #0f3a75
    foreground (fg): #f5f9ff
    muted text (muted): #c0d6f3
    accent (accent): #ffd447
    accent (as text) (accent-text): #ffd447
    surface (surface): #174688
  ...
```

`tap theme show <slug> --json` lists the same values under
`tokens.colors`, including `accent2`, `statusOk`, `statusWarn`, and
`statusError`, plus every raw variable under `tokens.other`.

A deck component reads the same values at run time with `useTheme()`. See
[Custom Components](/guide/custom-components#theme-tokens).

::: warning `--accent` is a fill, not a text color
Several themes pick an accent that cannot be read as text on their own
background: `zine`'s is `#ffe600` on an `#e8e5dd` page. Paint shapes with
`--accent`; use `--accent-text` for text and thin lines that sit on
`--bg`. The same rule applies in CSS and in a component. See
[The color rule](/reference/components-reference#the-color-rule).
:::

## Illustration Style

Each theme also declares how an illustration should look next to it, in
seven short fields: `medium`, `line`, `shapes`, `texture`, `palette_use`,
`mood`, and `avoid`. `tap theme show <slug>` prints them under
**Illustration**.

`tap theme show <slug> --prompt` turns the same fields into a plain-text
style brief written for an image model. It names the palette, and the
status colors as colors to reach for only where meaning requires them:

```
$ tap theme show terminal --prompt
Illustration style for the "Terminal" slide theme: flat vector on near-black,
like a TUI with amber and mint green accents. Use only this palette:
background #0f1a16, foreground #dcebe0, accent #ffb84d, muted #9db8a8,
surface #1d2b25. Status colors, only where meaning requires them: ok #23b561,
warn #bd7100, error #d60600. Palette use: dark background dominant, amber
accent and mint green accent used one or two per image. Line: 1 to 2px
strokes, monospace-grid aligned, square caps.
...
```

Put that text in front of your own image request and the result sits on the
slide instead of fighting it. See
[AI Image Generation](/guide/ai-images#matching-the-theme).

## What Themes Control

Each theme defines:

| Aspect | Description |
|--------|-------------|
| **Typography** | Font families, sizes, weights, and line heights for headings, body text, and code |
| **Colors** | Background colors, text colors, accent colors, and syntax highlighting palette |
| **Animations** | How elements appear on slides |
| **Transitions** | How slides transition between each other |
| **Spacing** | Padding, margins, and overall layout density |
| **Code styling** | Code block appearance, syntax highlighting colors, and font sizing |

## Customizing Themes

### Color Overrides

Override a handful of colors on top of a built-in theme with `themeColors`
in frontmatter:

```yaml
---
theme: terminal
themeColors:
  accent: "#ff0000"
  background: "#0d0d0d"
---
```

Available color keys: `background`, `text`, `muted`, `accent`, `codeBg`.

### Custom Theme CSS

For complete customization, point `customTheme` at your own CSS file:

```yaml
---
customTheme: "./my-theme.css"
---
```

Tap serves that file and loads it after the built-in theme's CSS, so it can
override any of the theme's custom properties or rules. See
`docs/reference/theme-porting.md` for the full set of CSS custom properties
a theme defines and the selector conventions (`[data-theme="..."]`) to
follow, and for lessons learned porting the 20 built-in themes - the same
pitfalls apply to a hand-written custom theme.
