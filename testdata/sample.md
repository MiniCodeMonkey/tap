---
title: Tap Sample Presentation
theme: paper
author: Tap Team
date: "2026-01-23"
aspectRatio: "16:9"
transition: fade
codeTheme: github-dark
fragments: true
drivers:
  shell:
    timeout: 30
  sqlite:
    connections:
      default:
        database: ":memory:"
  python:
    command: python3
    args: ["-c"]
    timeout: 10
---

<!-- layout: title -->

# Welcome to Tap

Your markdown-based presentation tool

---

<!--
layout: section
transition: slide
-->

## Getting Started

---

<!--
layout: default
notes: |
  This slide introduces the core features.
  Make sure to highlight each bullet point one by one.
-->

## Core Features

Tap is designed for developers who love markdown.

<!-- pause -->

- **Simple** - Write slides in plain markdown
- **Powerful** - Execute live code during presentations

<!-- pause -->

- **Fast** - Hot reload keeps you in the flow
- **Beautiful** - Gorgeous themes out of the box

---

<!--
layout: two-column
notes: Getting started is easy - just follow these steps.
-->

## Two Column Layout

|||

### Left Side

Install Tap with a single command:

```bash
go install github.com/MiniCodeMonkey/tap@latest
```

|||

### Right Side

Create your first presentation:

```bash
tap new my-presentation
tap dev my-presentation.md
```

---

<!--
layout: code-focus
transition: zoom
-->

## Code Focus Layout

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, Tap!")
}
```

---

<!--
layout: quote
notes: A memorable quote to inspire the audience.
-->

> "The best way to predict the future is to invent it."
>
> — Alan Kay

---

<!--
layout: big-stat
-->

# 10x

Faster presentation creation

---

<!-- layout: default -->

## Live Code Execution

Run shell commands directly from your slides:

```bash {driver: shell}
echo "Hello from the shell!"
date
pwd
```

---

<!-- layout: default -->

## SQLite Demo

Query data with SQLite:

```sql {driver: sqlite, connection: default}
CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, role TEXT);
INSERT INTO users (name, role) VALUES ('Alice', 'Admin'), ('Bob', 'User'), ('Charlie', 'User');
SELECT * FROM users;
```

---

<!--
layout: default
background: "#f0f4f8"
-->

## Custom Background

This slide has a custom background color set via the background directive.

Backgrounds can be:
- Colors (hex or named)
- Gradients (`linear-gradient(...)`)
- Images (`image.png`)

---

<!--
layout: default
transition: push
notes: |
  Here are some additional notes for the presenter.

  - Remember to demonstrate the fragment feature
  - Show how the pause directive creates incremental reveals
-->

## Fragment Reveals

Fragments let you reveal content incrementally:

<!-- pause -->

1. First point appears

<!-- pause -->

2. Then the second

<!-- pause -->

3. And finally the third

---

<!-- layout: section -->

## Code Highlighting

---

<!-- layout: default -->

## Syntax Highlighting

Tap supports syntax highlighting for many languages:

```typescript
interface Presentation {
  title: string;
  slides: Slide[];
  config: Config;
}

function createPresentation(title: string): Presentation {
  return {
    title,
    slides: [],
    config: { theme: 'paper' }
  };
}
```

---

<!-- layout: default -->

## Python with Custom Driver

Using the custom Python driver defined in frontmatter:

```python {driver: python}
for i in range(5):
    print(f"Count: {i}")
```

---

<!--
layout: three-column
notes: Comparing different options side by side.
-->

## Feature Comparison

|||

### Free Tier

- 5 presentations
- Basic themes
- Community support

|||

### Pro Tier

- Unlimited presentations
- All themes
- Priority support
- Custom branding

|||

### Enterprise

- Everything in Pro
- Self-hosted option
- SSO integration
- Dedicated support

---

<!-- layout: default -->

## Markdown Features

Tap supports all standard markdown features:

### Lists

- Unordered item 1
- Unordered item 2
  - Nested item

1. Ordered item 1
2. Ordered item 2

### Task Lists

- [x] Complete feature
- [ ] Work in progress
- [ ] Not started

---

<!-- layout: default -->

## Tables

| Feature | Tap | Competitors |
|---------|-----|-------------|
| Markdown | Yes | Some |
| Live Code | Yes | No |
| Hot Reload | Yes | Some |
| CLI Tool | Yes | No |

---

<!-- layout: default -->

## Links and Emphasis

Visit [the Tap documentation](https://tap.sh/docs) for more information.

**Bold text** for emphasis, *italic text* for subtle emphasis, and ~~strikethrough~~ for corrections.

Inline `code` is also supported for technical terms.

---

<!--
layout: cover
background: "https://images.unsplash.com/photo-1517694712202-14dd9538aa97"
notes: Full-bleed background image with text overlay.
-->

# Cover Layout

Full-bleed background images for impact

---

<!--
layout: sidebar
-->

## Sidebar Layout

||| main

The main content area takes up most of the space.

Perfect for content that needs some supporting information on the side.

- Point one
- Point two
- Point three

||| sidebar

### Resources

- Documentation
- API Reference
- Examples
- Support

---

<!--
layout: split-media
-->

## Split Media

||| content

### Image + Text

This layout places an image alongside your content.

Great for:
- Product screenshots
- Diagrams
- Photos

||| media

![Placeholder Image](https://via.placeholder.com/800x600)

---

<!-- layout: blank -->

<div style="display: flex; justify-content: center; align-items: center; height: 100%;">
  <h1 style="font-size: 12rem; color: #7C3AED;">Custom HTML</h1>
</div>

---

<!-- layout: section -->

## Transitions

---

<!--
layout: default
transition: none
-->

## No Transition

This slide has no transition animation.

---

<!--
layout: default
transition: fade
-->

## Fade Transition

This slide fades in and out.

---

<!--
layout: default
transition: slide
-->

## Slide Transition

This slide slides in from the right.

---

<!--
layout: default
transition: push
-->

## Push Transition

This slide pushes the previous one out.

---

<!--
layout: default
transition: zoom
-->

## Zoom Transition

This slide zooms in from the center.

---

<!--
scroll: true
scroll-speed: 500
-->

## Scroll Test Slide

```typescript
// This is a long code block to test scroll functionality
interface DataProcessor {
  id: number;
  name: string;
  config: ProcessorConfig;
}

interface ProcessorConfig {
  timeout: number;
  retries: number;
  batchSize: number;
}

class DataProcessingService {
  private processors: Map<string, DataProcessor> = new Map();

  async process(data: unknown[]): Promise<void> {
    for (const item of data) {
      await this.processItem(item);
    }
  }

  private async processItem(item: unknown): Promise<void> {
    console.log('Processing item:', item);
  }

  registerProcessor(name: string, processor: DataProcessor): void {
    this.processors.set(name, processor);
  }

  getProcessor(name: string): DataProcessor | undefined {
    return this.processors.get(name);
  }
}

// Additional lines to ensure content exceeds viewport
function createProcessor(id: number, name: string): DataProcessor {
  return {
    id,
    name,
    config: {
      timeout: 5000,
      retries: 3,
      batchSize: 100
    }
  };
}

const service = new DataProcessingService();
service.registerProcessor('default', createProcessor(1, 'Default'));
service.registerProcessor('fast', createProcessor(2, 'Fast'));
service.registerProcessor('reliable', createProcessor(3, 'Reliable'));

export { DataProcessingService, createProcessor };
```

---

## After Scroll Slide

This slide follows the scroll test slide.

---

<!-- layout: title -->

# Thank You!

Questions?

**Press S for presenter view**
