# Tap Documentation Website - PRD

## Overview

Build the documentation website for Tap at tap.sh using VitePress. The site combines a striking landing page with comprehensive documentation, inspired by Linear's clean, minimal docs aesthetic.

**Hosting:** Netlify
**Domain:** tap.sh

---

## Philosophy Section

The docs should open with Tap's core philosophy. This sets the tone and helps developers immediately understand if Tap is for them.

### Core Principles

**1. Slides shouldn't be harder than code**
You already write markdown every day. READMEs, docs, notes. Tap presentations are just markdown files. No proprietary formats, no GUI drag-and-drop, no "slide master" templates. Write text, get slides.

**2. Beautiful by default**
Open any presentation tool and the first slide looks like a ransom note. Tap ships with 5 opinionated, distinctive themes that look great out of the box. You don't need design skills—just pick a theme and present.

**3. Batteries included, zero config**
`tap dev slides.md` and you're presenting. Hot reload, keyboard navigation, presenter mode, syntax highlighting—it all just works. No plugins to install, no config files to create, no build step to configure.

**4. Code is a first-class citizen**
Tap is built for technical presentations. Syntax highlighting with VS Code themes. Line highlighting. Code diffs. Step-by-step reveals. And the killer feature: live code execution. Run SQL queries, shell commands, or any language directly in your slides.

**5. Progressive complexity**
Simple things are simple. A basic presentation is 100% markdown—no HTML, no CSS, no JavaScript. But when you need more power, it's there. Custom layouts, custom animations, custom themes. You graduate to complexity only when you need it.

**6. Single binary, works everywhere**
One `brew install` and you're done. No Node.js runtime, no Python dependencies, no Docker containers. Tap is a single binary that runs on macOS, Linux, and Windows.

---

## Landing Page

### Hero Section

**Headline:** "Presentations for developers"

**Subheadline:** "Markdown slides with live code, beautiful themes, and zero config."

**Elements:**
- Bold, clean typography (Linear-inspired)
- Animated terminal showing `tap dev` workflow
- Install command with copy button: `brew install tap-slides`
- Primary CTA: "Get Started" → /getting-started
- Secondary CTA: "View on GitHub" → repo

### Value Props

Three cards, minimal design:

| Prop | Description |
|------|-------------|
| **Markdown-first** | Write slides in the format you already know. Version control friendly, editor agnostic. |
| **Live code execution** | Run SQL, shell, or any language. Results render directly on your slides. |
| **Single binary** | No runtime dependencies. Install once, works everywhere. |

### Code Example

Side-by-side:
- Left: Simple markdown source (syntax highlighted)
- Right: Visual representation of rendered slide (stylized mockup)

```markdown
---
theme: phosphor
---

# Database Demo

```sql {driver: "sqlite"}
SELECT name, role FROM team
WHERE department = 'Engineering';
```

The query runs live during your presentation.
```

### Themes Preview

Grid of 5 theme cards:
- Theme name
- 2-3 word description
- Abstract visual representation (colored blocks/shapes suggesting the aesthetic)

| Theme | Vibe |
|-------|------|
| Minimal | Clean, spacious |
| Gradient | Modern, colorful |
| Terminal | Hacker aesthetic |
| Brutalist | Bold, geometric |
| Keynote | Professional, polished |

### Quick Start

```
$ brew install tap-slides
$ tap new my-talk.md
$ tap dev my-talk.md
  → Running at http://localhost:3000
```

### Footer

- GitHub link
- MIT License
- "Made for developers who present"

---

## Documentation Structure

### Information Architecture

```
/                       # Landing page
/getting-started        # Quick start guide
/guide/                 # Conceptual guides
/reference/             # Lookup documentation
/examples/              # Full presentation examples
```

### Sidebar Navigation

```
Getting Started

Guide
  ├── Writing Slides
  ├── Layouts
  ├── Themes
  ├── Animations & Transitions
  ├── Code Blocks
  ├── Live Code Execution
  ├── Presenter Mode
  ├── Images & Media
  └── Building & Export

Reference
  ├── CLI Commands
  ├── Frontmatter Options
  ├── Slide Directives
  ├── Layouts Reference
  ├── Drivers
  └── Keyboard Shortcuts

Examples
```

---

## Pages - Detailed

### Getting Started

**Status:** Write now (core workflow is stable)

Content:
1. Install Tap (brew, go install, binary download)
2. Create first presentation: `tap new`
3. Start dev server: `tap dev`
4. Basic slide syntax (frontmatter, slide separators)
5. Add a few slides manually
6. Build for production: `tap build`
7. Next steps (links to Guide)

### Guide: Writing Slides

**Status:** Write now

- Frontmatter basics (title, theme)
- Slide separators (`---`)
- Markdown syntax supported (via goldmark)
- Local directives (HTML comment YAML blocks)
- Speaker notes

### Guide: Layouts

**Status:** Write now (list is defined in spec)

- Default layout behavior
- How to specify: `layout: name` in directive block
- List all 10+ layouts with descriptions:
  - title, section, two-column, three-column, code-focus, big-stat, quote, cover, sidebar, split-media, blank

### Guide: Themes

**Status:** Write now (list is defined)

- The 5 built-in themes
- Setting theme in frontmatter
- What themes control (typography, colors, animations, transitions)
- Customizing themes (stub - details TBD)

### Guide: Animations & Transitions

**Status:** Write now (concepts defined)

- Slide transitions (none, fade, slide, push, zoom)
- Setting globally vs per-slide
- Fragments/incremental reveals
- `<!-- pause -->` syntax
- `fragments: true` for lists

### Guide: Code Blocks

**Status:** Write now

- Syntax highlighting (Shiki)
- Specifying language
- Line highlighting (`{1,3-5}` syntax)
- Code diffs
- Multi-step reveals
- Font size configuration

### Guide: Live Code Execution

**Status:** Write now (architecture defined)

- What it is and why it's useful
- Driver concept
- Code block syntax: `{driver: "sqlite"}`
- Built-in drivers: SQLite, MySQL, PostgreSQL, Shell
- Connection configuration in frontmatter
- Environment variables for credentials
- Timeout protection
- Error handling
- Note: Only works in `tap dev`, not static builds

### Guide: Presenter Mode

**Status:** Write now (fully specced)

- What it is (dual-window, synced)
- URLs: `/` vs `/presenter`
- Presenter view features (notes, timer, next slide)
- Cross-device usage (iPad as controller)
- QR code for easy access
- Password protection
- Keyboard shortcuts

### Guide: Images & Media

**Status:** Write now

- Relative paths (resolved from markdown file)
- Absolute URLs
- Supported formats
- Sizing: `{width=50%}` syntax
- Positioning: `{position=left}`
- Background images via `layout: cover`
- Build behavior (images copied to dist/)
- Missing image handling (graceful error)

### Guide: Building & Export

**Status:** Write now

- `tap build` command
- Output structure (dist/)
- Deploying to static hosts
- `tap serve` for preview
- PDF export via `tap pdf`
- PDF options (slides only, notes, both)

### Reference: CLI Commands

**Status:** Write now (commands defined in spec)

| Command | Description |
|---------|-------------|
| `tap new` | Create new presentation |
| `tap dev <file>` | Start dev server with hot reload |
| `tap build <file>` | Generate static HTML bundle |
| `tap serve [dir]` | Serve built presentation |
| `tap pdf <file>` | Export to PDF |
| `tap add [file]` | Add slide via TUI |

Include all flags for each command.

### Reference: Frontmatter Options

**Status:** Write now (defined in spec)

Full table of all frontmatter options:
- title, theme, author, date
- aspectRatio, transition, codeTheme
- fragments
- drivers configuration

### Reference: Slide Directives

**Status:** Write now

All local directive options:
- layout, transition, fragments
- background, notes

### Reference: Layouts

**Status:** Stub now, expand later

List all layouts with:
- Name
- Description
- When to use
- Example (stub)

### Reference: Drivers

**Status:** Write now (architecture defined)

- SQLite driver
- MySQL driver
- PostgreSQL driver
- Shell driver
- Configuration format
- Custom drivers (stub)

### Reference: Keyboard Shortcuts

**Status:** Write now

| Key | Action |
|-----|--------|
| `→` / `Space` | Next slide |
| `←` | Previous slide |
| `S` | Open presenter view |
| `O` | Toggle overview |
| `R` | Reset timer (presenter) |

### Examples

**Status:** Stub now

Placeholder pages for:
- Tech Talk (conference presentation)
- Demo Day (product demo with live code)
- Workshop (teaching format)
- Lightning Talk (5-minute format)

Each will eventually have full markdown source + explanation.

---

## Technical Implementation

### VitePress Setup

```
docs/
├── .vitepress/
│   ├── config.ts
│   └── theme/
│       ├── index.ts
│       ├── style.css
│       └── components/
│           ├── HomeHero.vue
│           ├── HomeFeatures.vue
│           ├── HomeThemes.vue
│           ├── HomeCode.vue
│           └── CopyButton.vue
├── public/
│   └── favicon.ico
├── index.md
├── getting-started.md
├── guide/
│   └── *.md
├── reference/
│   └── *.md
└── examples/
    └── *.md
```

### Design Direction

**Reference:** Linear docs (linear.app/docs)
- Clean, minimal
- Lots of whitespace
- Subtle animations
- Monochrome with accent color

**Typography:**
- System font stack (fast, native feel)
- Monospace for code: SF Mono / Menlo / monospace

**Color Palette:**
```css
:root {
  --tap-accent: #6366f1;      /* Indigo */
  --tap-accent-light: #818cf8;
  --tap-accent-dark: #4f46e5;

  --tap-bg: #ffffff;
  --tap-bg-soft: #f8fafc;
  --tap-text: #0f172a;
  --tap-text-muted: #64748b;

  /* Dark mode */
  --tap-bg-dark: #0f172a;
  --tap-bg-soft-dark: #1e293b;
  --tap-text-dark: #f8fafc;
  --tap-text-muted-dark: #94a3b8;
}
```

**Logo:** Text-based "Tap" for now. Clean sans-serif, possibly with subtle accent on the "T" or a small visual mark.

### Netlify Config

```toml
# netlify.toml
[build]
  command = "npm run docs:build"
  publish = "docs/.vitepress/dist"

[[redirects]]
  from = "/*"
  to = "/index.html"
  status = 200
```

---

## Tasks

### Phase 1: Scaffold
- [ ] Initialize VitePress in /docs
- [ ] Configure site metadata, nav, sidebar
- [ ] Set up custom theme with Linear-inspired styling
- [ ] Create all page files (empty or stubbed)
- [ ] Set up Netlify config

### Phase 2: Landing Page
- [ ] Build HomeHero component
- [ ] Build HomeFeatures component
- [ ] Build HomeThemes component (placeholder visuals)
- [ ] Build HomeCode component
- [ ] Add copy-to-clipboard functionality
- [ ] Polish responsive design
- [ ] Add subtle animations

### Phase 3: Documentation Content
- [ ] Write Getting Started
- [ ] Write all Guide pages
- [ ] Write all Reference pages
- [ ] Stub Examples pages

### Phase 4: Polish
- [ ] Dark mode refinement
- [ ] Mobile navigation
- [ ] Search configuration
- [ ] SEO meta tags
- [ ] Social preview image
- [ ] Favicon

---

## Out of Scope (For Now)

- Interactive live demos (need Tap built first)
- Real theme screenshots (need themes built)
- Blog section
- Changelog page
- Community/Discord links
- Versioned docs
