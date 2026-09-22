# Themes

Themes control typography, colors, animations, and transitions. Tap ships
`base` plus 20 designed themes.

## Setting a Theme

```yaml
---
theme: terminal
---
```

An unknown theme name falls back to `base` with a warning. Press `t` while
presenting to cycle themes live, or add `?theme=<slug>` to the URL to force
one.

From the command line, `tap theme set <slug> [deck]` writes the same key,
the change the `t` key makes. A deck with no frontmatter gets one; an
unknown slug exits 1 with the list of themes.

## Built-in Themes

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

## Picking a Theme

Two questions: the room (bright room or weak projector: pick a `light`
theme; dark room: a `dark` theme can carry more contrast) and the talk
(match the pitch above to what's actually being presented).

## What Themes Control

- **Typography:** Font families, sizes, weights
- **Colors:** Background, text, accent, syntax highlighting
- **Animations:** Element appearance effects
- **Transitions:** Slide transition styles
- **Spacing:** Padding, margins, layout density
- **Code styling:** Code block appearance

## Customizing Themes

### Color Overrides
```yaml
---
theme: terminal
themeColors:
  accent: "#ff0000"
  background: "#0d0d0d"
---
```

Available keys: `background`, `text`, `muted`, `accent`, `codeBg`

### Custom Theme CSS
```yaml
---
customTheme: "./my-theme.css"
---
```

Loaded after the built-in theme's CSS, so it can override any of its custom
properties. See `docs/reference/theme-porting.md` for the CSS custom
property contract and selector conventions (`[data-theme="..."]`).

## Theme Tokens

Every theme defines the same custom properties: `--bg --fg --muted
--accent --accent-text --surface --font-display --font-body --font-mono
--ease --dur --space-unit --radius --stroke-width`, plus the Shiki token
variables so code colors follow the theme.

Read them, and the theme's illustration style, on the command line:

```bash
tap theme list                       # every theme
tap theme show blueprint             # tokens and illustration style
tap theme show blueprint --json
tap theme show blueprint --prompt    # style brief for an image model
tap theme show deck.md --prompt
```

A deck component reads the same values at run time with `useTheme()`. Put
the `--prompt` brief in front of an image request so generated
illustrations match the theme.
