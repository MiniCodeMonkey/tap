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
theme: terminal
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
- 12 built-in layouts, each with its own set of named slots:
  - Default content
  - Title slide
  - Section header
  - Two/Three column
  - Code focus (full-screen code)
  - Big stat (large number emphasis)
  - Quote
  - Image background (cover)
  - Sidebar
  - Split media (image + text)
  - Blank/Custom

### 4. **Theme System**

Tap uses a modern CSS custom properties architecture. Themes define CSS variables that control all visual aspects, enabling runtime theme switching and easy customization.

**Built-in Themes:**

`base` is the plain fallback, used when a deck names no theme or names one that doesn't exist. 20 designed themes sit alongside it, each with a `light` or `dark` polarity and a one-line pitch describing the kind of talk it fits: `terminal`, `product`, `swiss`, `newsprint`, `zine`, `poster`, `blueprint`, `riso`, `retro-computing`, `paperback`, `keynote`, `editorial`, `observatory`, `arcade`, `isometric`, `ink`, `lab-notebook`, `bauhaus`, `sketch`, `transit`. See `docs/guide/themes.md` for the full pitch and polarity of each.

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
- **Motion-powered** declarative animations (composable, timeline-driven)
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
- **Shiki** for syntax highlighting, with its CSS-variables theme: token colors come from `--shiki-*` custom properties each tap theme defines, so code follows the active theme without re-highlighting
- Highlighting runs in the browser, lazily, once per code block
- Line highlighting and ranges from the fence info string (```` ```go {1,3-5} ````), clamped to the block's real line count
- Code diffs visualization
- Live code execution via driver system
- Terminal recording playback (Asciinema), with the player bundled rather than loaded from a CDN
- Code text size comes from the theme, which keeps it at 36px or larger on the 1920px canvas

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
- **Requires `tap dev`**: static builds display a graceful placeholder indicating live code is not available

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
  - File watching for instant updates (markdown, themes, images, all assets)
  - Built-in slide builder (press `a` to add slide)
  - Keyboard shortcuts for common operations
  - ASCII art branding and friendly interface
- **Standalone slide builder** (`tap add`) for adding slides when dev server isn't running
  - TUI presents available layouts
  - User selects layout and fills in content fields
  - Generated markdown appended to presentation file
- **Screenshot command** (`tap screenshot`) renders one slide, or every slide, to a PNG through the PDF exporter's headless browser, with a non-zero exit status when the slide shows an error card, so an LLM can check a slide it just wrote
- **Theme inspection** (`tap theme list`, `tap theme show`) prints a theme's tokens and illustration style, including a `--prompt` style brief for an image model
- **Component scaffold** (`tap add component <Name>`) writes a working component file and its type declarations

### 12. **Deck-Supplied Components**
- A deck may supply React components from files in its own folder, as a whole-slide layout (`layout: ./slides/Thing.jsx`) or as an inline ```` ```component ./charts/Thing.jsx ```` fence with JSON props
- Bundled by tap itself with esbuild's Go API: no Node install, no build step, and never over the network
- `react`, `react/jsx-runtime`, `react-dom`, `react-dom/client`, `motion`, `motion/react`, and `tap` resolve to the host's own instances through `window.__TAP_HOST__`, so a component and the frontend share one React and one Motion
- Any other bare import resolves from a `node_modules` folder next to the deck
- Components receive the slide's slots, the fence's props, the presenter step, and print mode, and read the active theme's tokens with `useTheme()`
- Step counts come from the slide's `steps:` directive or a static `export const steps = N`, read without running the file
- See `docs/reference/components-reference.md`

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

### Frontend: React + Vite

**Core Components:**
- **UI Framework:** React 19
- **Build Tool:** Vite for fast development and bundling
- **Animation:** The Motion library, driven by theme-defined timing and easing
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
- Dev server binds to `0.0.0.0` for network access; the temporary servers `tap pdf` and `tap screenshot` start bind `127.0.0.1`, since only tap's own headless browser talks to them
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
- The hub retains the last slide, fragment, and step for 10 minutes after the last client disconnects (`TAP_HUB_STATE_RETENTION` overrides it), so a reload mid-talk lands back where the talk is
- A page's load-time URL hash is weighed against the hub's state only on the **first** state message of that page load; a later one, which is what a reconnect sends, wins outright, so a reconnecting presenter is never pulled backwards past the audience
- The retained state carries no theme, so a reconnecting window keeps the theme it was set to
- A relayed slide index that is negative, or past the last slide, is rejected
- `?print=true` never opens the websocket at all

**Keyboard Shortcuts:**
- `S`: Open presenter view in new window (from audience view)
- Arrow keys / Space: Navigate slides (both views)
- `O`: Toggle slide overview
- `R`: Reset timer (presenter view only)

### Driver System Architecture

The driver system enables live code execution during presentations by shelling out to external tools, languages, and services. This allows presenters to run JavaScript, Python, PHP, SQL queries, API calls, or any shell command directly from their slides.

**Interface Design:**
- Drivers execute external processes via `os/exec`
- Registry pattern for driver discovery
- Configuration via frontmatter (credentials via `.env` file)
- Full developer control, no command restrictions (presenters control their own slides)
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
5. React App → Renders slides with theme + animations
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
- **Better animations** (Motion-powered, theme-integrated)
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
theme: base                 # base (default) plus 20 designed themes, see docs/guide/themes.md
author: Name
date: 2026-01-23
aspectRatio: 16:9          # 16:9 (default), 4:3, or 16:10
transition: fade           # none, fade (default), slide, push, zoom
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
theme: terminal
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

Partial overrides work: only specify what you want to change. Invalid color values are ignored with a console warning.

### Custom Theme CSS
For complete control, create your own theme CSS file:

```yaml
---
customTheme: ./my-theme.css
---
```

The path is relative to your markdown file. The stylesheet is served at
`/api/custom-theme.css` and loaded after the built-in theme's CSS, so it
can override any of that theme's custom properties or rules.

Scope your rules the way a built-in theme does, with
`[data-theme="<slug>"]`, and redefine the tokens a theme owns:

```css
/* my-theme.css: overrides on top of the base theme */
[data-theme='base'] {
  --bg: #1a1a2e;
  --fg: #eaeaea;
  --muted: #888888;
  --accent: #e94560;
  --accent-text: #e94560;
  --surface: #16213e;

  --font-display: 'Your Font', system-ui, sans-serif;
  --font-body: 'Your Font', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', ui-monospace, monospace;

  --ease: cubic-bezier(0.4, 0, 0.2, 1);
  --dur: 400ms;

  --space-unit: 32px;
  --radius: 8px;
  --stroke-width: 2px;
}

[data-theme='base'] .slot h1 { font-weight: 700; }
```

See `docs/reference/theme-porting.md` for the full contract, the Shiki
token variables, and the selector conventions each built-in theme follows.

If the stylesheet fails to load, tap logs a console warning and the
built-in theme stays in place.

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

### Component Tests (React)
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
2. **Homebrew:** `brew install MiniCodeMonkey/tap/tap`
3. **Go install:** `go install github.com/MiniCodeMonkey/tap@latest`

### System Requirements
- **End users:** No dependencies, single binary with embedded frontend assets
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
- A second polarity (light and dark) for every built-in theme
- More for deck components, which ship today as whole-slide and inline React files bundled from the deck's own folder: hot module replacement without a page reload, remote component registries, and server-side rendering for PDF without a browser
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

### Why React Frontend?
- Decks are written mostly by LLMs, and LLMs know React deeply: layouts and illustrations are React components, so generating or extending one is a well-trodden path
- A deck-supplied component shares the host's React instead of bundling its own, keeping custom slide content small and consistent
- Huge ecosystem of existing components and patterns to draw from for layouts, charts, and diagrams
- Reactive updates (works well for live code results streaming in)
- Familiar to the overwhelming majority of contributors

### Why Not a Bigger Framework?
- Avoid complexity (focused tool, not a web app)
- Faster development for a focused tool
- Easier for contributors to understand
- Single binary distribution

### Security Considerations
- Presenters have full control over code execution in their own slides (no restrictions)
- Database credentials loaded from environment variables (never committed to git)
- Timeout enforcement (default 10s, configurable)
- No eval() in the frontend
- **WebSocket origin.** The dev server's hub accepts a connection with no `Origin` header, or one whose origin host equals the request's own `Host` header (which covers `localhost`, `127.0.0.1`, a LAN address, a fallback port, and the documented Vite proxy workflow). Everything else is refused with HTTP 403 and `Forbidden: origin not allowed`, and the rejected origin is logged. `tap dev --allow-origin <origin>` (repeatable) adds exceptions for a contributor's separate dev server.
- **Presenter password gates control, not just the view.** With `--presenter-password` set, a correct `?key=` on `/presenter` sets an HttpOnly `tap_presenter_key` cookie. A websocket connection without that cookie still receives sync and reload traffic, so it follows along, but the hub drops its navigation messages: an audience window moves only its own screen, and the presenter drives the room. With no password, any window can navigate any other, as before.
- **Image attributes are escaped and validated.** The alt text and URL of an image with an attribute block are HTML-escaped, and a size value must be a CSS length or percentage or it is dropped with a warning naming the slide. Before this, a quoted alt text broke the tag and a hostile `width` value reached the `style` attribute unescaped (issue #7).
- **Deck-supplied components.** A deck component is code the deck's author chose to run. It carries exactly the same trust as raw HTML written into the deck, and it runs with the page's full privileges. Tap never fetches component code from the network: every bundle is built with esbuild from files on disk, inside the deck's own folder, plus whatever those files import. Code and stylesheets may be imported from outside the deck folder; data and asset files (JSON, text, images, fonts) must live inside it. A static build contains the bundled component code, so publishing a built deck publishes that code. Build and present only decks you trust, the same rule that already applies to live code execution.

---

## Decisions

- **Licensing:** MIT
- **Branding:** "Tap" (use "Tap" in prose, `tap` for CLI commands, tap.sh for the domain/website)
- **Logo:** TBD
- **Documentation:** Separate documentation site (future priority: we want exceptional docs and getting started guides)
- **Telemetry:** None. Absolutely no telemetry.
- **Versioning:** Semantic versioning (semver)

---

## Next Steps

1. Create detailed PRD with user stories
2. Design mockups for key screens/themes
3. Set up project structure and CI/CD
4. Begin implementation with parser + one theme as proof-of-concept