---
title: Images & Media
---

# Images & Media

Tap supports images and media files to enrich your presentations. This guide covers how to add, size, and position visual content in your slides.

## Adding Images

### Relative Paths

Reference images relative to your markdown file's location:

```markdown
![Architecture diagram](./images/architecture.png)
```

Tap resolves paths from the markdown file's directory, so if your presentation is at `slides/demo.md` and you reference `./images/logo.png`, Tap looks for `slides/images/logo.png`.

::: tip Organize Your Assets
Keep images in an `images/` or `assets/` folder alongside your markdown file for clean project structure:
```
presentation/
├── slides.md
└── images/
    ├── logo.png
    ├── diagram.svg
    └── screenshot.png
```
:::

### Absolute URLs

Reference images from external URLs:

```markdown
![Company Logo](https://example.com/logo.png)
```

External images are fetched at render time. For reliability in presentations, consider downloading important images locally.

::: warning Network Dependency
External images require network access during your presentation. For offline presentations or unreliable connections, use local images instead.
:::

## Supported Formats

Tap supports common web image formats:

| Format | Extension | Best For |
|--------|-----------|----------|
| PNG | `.png` | Screenshots, diagrams, logos with transparency |
| JPEG | `.jpg`, `.jpeg` | Photos, complex images |
| SVG | `.svg` | Icons, diagrams, scalable graphics |
| GIF | `.gif` | Simple animations |
| WebP | `.webp` | Modern format with good compression |

::: tip SVG for Diagrams
SVG images scale perfectly at any size and look crisp on high-DPI displays. Use SVG for diagrams, icons, and any graphics that need to scale.
:::

## Image Sizing

Control image dimensions using the width attribute:

```markdown
![Small logo](./logo.png){width=100px}

![Half-width diagram](./diagram.svg){width=50%}

![Full-width hero](./hero.jpg){width=100%}
```

### Sizing Options

| Syntax | Result |
|--------|--------|
| `{width=200px}` | Fixed width in pixels |
| `{width=50%}` | Percentage of slide width |
| `{width=auto}` | Natural image size |

Percentage widths are relative to the content area of your slide, making them responsive to different screen sizes.

::: tip Consistent Sizing
Use percentage widths for images that should adapt to screen size, and pixel widths for elements that need exact dimensions (like icons or logos).
:::

## Image Positioning

Control where images appear on your slide using the position attribute:

```markdown
![Left-aligned](./image.png){position=left}

![Centered (default)](./image.png){position=center}

![Right-aligned](./image.png){position=right}
```

### Position Options

| Syntax | Result |
|--------|--------|
| `{position=left}` | Align image to the left |
| `{position=center}` | Center the image (default) |
| `{position=right}` | Align image to the right |

### Combining Attributes

Combine sizing and positioning:

```markdown
![Logo](./logo.png){width=150px position=right}

![Diagram](./diagram.svg){width=80% position=center}
```

## Background Images

Use the `cover` layout to display full-bleed background images:

```markdown
---
title: My Presentation
---

# Opening Slide

Regular content here.

---

<!--
layout: cover
background: ./images/hero-background.jpg
-->

# Big Statement

Text overlaid on the background image.
```

### Cover Layout Options

The cover layout supports additional background options:

```yaml
<!--
layout: cover
background: ./images/keynote-bg.jpg
backgroundOpacity: 0.7
backgroundPosition: center
-->
```

| Option | Values | Description |
|--------|--------|-------------|
| `background` | Path or URL | The background image source |
| `backgroundOpacity` | `0.0` - `1.0` | Dim the background (useful for text readability) |
| `backgroundPosition` | `center`, `top`, `bottom` | Position the background image |

::: tip Text Readability
When using background images with text overlay, use `backgroundOpacity: 0.5` or lower, or choose images with dark/neutral areas for text placement.
:::

## Build Behavior

When you run `tap build`, Tap processes your images:

1. **Local images** are copied to the `dist/` output directory
2. **Relative paths** are rewritten to work in the built output
3. **Directory structure** is preserved (e.g., `images/logo.png` → `dist/images/logo.png`)
4. **External URLs** are left unchanged

The built presentation is fully self-contained (except for external URLs), ready to be deployed to any static host.

### Output Structure

```
dist/
├── index.html
├── images/
│   ├── logo.png
│   ├── diagram.svg
│   └── screenshot.jpg
└── assets/
    └── (bundled CSS/JS)
```

## Missing Image Handling

Tap handles missing images gracefully:

| Scenario | Behavior |
|----------|----------|
| Missing local file | Warning in console, placeholder shown |
| Failed external URL | Error logged, alt text displayed |
| Invalid format | Warning with file path |

During development (`tap dev`), you'll see warnings in the console for missing images, allowing you to fix issues before presenting.

::: tip Verify Before Presenting
Run `tap build` before important presentations to catch any missing image errors. The build process validates all local image references.
:::

## Best Practices

### Performance

- **Optimize image file sizes** before adding to your presentation
- **Use appropriate formats**: JPEG for photos, PNG for screenshots, SVG for diagrams
- **Resize images** to reasonable dimensions (full-screen images don't need to be 4K)

### Accessibility

- **Always include alt text** describing the image content
- **Don't rely on images alone** to convey critical information
- **Use sufficient contrast** when overlaying text on images

### Organization

- **Use consistent naming** for image files (lowercase, hyphens)
- **Group images by topic** or slide in subdirectories for larger presentations
- **Keep originals** separate if you're optimizing copies

## Quick Reference

| Feature | Syntax | Example |
|---------|--------|---------|
| Basic image | `![alt](path)` | `![Logo](./logo.png)` |
| External URL | `![alt](url)` | `![Logo](https://...)` |
| Width (pixels) | `{width=Npx}` | `{width=200px}` |
| Width (percent) | `{width=N%}` | `{width=50%}` |
| Position | `{position=X}` | `{position=right}` |
| Combined | `{attr1 attr2}` | `{width=50% position=left}` |
| Background | `layout: cover` | In directive block |

## Next Steps

- Learn about [Layouts](/guide/layouts) for more ways to position content
- Explore [Building & Export](/guide/building-export) to understand the output process
- See [Writing Slides](/guide/writing-slides) for other markdown features
