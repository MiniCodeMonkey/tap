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
- Support for inline HTML when needed (though unnecessary for 90%+ of use cases)
- **Directive system** for configuration (see below)

**File Structure:**
```markdown
---
title: My Presentation
theme: minimal
---

# First Slide

Content here...

---

# Second Slide

More content...
```

The first `---` opens frontmatter, the second `---` closes frontmatter AND begins slide 1, and each subsequent `---` starts a new slide.

### 2. **Directive System**

Tap uses a two-tier directive system:

**Global Directives (Frontmatter)**
- YAML block at the top of the markdown file
- Apply to the entire presentation
- Set defaults for theme, transitions, animations, driver connections, etc.

**Local Directives (Per-Slide)**
- YAML block inside an HTML comment at the start of a slide
- Override global settings for that specific slide
- Full YAML syntax supported (multi-line values, etc.)

Example:
```markdown
---
<!--
layout: two-column
transition: slide
notes: |
  Remember to demo this
  And mention the performance benefits
-->

# Slide Title
```

**Precedence:** Local directives override global directives.

**Environment Variables:**
- Reference environment variables with `$VAR_NAME` syntax in frontmatter
- Automatically loads `.env` file from presentation directory if present
- Useful for credentials: `password: $MYSQL_PASSWORD`
- Variables are resolved at runtime, never written to output

### 3. **Layout System**
- **Explicit selection** via local directive block with `layout: name`
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

### 4. **Theme System**

Tap uses a modern CSS custom properties architecture with TailwindCSS utilities. Themes define CSS variables that control all visual aspects, enabling runtime theme switching and easy customization.

**Built-in Themes:**

1. **Paper** - Ultra-clean and airy, like premium paper. Pure white backgrounds, confident Inter typography with tight letter-spacing (-0.02em), sophisticated warm accent (#78716c), and delicate shadows on code blocks. Perfect for professional presentations where content takes center stage.

2. **Noir** - Cinematic film noir elegance. Deep charcoal backgrounds (#0a0a0a), crisp white text, sophisticated gold accent (#d4af37). Features Playfair Display serif for headings, Inter for body, multi-layer soft shadows, and subtle vignette effects. Ideal for executive presentations and premium brand talks.

3. **Aurora** - Northern lights inspiration with vibrant color flows. Animated gradient mesh backgrounds (deep purples → electric blues → teals), glassmorphism with backdrop-blur, Space Grotesk typography, and glowing borders. Dynamic and mesmerizing for creative presentations.

4. **Phosphor** - Authentic CRT monitor aesthetic. True black (#000) background, phosphor green (#00ff00) text with multi-layer glow effects (5px/10px/20px text-shadows), subtle scanline overlay, and JetBrains Mono throughout. Everything IS terminal. Perfect for hacker talks and retro-tech themes.

5. **Poster** - Bold graphic design inspired by classic posters. Stark black and white with ONE electric red accent (#ef4444), Anton typeface for massive ALL CAPS headings, thick 4px borders, harsh 8px drop shadows, and zero rounded corners. Impossible to ignore.

**Theme Customization:**

- **Quick color swaps:** Use `themeColors` in frontmatter to override specific colors
- **Custom themes:** Use `customTheme` to load your own CSS file
- Themes control: typography, colors, layouts, animations, transitions, CSS variables

### 5. **Slide Transitions**
- 5 built-in slide-to-slide transitions:
  1. **None** - Instant switch (no animation)
  2. **Fade** - Cross-fade between slides (default)
  3. **Slide** - Horizontal slide left/right
  4. **Push** - New slide pushes old slide out
  5. **Zoom** - Subtle zoom in/out effect
- Set globally in frontmatter: `transition: fade`
- Override per-slide with `transition: slide` in local directive block
- Themes may specify their own default transition

### 6. **Animation System**
- **Svelte-powered** declarative animations (composable, timeline-driven)
- Built-in animation presets:
  - Typewriter effects for code
  - Count-up animations for numbers
  - Cascade/stagger for lists
  - Spring physics for smooth motion
- Theme-specific animation defaults
- Per-element animation overrides via CSS classes

**Incremental Reveals (Fragments):**
- Reveal content step-by-step with click/keypress
- Use `<!-- pause -->` inline to create reveal points within a slide
- Lists can auto-fragment with `fragments: true` in local directives (reveals bullets one at a time)
- Fragments work with all content types (text, code, images)
- Configure globally in frontmatter or per-slide with local directive block

### 7. **Code Presentation**
- **Shiki** for syntax highlighting (VS Code themes)
  - Build-time highlighting by default (faster, smaller bundle)
  - Runtime highlighting loaded automatically when presentation contains driver-enabled code blocks
- Line highlighting and ranges
- Multi-step code reveals (progressive disclosure)
- Code diffs visualization
- Live code execution via driver system
- Terminal recording playback (Asciinema)
- Configurable font sizes per-slide

### 8. **Live Code Execution**
- **Driver-based architecture** for extensibility
- Built-in drivers:
  - **SQLite** - In-memory or file-based queries
  - **MySQL/PostgreSQL** - Via configured connections
  - **Shell** - Execute system commands, scripts, any CLI tool
  - **Custom** - Community-provided drivers
- Connection configuration via frontmatter (credentials via environment variables)
- Results displayed in real-time on slide
- Error handling and timeout protection
- **Requires `tap dev`** — static builds display a graceful placeholder indicating live code is not available

**Code Block Syntax:**
````markdown
```sql {driver: "mysql", connection: "demo"}
SELECT * FROM users LIMIT 5;
```
````

The driver and connection are specified in the code block's info string using `{key: "value"}` syntax.

### 9. **Image Handling**
- **Relative paths:** Images referenced relative to the markdown file location
  - `![Alt](./images/diagram.png)` resolves from the markdown file's directory
- **Absolute URLs:** External images via `https://` URLs (user's responsibility for availability)
- **Build behavior:** `tap build` copies referenced local images to the output directory
  - Images are placed in `dist/assets/` with content-hashed filenames
  - HTML references are rewritten automatically
- **Supported formats:** PNG, JPG, JPEG, GIF, SVG, WebP
- **Missing images:** Display graceful inline error message on the slide (not a build failure)

**Image Positioning & Sizing:**
- **Inline:** Default behavior, image flows with content
- **Sized:** `![Alt](img.png){width=50%}` or `{width=300px}` (custom markdown extension)
- **Left/Right:** Use layout directives or `{position=left}` / `{position=right}`
- **Cover:** Use `layout: cover` with `background: ./image.png` in local directives for full-bleed image slides
- **Future:** AI-generated images via Gemini integration (planned)

### 10. **Offline Support**
- Built presentations are fully self-contained
- No external CSS/JS dependencies loaded at runtime
- All fonts embedded or use system font stacks
- Only external URLs explicitly used by the presenter (e.g., external images) require connectivity

### 11. **Interactive CLI/TUI**
- **Scaffolding commands** for creating new presentations (`tap new`)
- **Dev server is a full TUI** (`tap dev`):
  - Live preview with hot reload
  - File watching for instant updates (markdown, themes, images — all assets)
  - Built-in slide builder (press `a` to add slide)
  - Keyboard shortcuts for common operations
  - ASCII art branding and friendly interface
- **Standalone slide builder** (`tap add`) for adding slides when dev server isn't running
  - TUI presents available layouts
  - User selects layout and fills in content fields
  - Generated markdown appended to presentation file

---

## Technical Architecture

### Backend: Go

**Core Components:**
- **CLI Framework:** Cobra for command interface
- **TUI:** Bubble Tea + Lip Gloss for interactive forms, selects, and styling
- **Markdown Parser:** goldmark with common extensions enabled (tables, strikethrough, task lists, autolinks)
- **Config Parser:** gopkg.in/yaml.v3 for YAML configuration
- **Dev Server:** Built-in net/http with nhooyr.io/websocket for hot reload and presenter sync
  - Auto-reconnect on connection loss with subtle disconnection indicator in slide corner
- **Code Execution:** Driver registry pattern with os/exec for external processes
- **QR Code:** skip2/go-qrcode for presenter mode QR codes (terminal + `/qr` endpoint)
- **Build System:** Static site generator for deployment
- **PDF Export:** Playwright via playwright-go (requires Chrome/Chromium)
- **Distribution:** Single binary (cross-compiled for macOS, Linux, Windows)

**Commands:**
- `tap new` - Create new presentation with interactive prompts
- `tap dev <file>` - Start dev server TUI with hot reload, file watching, and slide builder
  - `--port=3000` - Server port (default: 3000)
  - `--presenter-password=<secret>` - Require password for presenter mode
- `tap build <file>` - Generate static HTML/CSS/JS bundle
- `tap serve [dir]` - Serve built presentation for preview (defaults to `dist/`)
- `tap pdf <file>` - Export to PDF via Playwright
  - `--content=slides` - Slides only (default)
  - `--content=notes` - Speaker notes only
  - `--content=both` - Slides with speaker notes
- `tap add [file]` - Standalone slide builder TUI (when dev server isn't running)

### Frontend: Svelte + Vite

**Core Components:**
- **UI Framework:** Svelte 5 (compiles to tiny vanilla JS)
- **Build Tool:** Vite for fast development and bundling
- **Animation:** Svelte transitions and spring physics (no external animation libraries)
- **Code Highlighting:** Shiki (build-time by default; runtime loaded automatically for presentations with live code drivers)
- **WebSocket Client:** For hot reload communication
- **Router:** Hash-based navigation (`#5` for slide 5)

**Build Requirement:** Node.js 18+ is required to build the frontend assets. The Go binary embeds pre-built frontend assets, so end users don't need Node.js installed.

**Player Features:**
- Keyboard navigation (arrow keys, space)
- Speaker notes via local directive block (see Directive System)
- Presenter mode (see below)
- Slide overview/thumbnail view for navigation
- Progress indicator
- Slide counter
- URL-based slide access (`#5`)

### Presenter Mode

**Architecture:** Dual-window with WebSocket synchronization (requires `tap dev`, not available in static builds).

**URLs:**
- Audience view: `http://localhost:3000/` or `http://localhost:3000/#5`
- Presenter view: `http://localhost:3000/presenter` or `http://localhost:3000/presenter#5`

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
- **QR Code:** Displayed in terminal on server start and available at `/qr` endpoint
  - Encodes the presenter view URL (includes password if set)
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
- Configuration via frontmatter (credentials via `.env` file)
- Full developer control — no command restrictions (presenters control their own slides)
- Timeout protection (default 10s, configurable)
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
- **Better animations** (Svelte-powered, theme-integrated)
- **Interactive dev experience** (CLI, hot reload, slide builder)

### vs. Slidev
- **Less configuration required** (beautiful out-of-box)
- **Simpler syntax** (pure markdown, no Vue components needed)
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
- Hot reload shows changes instantly (when dev server running)

**Option B - Interactive (in dev server):**
- Press `a` in the `tap dev` TUI
- Layout picker appears inline
- Generated markdown appended to file

**Option C - Interactive (standalone):**
- Run `tap add slides.md` when dev server isn't running
- Same TUI layout picker experience
- Generated markdown appended to file

### Development Workflow

1. Run `tap dev slides.md` to start the TUI
2. Write slides in markdown (in your editor)
3. Preview in browser updates live via hot reload
4. Press `a` in the TUI to add a new slide interactively
5. Press `o` to open/refresh browser
6. Press `q` to quit the dev server

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

## Configuration

All configuration lives in the presentation file's YAML frontmatter. No separate config files needed.

### Frontmatter (Global Directives)
```yaml
---
title: Presentation Title
theme: paper               # paper (default), noir, aurora, phosphor, poster
author: Name
date: 2026-01-23
aspectRatio: 16:9          # 16:9 (default), 4:3, or 16:10
transition: fade           # none, fade (default), slide, push, zoom
codeTheme: github-dark     # Shiki theme for syntax highlighting
fragments: false           # Auto-fragment lists (default: false)

# Theme color customization (optional - override specific colors)
themeColors:
  accent: "#3b82f6"        # Override accent color
  background: "#fafafa"    # Override background
  # Available keys: background, text, muted, accent, codeBg

# Custom theme file (optional - for complete theme control)
customTheme: ./my-theme.css  # Path relative to markdown file

# Driver configuration for live code execution
drivers:
  mysql:
    demo:
      host: localhost
      port: 3306
      database: demo_db
      username: root
      password: $MYSQL_PASSWORD  # Environment variable reference
  shell:
    timeout: 10  # Seconds before execution is killed
---
```

### Environment Variables
- Create a `.env` file in the same directory as your presentation
- Reference variables in frontmatter with `$VAR_NAME` syntax
- Variables are resolved at runtime, never written to build output
- Example `.env`:
  ```
  MYSQL_PASSWORD=secret123
  API_KEY=abc123
  ```

### Local Directives (Per-Slide)
Place a YAML block inside an HTML comment at the start of any slide:
```markdown
---
<!--
layout: two-column
transition: slide
fragments: true
background: ./images/bg.png
notes: |
  Remember to explain the diagram
  Point out the async flow
-->

# Slide Title
Content here...
```

Available local directives:
- `layout` - Override the auto-detected layout
- `transition` - Override the slide transition
- `fragments` - Enable/disable incremental reveals
- `background` - Set a background image
- `notes` - Speaker notes (supports multi-line with `|`)

### Supported Aspect Ratios
- **16:9** (default) - Standard widescreen, works for most projectors and screens
- **4:3** - Traditional aspect ratio, useful for older projectors
- **16:10** - Common laptop screen ratio

### Theme Color Customization
Quickly override specific colors in any built-in theme using `themeColors` in frontmatter:

```yaml
---
theme: paper
themeColors:
  accent: "#3b82f6"    # Brand blue instead of warm gray
  background: "#f8fafc" # Slightly cooler white
---
```

**Available color keys:**
- `background` - Main slide background color
- `text` - Primary text color
- `muted` - Secondary/muted text color
- `accent` - Accent color for highlights, links, list markers
- `codeBg` - Background color for code blocks

Partial overrides work — only specify what you want to change. Invalid color values are ignored with a console warning.

### Custom Theme CSS
For complete control, create your own theme CSS file:

```yaml
---
customTheme: ./my-theme.css
---
```

The path is relative to your markdown file. Your custom CSS must define a `.theme-custom` class with all required CSS variables:

```css
/* my-theme.css - Custom theme template */
.theme-custom {
  /* Required color variables */
  --color-bg: #1a1a2e;
  --color-text: #eaeaea;
  --color-muted: #888888;
  --color-accent: #e94560;
  --color-code-bg: #16213e;

  /* Optional extended colors */
  --color-border: rgba(255, 255, 255, 0.1);
  --color-surface: #1f1f3a;
  --color-surface-elevated: #252550;
  --color-link: var(--color-accent);
  --color-code-text: #d4d4d4;

  /* Typography */
  --font-sans: 'Your Font', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', monospace;

  /* Animation timing */
  --transition-duration: 400ms;
  --transition-timing: ease-out;
  --fragment-duration: 300ms;

  /* Apply base styles */
  font-family: var(--font-sans);
  color: var(--color-text);
  background-color: var(--color-bg);
}

/* Add custom typography, code styling, etc. as needed */
.theme-custom h1 { font-weight: 700; }
.theme-custom pre { border-radius: 8px; }
```

If the custom theme file is not found, Tap falls back to the default theme (paper) with a console warning.

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
2. **Homebrew:** `brew install tap-slides`
3. **Go install:** `go install github.com/tapsh/tap@latest`

### System Requirements
- **End users:** No dependencies—single binary with embedded frontend assets
- **Development:** Go 1.22+, Node.js 18+ (for frontend development)
- **Optional:** Database clients for live query drivers (mysql, psql, sqlite3)

### Deployment Options
- **Static hosting:** Build to static files, deploy anywhere
- **Local preview:** Use `tap serve` to preview built output before deploying
- **PDF export:** Standalone PDF file (requires Chrome/Chromium for Playwright)
  - Use `--notes` flag to include presenter notes

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
- Compiles to vanilla JS (tiny bundle size)
- Built-in animation and transition system (no external animation libraries needed)
- Reactive updates (perfect for live code)
- Simple learning curve
- Clean component architecture
- Spring physics built into the framework

### Why Not Full Framework?
- Avoid complexity (focused tool, not a web app)
- Faster development for focused tool
- Easier for contributors to understand
- Single binary distribution

### Security Considerations
- Presenters have full control over code execution in their own slides (no restrictions)
- Database credentials loaded from environment variables (never committed to git)
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