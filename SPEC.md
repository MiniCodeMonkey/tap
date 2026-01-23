# Tap - Product & Technical Specification

## Product Vision

**Tap** is a markdown-based presentation tool specifically designed for technical presentations. It solves the rigidity problem of Marp and the complexity problem of Slidev by providing beautiful defaults with easy customization, all while being particularly optimized for code-heavy presentations.

**Core Philosophy:**
- Beautiful by default
- Simple for common cases, powerful for advanced needs
- 100% markdown for basic presentations
- Progressive enhancement (CSS/JS only when needed)
- Developer-first experience

---

## Target Audience

**Primary:** Developers giving technical talks (conference speakers, internal tech talks, meetups)

**Secondary:** Technical educators, developer advocates, engineering managers

**Key user needs:**
- Present code effectively with syntax highlighting and animations
- Embed live code execution and database queries
- Create professional-looking slides without design skills
- Quick iteration and hot-reload during creation
- Export to static HTML or PDF for distribution

---

## Core Product Requirements

### 1. **Presentation Format**
- **Markdown-first** with frontmatter configuration
- Slides separated by `---` (standard markdown convention)
- Support for inline HTML/CSS escape hatches
- Custom component syntax (e.g., `::left::` `::right::` for two-column layouts)
- Metadata blocks for code execution configuration

### 2. **Layout System**
- **Auto-detection** of layout based on content structure
- **Explicit override** via `<!-- layout: name -->` comments
- 10+ built-in layouts per theme:
  - Title slide
  - Section header
  - Two/Three column
  - Code focus (full-screen code)
  - Big stat (large number emphasis)
  - Quote
  - Image background
  - Sidebar
  - Split media (image + text)
  - Blank/Custom

### 3. **Theme System**
- 5 distinctive built-in themes with completely different aesthetics:
  1. **Minimal** - Clean, Apple-style, Helvetica, whitespace
  2. **Gradient** - Modern, colorful gradients, glassmorphism
  3. **Terminal** - Hacker aesthetic, monospace, CRT effects
  4. **Brutalist** - Bold, geometric, high contrast
  5. **Keynote** - Professional, subtle shadows, smooth
- Custom theme support via configuration files
- Themes control: typography, colors, layouts, animations, CSS

### 4. **Animation System**
- **Remotion-inspired** declarative animations (frame-based, composable, timeline-driven)
- Built-in animation presets:
  - Fade/slide transitions
  - Typewriter effects for code
  - Count-up animations for numbers
  - Cascade/stagger for lists
  - Spring physics for smooth motion
- Theme-specific animation defaults
- Per-element animation overrides via CSS classes
- Timeline-based multi-step reveals

### 5. **Code Presentation**
- **Shiki** for syntax highlighting (VS Code themes)
- Line highlighting and ranges
- Multi-step code reveals (progressive disclosure)
- Code diffs visualization
- Live code execution via driver system
- Terminal recording playback (Asciinema)
- Configurable font sizes per-slide

### 6. **Live Code Execution**
- **Driver-based architecture** for extensibility
- Built-in drivers:
  - **SQLite** - In-memory or file-based queries
  - **MySQL/PostgreSQL** - Via configured connections
  - **Shell** - Execute system commands (uptime, scripts, etc.)
  - **Custom** - Community-provided drivers
- Connection configuration via `tap.yaml`
- Results displayed in real-time on slide
- Error handling and timeout protection
- Security sandboxing (shell command whitelist, timeouts)

### 7. **Image Handling**
- **Relative paths:** Images referenced relative to the markdown file location
  - `![Alt](./images/diagram.png)` resolves from the markdown file's directory
- **Absolute URLs:** External images via `https://` URLs (user's responsibility for availability)
- **Build behavior:** `tap build` copies referenced local images to the output directory
  - Images are placed in `dist/assets/` with content-hashed filenames
  - HTML references are rewritten automatically
- **Supported formats:** PNG, JPG, JPEG, GIF, SVG, WebP
- **Background images:** Via layout directives or CSS

### 8. **Offline Support**
- Built presentations are fully self-contained
- No external CSS/JS dependencies loaded at runtime
- All fonts embedded or use system font stacks
- Only external URLs explicitly used by the presenter (e.g., external images) require connectivity

### 9. **Interactive CLI/TUI**
- **Scaffolding commands** for creating new presentations (`tap new`)
- **Interactive slide builder** for adding slides to existing presentations (`tap add`)
  - TUI presents available layouts
  - User selects layout and fills in content fields
  - Generated markdown appended to presentation file
- **Live preview** in dev mode with hot reload
- **Keyboard shortcuts** for common operations
- **File watching** for instant updates
- ASCII art branding and friendly CLI interface

---

## Technical Architecture

### Backend: Go

**Core Components:**
- **CLI Framework:** Cobra for command interface
- **TUI:** Bubble Tea + Lip Gloss for interactive forms, selects, and styling
- **Markdown Parser:** goldmark for robust parsing
- **Config Parser:** gopkg.in/yaml.v3 for YAML configuration
- **Dev Server:** Built-in net/http with gorilla/websocket for hot reload and presenter sync
- **Code Execution:** Driver registry pattern with os/exec for external processes
- **QR Code:** skip2/go-qrcode for presenter mode QR codes (terminal + `/qr` endpoint)
- **Build System:** Static site generator for deployment
- **PDF Export:** Playwright via rod or playwright-go
- **Distribution:** Single binary (cross-compiled for macOS, Linux, Windows)

**Commands:**
- `tap new` - Create new presentation with interactive prompts
- `tap dev <file>` - Start dev server with hot reload
  - `--port=3000` - Server port (default: 3000)
  - `--presenter-password=<secret>` - Require password for presenter mode
- `tap build <file>` - Generate static HTML/CSS/JS bundle
- `tap pdf <file>` - Export to PDF via Playwright
- `tap add` - Interactive slide builder (TUI for adding slides with layout selection)

### Frontend: Svelte + Vite

**Core Components:**
- **UI Framework:** Svelte 5 (compiles to tiny vanilla JS)
- **Build Tool:** Vite for fast development and bundling
- **Animation:** Svelte transitions + GSAP for advanced effects
- **Code Highlighting:** Shiki (runtime or build-time)
- **WebSocket Client:** For hot reload communication
- **Router:** Simple hash-based navigation

**Build Requirement:** Node.js 18+ is required to build the frontend assets. The Go binary embeds pre-built frontend assets, so end users don't need Node.js installed.

**Player Features:**
- Keyboard navigation (arrow keys, space)
- Speaker notes via HTML comments: `<!-- Your speaker notes here -->`
- Presenter mode (see below)
- Slide overview/thumbnail view for navigation
- Progress indicator
- Slide counter
- URL-based slide access (#/5)

### Presenter Mode

**Architecture:** Dual-window with WebSocket synchronization (requires `tap dev`, not available in static builds).

**URLs:**
- Audience view: `http://localhost:3000/` or `http://localhost:3000/#/5`
- Presenter view: `http://localhost:3000/presenter` or `http://localhost:3000/presenter#/5`

**Presenter View Features:**
- Current slide (compact view)
- Next slide preview
- Speaker notes for current slide
- Elapsed timer (click to reset)
- Slide counter (e.g., "5 / 24")
- Touch-friendly controls for tablet use

**Cross-Device Support:**
- Dev server binds to `0.0.0.0` for network access
- Access presenter view from any device on the same network (e.g., iPad)
- **QR Code:** Displayed in terminal on server start and available at `/qr` endpoint for easy mobile connection
- **Password Protection:** Optional `--presenter-password=<secret>` flag
  - When set, `/presenter` requires `?key=<secret>` query parameter
  - QR code includes the password automatically
  - Prevents unauthorized access on public networks (e.g., conference WiFi)

**Sync Mechanism:**
- WebSocket broadcast keeps all connected windows in sync
- Navigation in presenter window controls all audience windows
- Reuses existing hot-reload WebSocket infrastructure

**Keyboard Shortcuts:**
- `S` — Open presenter view in new window (from audience view)
- Arrow keys / Space — Navigate slides (both views)
- `O` — Toggle slide overview
- `R` — Reset timer (presenter view only)

### Driver System Architecture

The driver system enables live code execution during presentations by shelling out to external tools, languages, and services. This allows presenters to run JavaScript, Python, PHP, SQL queries, API calls, or any shell command directly from their slides.

**Interface Design:**
- Drivers execute external processes via `os/exec`
- Registry pattern for driver discovery
- Configuration via `tap.yaml`
- Sandboxed execution with command whitelists
- Timeout and resource limits
- Structured result format (success, data, error)

**Error Handling UX:**
- Failed executions display a subtle, non-distracting error indicator
- Error message shown in a muted style (not bright red)
- Presenter can retry execution with a keyboard shortcut
- Graceful degradation: show "Execution failed" rather than crashing

**Driver Lifecycle:**
1. Parse code block metadata (`{driver: "mysql", connection: "demo"}`)
2. Load driver from registry
3. Shell out to external tool/language with timeout
4. Capture stdout/stderr
5. Return structured result (success, output, error)
6. Frontend renders result with animation (or error state)

---

## Data Flow

```
1. [slides.md]
   ↓
2. Go Parser → AST (JSON structure)
   ↓
3. Transformer → Normalized slide objects with layout detection
   ↓
4. Dev Server → Serves JSON to frontend via HTTP
   ↓
5. Svelte App → Renders slides with theme + animations
   ↓
6. WebSocket → Hot reload on file changes
   ↓
7. Live Code → POST to /api/execute → Driver (os/exec) → Result → Animated display
```

---

## Key Differentiators

### vs. Marp
- **More flexible layouts** (not rigid template-based)
- **Live code execution** (not just static highlighting)
- **Better animations** (Remotion-style, theme-integrated)
- **Interactive dev experience** (CLI, hot reload, slide builder)

### vs. Slidev
- **Less configuration required** (beautiful out-of-box)
- **Simpler syntax** (pure markdown, no Vue components needed)
- **Auto-layout detection** (no manual layout specification)
- **Single binary** (no Node.js runtime required for end users)

### vs. PowerPoint/Keynote
- **Version control friendly** (plain markdown files)
- **Code-first** (syntax highlighting, execution, terminal recordings)
- **Developer workflow** (CLI, text editor, git)
- **Reproducible** (no binary formats)

---

## User Experience Flow

### Creating a New Presentation

1. Run `tap new`
2. Interactive prompts: title, theme selection, output file
3. Generates starter `slides.md` with example slides
4. Run `tap dev slides.md` to start development

### Adding Slides

**Option A - Manual:**
- Edit `slides.md` directly in preferred text editor
- Hot reload shows changes instantly

**Option B - Interactive:**
- Run `tap add` (while dev server running)
- TUI shows layout picker
- Form fields based on layout choice
- Generated markdown inserted into file

### Development Workflow

1. Write slides in markdown
2. Preview in browser updates live
3. Press 'a' in terminal to add slide
4. Press 'o' to open/refresh browser
5. Keyboard shortcuts for navigation during preview

### Building for Production

1. Run `tap build slides.md`
2. Generates `dist/` folder with:
   - `index.html` (self-contained)
   - Bundled CSS/JS
   - Optimized assets
3. Deploy to any static host (Netlify, Vercel, GitHub Pages)
4. Or run `tap pdf slides.md` for PDF export

---

## Design System

### Typography Scale
- **Theme-controlled** font families
- **Responsive sizing** for different screen sizes
- **Code font** separate from body font
- **Hierarchy:** Title (8rem) → Heading (6rem) → Body (2rem) → Code (1.5-2.5rem)

### Color System
- Each theme defines:
  - Background colors
  - Text colors (primary, secondary)
  - Accent colors
  - Code syntax colors (via Shiki theme)
  - Semantic colors (error, success, warning)

### Spacing System
- Consistent padding/margins across layouts
- Theme-specific spacing multipliers
- Grid-based layouts for columns

### Animation Timing
- **Fast:** 200-300ms (subtle transitions)
- **Medium:** 400-600ms (standard transitions)
- **Slow:** 800-1200ms (emphasis animations)
- **Stagger:** 50-150ms delays between items

---

## Configuration File Structure

### `slides.md` Frontmatter
```yaml
title: Presentation Title
theme: minimal
author: Name
date: 2026-01-23
aspectRatio: 16:9  # Default. Supported: 16:9, 4:3, 16:10
```

### `tap.yaml` (optional project config)
```yaml
drivers:
  mysql:
    demo:
      host: localhost
      port: 3306
      database: demo_db
      username: root
      password: secret
  shell:
    timeout: 10
    allowed_commands:
      - uptime
      - php
      - node
      - python

defaults:
  theme: minimal
  animations: true
  codeTheme: github-dark
  aspectRatio: 16:9
```

### Supported Aspect Ratios
- **16:9** (default) - Standard widescreen, works for most projectors and screens
- **4:3** - Traditional aspect ratio, useful for older projectors
- **16:10** - Common laptop screen ratio

---

## Testing Strategy

### Go Testing
All Go tests use the standard `testing` package with table-driven tests.

**Unit Tests:**
- Parser: markdown → AST conversion
- Transformer: layout detection, slide normalization
- Drivers: code execution, error handling
- Configuration: YAML loading, validation

**Integration Tests:**
- CLI commands: output verification
- Dev server: HTTP responses, WebSocket
- Build process: static file generation

**Browser Tests (Playwright):**
- Full presentation flow
- Keyboard navigation
- Live code execution
- Hot reload functionality
- Visual regression tests for themes
- Cross-browser compatibility

### Component Tests (Svelte)
- Slide rendering with different layouts
- Animation triggers and timing
- Theme application
- Code block rendering

### Performance Tests
- Animation frame rate (60fps target)
- Code execution timeouts
- Large presentation handling (100+ slides)

---

## Distribution & Installation

### Installation Methods
1. **Direct download:** Pre-built binaries for macOS (Intel + Apple Silicon), Linux, Windows
2. **Homebrew:** `brew install tapsh/tap/tap`
3. **Go install:** `go install github.com/tapsh/tap@latest`

### System Requirements
- **End users:** No dependencies—single binary with embedded frontend assets
- **Development:** Go 1.22+, Node.js 18+ (for frontend development)
- **Optional:** Database clients for live query drivers (mysql, psql, sqlite3)

### Deployment Options
- **Static hosting:** Build to static files, deploy anywhere
- **Self-hosted:** Run dev server in production (not recommended)
- **PDF export:** Standalone PDF file (requires Chrome/Chromium for Playwright)

---

## Success Metrics

- Time to create first presentation: <5 minutes
- Lines of CSS needed for customization: <50 for most users
- Build time for 50-slide presentation: <2 seconds

---

## Future Enhancements (Post-MVP)

### Phase 2
- Remote control (phone as clicker)
- Drawing/annotation mode during presentation
- Multiplayer/collaborative editing

### Phase 3
- Theme marketplace/gallery
- Plugin system for custom functionality
- Cloud sync and version history
- Recording/streaming integration
- Interactive polls and Q&A

### Phase 4
- Web-based editor (no local installation)
- Template library
- AI-assisted slide generation
- Multilingual support
- Accessibility improvements (screen readers, high contrast)

---

## Technical Constraints & Decisions

### Why Go Backend?
- Single binary distribution (no runtime dependencies)
- Excellent cross-platform compilation
- Fast startup time for CLI tools
- Built-in concurrency for dev server and file watching
- Strong standard library (HTTP, WebSocket, file I/O)

### Why Svelte Frontend?
- Compiles to vanilla JS
- Built-in animation system
- Reactive updates (perfect for live code)
- Simple learning curve
- Clean component architecture

### Why Not Full Framework?
- Avoid complexity (focused tool, not a web app)
- Faster development for focused tool
- Easier for contributors to understand
- Single binary distribution

### Security Considerations
- Code execution sandboxing via command whitelist
- Shell commands must be explicitly allowed in `tap.yaml`
- Database connection validation
- Timeout enforcement (default 10s, configurable)
- No eval() or arbitrary code execution in frontend

---

## Decisions

- **Licensing:** MIT
- **Branding:** "Tap" (use "Tap" in prose, `tap` for CLI commands, tap.sh for the domain/website)
- **Logo:** TBD
- **Documentation:** Separate documentation site (future priority — we want exceptional docs and getting started guides)
- **Telemetry:** None. Absolutely no telemetry.
- **Versioning:** Semantic versioning (semver)

---

## Next Steps

1. Create detailed PRD with user stories
2. Design mockups for key screens/themes
3. Set up project structure and CI/CD
4. Begin implementation with parser + one theme as proof-of-concept