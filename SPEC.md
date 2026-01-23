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
- **Remotion-inspired** declarative animations
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
- Connection configuration via `tap.toml`
- Results displayed in real-time on slide
- Error handling and timeout protection
- Security sandboxing (shell command whitelist, timeouts)

### 7. **Interactive CLI/TUI**
- **Scaffolding commands** for creating presentations
- **Interactive slide builder** with layout selection
- **Live preview** in dev mode with hot reload
- **Keyboard shortcuts** for common operations
- **File watching** for instant updates
- ASCII art branding and friendly CLI interface

---

## Technical Architecture

### Backend: PHP 8.3+

**Core Components:**
- **CLI Framework:** Symfony Console for command interface
- **TUI:** Laravel Prompts for interactive forms, selects, and spinners
- **Markdown Parser:** league/commonmark for robust parsing
- **Config Parser:** yosymfony/toml for TOML configuration
- **Dev Server:** ReactPHP or Swoole for HTTP + WebSocket
- **Code Execution:** Driver registry pattern with PDO, Symfony Process
- **Build System:** Static site generator for deployment
- **Distribution:** Single PHAR file for easy installation

**Commands:**
- `tap new` - Create new presentation with interactive prompts
- `tap dev <file>` - Start dev server with hot reload
- `tap build <file>` - Generate static HTML/CSS/JS bundle
- `tap pdf <file>` - Export to PDF via headless browser
- `tap add` - Interactive slide builder (TUI)

### Frontend: Svelte + Vite

**Core Components:**
- **UI Framework:** Svelte 4+ (compiles to tiny vanilla JS)
- **Build Tool:** Vite for fast development and bundling
- **Animation:** Svelte transitions + GSAP for advanced effects
- **Code Highlighting:** Shiki (runtime or build-time)
- **WebSocket Client:** For hot reload communication
- **Router:** Simple hash-based navigation

**Player Features:**
- Keyboard navigation (arrow keys, space)
- Speaker notes support in markdown
- Presenter mode with notes, timer, and next slide preview
- Slide overview/thumbnail view for navigation
- Progress indicator
- Slide counter
- URL-based slide access (#/5)

### Driver System Architecture

**Interface Design:**
- Language-agnostic driver interface
- Registry pattern for driver discovery
- Configuration via YAML file
- Sandboxed execution environment
- Timeout and resource limits
- Structured result format (success, data, error)

**Driver Lifecycle:**
1. Parse code block metadata (`{driver: "mysql", connection: "demo"}`)
2. Load driver from registry
3. Execute with timeout/security constraints
4. Return structured result
5. Frontend renders result with animation

---

## Data Flow

```
1. [slides.md] 
   ↓
2. PHP Parser → AST (JSON structure)
   ↓
3. Transformer → Normalized slide objects with layout detection
   ↓
4. Dev Server → Serves JSON to frontend via HTTP
   ↓
5. Svelte App → Renders slides with theme + animations
   ↓
6. WebSocket → Hot reload on file changes
   ↓
7. Live Code → POST to /api/execute → Driver → Result → Animated display
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
- **PHP backend** (familiar to Laravel community)

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
aspectRatio: 16/9
```

### `tap.toml` (optional)
```toml
[drivers.mysql.demo]
host = "localhost"
port = 3306
database = "demo_db"
username = "root"
password = "secret"

[drivers.shell]
timeout = 10
allowed_commands = ["uptime", "php", "node"]

[defaults]
theme = "minimal"
animations = true
codeTheme = "github-dark"
```

---

## Testing Strategy

### PHP Testing (Pest)
All PHP tests use **Pest** for a clean, expressive testing experience.

**Unit Tests:**
- Parser: markdown → AST conversion
- Transformer: layout detection, slide normalization
- Drivers: code execution, error handling
- Configuration: TOML loading, validation

**Integration Tests:**
- CLI commands: output verification
- Dev server: HTTP responses, WebSocket
- Build process: static file generation

**Browser Tests (Pest):**
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
- Bundle size monitoring (<50KB target)
- Animation frame rate (60fps target)
- Code execution timeouts
- Large presentation handling (100+ slides)

---

## Distribution & Installation

### Installation Methods
1. **Composer global:** `composer global require tapsh/tap`
2. **PHAR download:** Direct binary download
3. **Homebrew (future):** `brew install tap`

### System Requirements
- PHP 8.3+
- Node.js 18+ (for building frontend, not runtime)
- SQLite extension (for SQLite driver)
- MySQL/PostgreSQL client libraries (for database drivers)

### Deployment Options
- **Static hosting:** Build to static files, deploy anywhere
- **Self-hosted:** Run dev server in production (not recommended)
- **PDF export:** Standalone PDF file

---

## Success Metrics

- Time to create first presentation: <5 minutes
- Lines of CSS needed for customization: <50 for most users
- Build time for 50-slide presentation: <2 seconds
- Bundle size for typical presentation: <100KB

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

### Why PHP Backend?
- Target audience familiarity (Laravel community)
- Excellent CLI tooling
- Easy distribution (single PHAR)
- Mature markdown parsing libraries
- Fast file operations

### Why Svelte Frontend?
- Tiny bundle size (compiles to vanilla JS)
- Built-in animation system
- Reactive updates (perfect for live code)
- Simple learning curve
- Clean component architecture

### Why Not Full Framework?
- Avoid complexity (no Laravel/Symfony full stack needed)
- Faster development for focused tool
- Easier for contributors to understand
- Smaller installation footprint

### Security Considerations
- Code execution sandboxing
- Shell command whitelist
- Database connection validation
- Timeout enforcement
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