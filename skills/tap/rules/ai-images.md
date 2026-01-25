# AI Image Generation

Tap integrates with Google Gemini to generate images from text prompts during development.

## Requirements

Set the environment variable:

```bash
export GEMINI_API_KEY=your-api-key
```

## Usage

In the dev server (`tap dev`), press `i` to open the image generator.

### Workflow

1. Select a slide (arrow keys + Enter)
2. Choose "Add new image" or select an existing image to regenerate
3. Enter a descriptive prompt (up to 2000 characters)
4. Wait for generation
5. Image is saved and inserted into markdown

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `i` | Open generator |
| `↑`/`↓` or `k`/`j` | Navigate |
| `Enter` | Select/Submit |
| `Esc` | Cancel |
| `r` | Retry on error |

## Markdown Format

Generated images use this format:

```markdown
<!-- ai-prompt: descriptive prompt here -->
![](images/generated-a1b2c3d4.png)
```

The HTML comment stores the prompt for regeneration. Images are saved to an `images/` directory.

## File Naming

Format: `generated-{hash}.{ext}`

- Hash is derived from image content (SHA256, first 8 chars)
- Extensions: `.png`, `.jpg`, `.webp`, `.gif`

## Writing Prompts

### Be Specific

Include:
- Subject matter
- Style (illustration, photo, diagram, icon)
- Color scheme
- Composition

### Examples

| Weak | Strong |
|------|--------|
| "database" | "minimalist isometric database server illustration, blue and white" |
| "security" | "shield icon with lock, flat design, blue gradient background" |
| "team" | "four professionals at whiteboard, modern office, warm lighting" |

## Regenerating Images

1. Press `i`
2. Select slide with existing AI image
3. Choose the image from the list
4. Edit prompt or submit unchanged
5. New image replaces old (old file deleted)

## Error Handling

| Error | Solution |
|-------|----------|
| Authentication failed | Check GEMINI_API_KEY |
| Rate limit | Wait and retry (press `r`) |
| Content policy | Use different prompt |
| No image generated | Rephrase prompt |
| Network error | Check connection, retry |

## Directory Structure

```
presentation/
├── slides.md
└── images/
    ├── generated-a1b2c3d4.png
    └── manual-image.png
```

## When to Use

Good for:
- Conceptual illustrations
- Abstract backgrounds
- Custom icons
- Quick visual prototyping

Consider Mermaid instead for:
- Technical diagrams
- Flowcharts
- Architecture diagrams
- Precise structural visuals

## Best Practices

- Match image style to presentation theme
- Preview all images before presenting
- Keep prompts for reference/consistency
- Use simple, uncluttered compositions for slides
