---
title: Asciinema Recordings
---

# Asciinema Terminal Recordings

Tap supports embedding [asciinema](https://asciinema.org/) terminal recordings directly in your slides. Recordings play back as interactive terminal sessions with controls for play/pause and speed adjustment.

## Basic Usage

Add an `asciinema` code block with a `src` pointing to your `.cast` file:

````markdown
```asciinema {src: "./demo.cast"}
```
````

The recording renders as an interactive player when your slide is displayed.

## Configuration

All configuration goes in the `{...}` after `asciinema`:

````markdown
```asciinema {src: "./demo.cast", autoPlay: true, loop: true, speed: 2}
```
````

### Available Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `src` | string | *required* | Path to `.cast` file (relative to your markdown) |
| `autoPlay` | boolean | `false` | Start playback automatically when the slide appears |
| `loop` | boolean | `false` | Loop the recording continuously |
| `speed` | number | `1.0` | Playback speed multiplier (e.g., `2` for double speed) |
| `startAt` | number | `0` | Start playback from this time (in seconds) |
| `cols` | number | auto | Override terminal columns |
| `rows` | number | auto | Override terminal rows |
| `idleTimeLimit` | number | none | Cap idle time between frames (seconds) |
| `fit` | string | `"width"` | How the player fits: `width`, `height`, `both`, or `none` |
| `poster` | string | none | Frame to show before playback starts |

## Examples

### Auto-Playing Demo Loop

Perfect for title slides or background demos:

````markdown
```asciinema {src: "./demo.cast", autoPlay: true, loop: true, speed: 2}
```
````

### Slow Tutorial Walkthrough

For step-by-step explanations:

````markdown
```asciinema {src: "./tutorial.cast", speed: 0.5}
```
````

### Trimming Idle Time

Remove long pauses between commands:

````markdown
```asciinema {src: "./session.cast", idleTimeLimit: 2}
```
````

### Fixed Terminal Size

Force a specific terminal size:

````markdown
```asciinema {src: "./output.cast", cols: 120, rows: 30, fit: "none"}
```
````

## With Layouts

Recordings work well with multi-column layouts:

```markdown
<!--
layout: two-column
-->

# Installation

Follow along with the installation process step by step.

|||

```asciinema {src: "./install.cast", autoPlay: true, loop: true}
```
```

## Recording .cast Files

Use the [asciinema CLI](https://docs.asciinema.org/getting-started/) to create recordings:

```bash
# Install asciinema
brew install asciinema

# Record a terminal session
asciinema rec demo.cast

# Record with a 2-second idle time cap
asciinema rec --idle-time-limit 2 demo.cast
```

Place `.cast` files next to your markdown file or in a subdirectory. Tap resolves paths relative to your presentation file.

## Player Controls

When hovering over a recording, controls appear:

- **Play/Pause** button to start or stop playback
- **Speed** button to cycle through speeds (0.5x, 1x, 1.5x, 2x, 3x)

The player also supports the built-in asciinema-player controls at the bottom of the terminal.

## Building and Exporting

When you run `tap build`, `.cast` files are automatically:
- Resolved from their relative paths
- Copied to the `assets/` directory with content hashing for cache busting
- Path-rewritten in the output HTML

No additional configuration is needed.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Blank player | Check that the `.cast` file path is correct relative to your markdown |
| Player not loading | The asciinema-player library loads from CDN; check your network connection |
| Recording looks wrong | Try setting explicit `cols` and `rows` to match your recording |
| Playback too fast/slow | Adjust the `speed` option or use `idleTimeLimit` to trim pauses |

## Tips

- Keep recordings under 60 seconds for presentation flow
- Use `speed: 2` for demos that are mostly typing
- Use `idleTimeLimit` to trim long pauses automatically
- Use `autoPlay: true, loop: true` for ambient background demos
- Test playback before presenting (the player library loads from CDN)

## Next Steps

- [Code Blocks](/guide/code-blocks) - Syntax highlighting for static code
- [Live Code Execution](/guide/live-code-execution) - Run code directly in slides
- [Layouts](/guide/layouts) - Position recordings alongside content
