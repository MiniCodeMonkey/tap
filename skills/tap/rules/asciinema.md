# Asciinema Terminal Recordings

Tap embeds asciinema terminal recordings as interactive players in slides.

## Syntax

Use an `asciinema` code block with configuration in the info string:

````markdown
```asciinema {src: "./demo.cast"}
```
````

## Configuration Options

All options go in the `{...}` after `asciinema`:

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `src` | string | (required) | Path to .cast file (relative to markdown) |
| `autoPlay` | boolean | `false` | Start playback automatically |
| `loop` | boolean | `false` | Loop recording continuously |
| `speed` | number | `1.0` | Playback speed multiplier |
| `startAt` | number | `0` | Start time in seconds |
| `cols` | number | auto | Terminal columns |
| `rows` | number | auto | Terminal rows |
| `idleTimeLimit` | number | none | Max idle time between frames (seconds) |
| `fit` | string | `"width"` | Fit mode: `width`, `height`, `both`, `none` |
| `poster` | string | none | Poster frame specification |

## Common Patterns

### Auto-playing demo loop

````markdown
```asciinema {src: "./demo.cast", autoPlay: true, loop: true, speed: 2}
```
````

### Slow walkthrough

````markdown
```asciinema {src: "./tutorial.cast", speed: 0.5}
```
````

### Fixed-size terminal

````markdown
```asciinema {src: "./output.cast", cols: 120, rows: 30, fit: "none"}
```
````

## With Layouts

Combine with split-media or two-column layouts:

```markdown
<!--
layout: two-column
-->

# CLI Demo

Walk through the installation process.

|||

```asciinema {src: "./install.cast", autoPlay: true, loop: true}
```
```

## Recording .cast Files

Use the `asciinema` CLI to create recordings:

```bash
# Install asciinema
brew install asciinema

# Record a session
asciinema rec demo.cast

# Record with idle time limit
asciinema rec --idle-time-limit 2 demo.cast
```

## How It Works

1. Parser moves `{...}` config from the info string into the code block body
2. Frontend finds `language-asciinema` code blocks in the rendered HTML
3. Config is parsed and the asciinema-player library loads from CDN
4. Player replaces the code block with an interactive terminal
5. During `tap build`, `.cast` files are copied to `assets/` with content hashing

## Best Practices

- Keep recordings under 60 seconds for presentation flow
- Use `idleTimeLimit` to trim long pauses
- Use `speed: 2` for demos that are mostly typing
- Place `.cast` files next to your markdown file
- Test playback before presenting (requires network for CDN library)
