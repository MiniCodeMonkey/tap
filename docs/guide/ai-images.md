---
title: AI Image Generation
---

# AI Image Generation

Tap integrates with Google's Gemini API to generate images directly from text prompts. Create custom visuals for your presentations without leaving your workflow.

## Setup

Set your Gemini API key as an environment variable:

```bash
export GEMINI_API_KEY=your-api-key-here
```

Get an API key from [Google AI Studio](https://aistudio.google.com/apikey).

::: tip Environment Files
Add the key to your shell profile (`.bashrc`, `.zshrc`) or a project `.env` file for persistence.
:::

## Using the Image Generator

### Opening the Generator

While running `tap dev`, press `i` to open the image generator.

### Workflow

1. **Select a slide** - Choose which slide to add the image to using arrow keys
2. **Choose action** - Add a new image or regenerate an existing one
3. **Enter prompt** - Describe the image you want (up to 2000 characters)
4. **Wait for generation** - The image generates in a few seconds
5. **Done** - The image is saved and inserted into your markdown

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `i` | Open image generator |
| `↑` / `k` | Navigate up |
| `↓` / `j` | Navigate down |
| `Enter` | Select / Submit prompt |
| `Esc` | Cancel / Go back |
| `r` | Retry on error |

## Markdown Format

Generated images are stored with their prompt as metadata:

```markdown
<!-- ai-prompt: a minimalist illustration of a rocket launching -->
![](images/generated-a1b2c3d4.png)
```

The HTML comment preserves the prompt for regeneration. The image file is saved to an `images/` directory alongside your markdown file.

## Regenerating Images

To regenerate an existing AI image:

1. Press `i` to open the generator
2. Select the slide containing the image
3. Choose the image to regenerate from the list
4. Edit the prompt if desired, or submit to regenerate with the same prompt
5. The new image replaces the old one (old file is deleted)

## Writing Effective Prompts

### Be Specific

Vague prompts produce unpredictable results. Include:
- Subject matter
- Style (illustration, photo, diagram)
- Color scheme
- Composition

| Less Effective | More Effective |
|----------------|----------------|
| "a database" | "a minimalist isometric illustration of database servers in blue and white" |
| "teamwork" | "four diverse professionals collaborating around a whiteboard, modern office, warm lighting" |
| "security" | "a shield icon with a lock, flat design, corporate blue gradient background" |

### Match Your Theme

Consider your presentation's visual style when writing prompts. For a noir theme, request dark backgrounds. For paper theme, request neutral tones.

### Keep It Presentation-Ready

Generated images work best when they're:
- Simple and uncluttered
- High contrast for visibility
- Relevant to slide content

## File Management

### Directory Structure

```
presentation/
├── slides.md
└── images/
    ├── generated-a1b2c3d4.png
    ├── generated-e5f6g7h8.png
    └── logo.png  (your own images)
```

### Filename Format

Generated images use the format `generated-{hash}.{ext}` where the hash is derived from the image content. This ensures:
- Unique filenames for different images
- Same content always produces the same filename
- Easy identification of AI-generated vs. manual images

### Supported Formats

The API returns images in standard web formats:
- PNG (most common)
- JPEG
- WebP
- GIF

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| Authentication failed | Invalid or missing API key | Check `GEMINI_API_KEY` is set correctly |
| Rate limit exceeded | Too many requests | Wait a moment and retry |
| Content policy | Prompt violated guidelines | Try a different prompt |
| No image generated | API couldn't produce image | Rephrase your prompt |
| Network error | Connection issue | Check internet and retry |

Press `r` to retry after an error, or `Esc` to cancel.

## Best Practices

### Organize Prompts

For presentations with many AI images, keep a reference of your prompts in a separate file. This makes it easier to maintain visual consistency.

### Review Before Presenting

AI-generated images can occasionally contain artifacts or unexpected elements. Always preview your slides before presenting.

### Consider Alternatives

AI generation is great for:
- Conceptual illustrations
- Abstract backgrounds
- Quick prototyping

For precise technical diagrams, consider [Mermaid diagrams](/guide/mermaid-diagrams) instead.

## Quick Reference

| Task | How |
|------|-----|
| Generate new image | Press `i` → Select slide → "Add new image" → Enter prompt |
| Regenerate image | Press `i` → Select slide → Choose existing image → Edit/submit prompt |
| View prompt | Check the `<!-- ai-prompt: ... -->` comment in markdown |
| Delete AI image | Remove the comment and image line from markdown, delete file manually |

## Next Steps

- Learn about [Images & Media](/guide/images-media) for sizing and positioning
- Explore [Mermaid Diagrams](/guide/mermaid-diagrams) for technical diagrams
- See [Layouts](/guide/layouts) for image placement options
