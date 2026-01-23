# PRD: Tap MVP - Markdown Presentation Tool

## Introduction

Tap is a markdown-based presentation tool designed specifically for technical presentations. It solves the rigidity problem of Marp and the complexity problem of Slidev by providing beautiful defaults with easy customization, optimized for code-heavy presentations.

**Core Philosophy:**
- Beautiful by default
- Simple for common cases, powerful for advanced needs
- 100% markdown for basic presentations
- Progressive enhancement (CSS/JS only when needed)
- Developer-first experience

**Target Users:** Developers giving technical talks (conference speakers, internal tech talks, meetups), technical educators, developer advocates, engineering managers.

## Goals

- Enable users to create their first presentation in under 5 minutes
- Require zero configuration for a professional-looking presentation
- Support live code execution for SQL, shell commands, and extensible drivers
- Provide 5 distinctive themes with completely different aesthetics
- Deliver a single binary distribution with no runtime dependencies
- Achieve <2 second build time for 50-slide presentations
- Maintain high code quality and test coverage for long-term maintainability

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI (Cobra)                              │
│  tap new | tap dev | tap build | tap serve | tap pdf | tap add  │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Go Backend                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Parser    │  │ Transformer │  │     Driver Registry     │  │
│  │  (goldmark) │  │  (layouts)  │  │ (mysql/pg/sqlite/shell) │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ Dev Server  │  │   Builder   │  │     PDF Exporter        │  │
│  │ (net/http)  │  │  (static)   │  │    (playwright-go)      │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼ (JSON + WebSocket)
┌─────────────────────────────────────────────────────────────────┐
│                     Svelte Frontend                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Slides    │  │  Animations │  │    Code Highlighting    │  │
│  │  Renderer   │  │  (svelte)   │  │       (Shiki)           │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Themes    │  │  Presenter  │  │     Hot Reload          │  │
│  │   System    │  │    Mode     │  │    (WebSocket)          │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

---

# Part 1: Go Backend

## US-001: Project Structure and Build System

**Description:** As a developer, I need a well-organized Go project structure so that the codebase is maintainable and follows Go conventions.

**Acceptance Criteria:**
- [ ] Initialize Go module as `github.com/tapsh/tap`
- [ ] Create directory structure:
  ```
  tap/
  ├── cmd/
  │   └── tap/
  │       └── main.go           # Entry point
  ├── internal/
  │   ├── cli/                  # Cobra commands
  │   ├── parser/               # Markdown parsing
  │   ├── transformer/          # AST to slide objects
  │   ├── server/               # Dev server + WebSocket
  │   ├── builder/              # Static site generator
  │   ├── driver/               # Code execution drivers
  │   ├── pdf/                  # PDF export
  │   ├── tui/                  # Bubble Tea components
  │   └── config/               # Configuration loading
  ├── frontend/                 # Svelte app (separate build)
  ├── embedded/                 # Embedded frontend assets
  ├── themes/                   # Built-in theme definitions
  ├── testdata/                 # Test fixtures
  ├── go.mod
  ├── go.sum
  └── Makefile
  ```
- [ ] Configure `go:embed` directive for frontend assets in `embedded/`
- [ ] Create Makefile with targets: `build`, `test`, `lint`, `dev`, `release`
- [ ] Set up golangci-lint configuration (`.golangci.yml`)
- [ ] All code passes `go vet` and `golangci-lint run`

**Technical Specifications:**
- Go version: 1.22+
- Use `//go:embed` for bundling frontend assets
- Cross-compile targets: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64

---

## US-002: CLI Framework Setup

**Description:** As a user, I want intuitive CLI commands so that I can create, develop, build, and export presentations.

**Acceptance Criteria:**
- [ ] Install Cobra: `github.com/spf13/cobra`
- [ ] Implement root command with version flag (`tap --version`)
- [ ] Implement subcommands with proper help text:
  - `tap new` - Create new presentation
  - `tap dev <file>` - Start dev server
  - `tap build <file>` - Generate static build
  - `tap serve [dir]` - Serve built presentation
  - `tap pdf <file>` - Export to PDF
  - `tap add [file]` - Add slide interactively
- [ ] Global flags: `--verbose`, `--help`
- [ ] Proper exit codes (0 success, 1 error)
- [ ] Colorized output using `github.com/fatih/color`
- [ ] Unit tests for command parsing and flag validation

**Technical Specifications:**
```go
// cmd/tap/main.go
package main

import (
    "os"
    "github.com/tapsh/tap/internal/cli"
)

func main() {
    if err := cli.Execute(); err != nil {
        os.Exit(1)
    }
}
```

**Version Output:**
```
tap version 0.1.0
```

**Command Specifications:**

| Command | Arguments | Flags | Description |
|---------|-----------|-------|-------------|
| `tap new` | none | `--theme`, `--output` | Interactive new presentation |
| `tap dev` | `<file>` | `--port`, `--presenter-password` | Start dev server TUI |
| `tap build` | `<file>` | `--output` | Generate static HTML |
| `tap serve` | `[dir]` | `--port` | Serve built files |
| `tap pdf` | `<file>` | `--output`, `--content` | Export to PDF |
| `tap add` | `[file]` | none | Add slide interactively |

---

## US-003: Configuration Parser

**Description:** As a user, I want to configure my presentation via YAML frontmatter so that settings are self-contained in the markdown file.

**Acceptance Criteria:**
- [ ] Install YAML parser: `gopkg.in/yaml.v3`
- [ ] Define `Config` struct with all frontmatter fields:
  ```go
  type Config struct {
      Title       string            `yaml:"title"`
      Theme       string            `yaml:"theme"`
      Author      string            `yaml:"author"`
      Date        string            `yaml:"date"`
      AspectRatio string            `yaml:"aspectRatio"`
      Transition  string            `yaml:"transition"`
      CodeTheme   string            `yaml:"codeTheme"`
      Fragments   bool              `yaml:"fragments"`
      Drivers     map[string]any    `yaml:"drivers"`
  }
  ```
- [ ] Parse frontmatter from markdown file (between first `---` delimiters)
- [ ] Validate configuration values:
  - `aspectRatio`: must be "16:9", "4:3", or "16:10"
  - `transition`: must be "none", "fade", "slide", "push", or "zoom"
  - `theme`: must be valid built-in theme or custom definition
- [ ] Load `.env` file from presentation directory using `github.com/joho/godotenv`
- [ ] Resolve `$VAR_NAME` environment variable references in config values
- [ ] Return meaningful error messages for invalid config
- [ ] Unit tests for config parsing, validation, and env var resolution

**Technical Specifications:**
```go
// internal/config/config.go
package config

type Config struct {
    Title       string                 `yaml:"title"`
    Theme       string                 `yaml:"theme"`
    Author      string                 `yaml:"author"`
    Date        string                 `yaml:"date"`
    AspectRatio string                 `yaml:"aspectRatio"` // "16:9" | "4:3" | "16:10"
    Transition  string                 `yaml:"transition"`  // "none" | "fade" | "slide" | "push" | "zoom"
    CodeTheme   string                 `yaml:"codeTheme"`
    Fragments   bool                   `yaml:"fragments"`
    Drivers     map[string]DriverConfig `yaml:"drivers"`
}

type DriverConfig struct {
    Connections map[string]ConnectionConfig `yaml:",inline"`
}

type ConnectionConfig struct {
    Host     string `yaml:"host"`
    Port     int    `yaml:"port"`
    Database string `yaml:"database"`
    Username string `yaml:"username"`
    Password string `yaml:"password"` // Supports $ENV_VAR syntax
    Timeout  int    `yaml:"timeout"`
}

func Load(path string) (*Config, error)
func (c *Config) Validate() error
func resolveEnvVars(value string) string
```

---

## US-004: Markdown Parser

**Description:** As a developer, I need to parse markdown files into an AST so that slides can be extracted and processed.

**Acceptance Criteria:**
- [ ] Install goldmark: `github.com/yuin/goldmark`
- [ ] Enable goldmark extensions: tables, strikethrough, task lists, autolinks
- [ ] Create custom goldmark extension for:
  - Image attributes: `![Alt](img.png){width=50%}` → extract width/position
  - Code block metadata: ` ```sql {driver: "mysql"} ` → extract driver config
- [ ] Split markdown content into slides on `---` delimiter (after frontmatter)
- [ ] Parse local directive blocks (HTML comments with YAML) at slide start
- [ ] Parse `<!-- pause -->` markers for fragment points
- [ ] Return structured `Presentation` and `Slide` objects:
  ```go
  type Presentation struct {
      Config  Config
      Slides  []Slide
  }

  type Slide struct {
      Index       int
      Content     string        // Raw markdown content
      HTML        string        // Rendered HTML
      Directives  SlideDirectives
      Fragments   []Fragment
      CodeBlocks  []CodeBlock
  }
  ```
- [ ] Unit tests for all parsing scenarios (see testdata fixtures)

**Technical Specifications:**
```go
// internal/parser/parser.go
package parser

import (
    "github.com/yuin/goldmark"
    "github.com/yuin/goldmark/extension"
)

type Parser struct {
    md goldmark.Markdown
}

func New() *Parser {
    md := goldmark.New(
        goldmark.WithExtensions(
            extension.Table,
            extension.Strikethrough,
            extension.TaskList,
            extension.Linkify,
        ),
    )
    return &Parser{md: md}
}

func (p *Parser) Parse(content []byte) (*Presentation, error)
func (p *Parser) parseSlide(content string, index int) (*Slide, error)
func (p *Parser) parseDirectives(comment string) (*SlideDirectives, error)
func (p *Parser) parseCodeBlockMeta(info string) (*CodeBlockMeta, error)
```

**Slide Directive Struct:**
```go
type SlideDirectives struct {
    Layout     string `yaml:"layout"`
    Transition string `yaml:"transition"`
    Fragments  *bool  `yaml:"fragments"`
    Background string `yaml:"background"`
    Notes      string `yaml:"notes"`
}
```

---

## US-004b: Image Handling

**Description:** As a presenter, I want flexible image handling so that I can include diagrams, screenshots, and photos in my slides.

**Acceptance Criteria:**
- [ ] Resolve relative image paths from markdown file's directory
- [ ] Support absolute URLs (`https://...`)
- [ ] Support formats: PNG, JPG, JPEG, GIF, SVG, WebP
- [ ] Parse image attributes from extended syntax:
  - `![Alt](img.png){width=50%}` - percentage width
  - `![Alt](img.png){width=300px}` - pixel width
  - `![Alt](img.png){position=left}` - float left
  - `![Alt](img.png){position=right}` - float right
- [ ] Display graceful inline error for missing images (not build failure)
- [ ] Build process copies local images to `dist/assets/` with content hashes
- [ ] Rewrite image paths in HTML output
- [ ] Unit tests for path resolution and attribute parsing

**Technical Specifications:**
```go
// internal/parser/image.go
type ImageAttributes struct {
    Width    string // "50%" or "300px"
    Position string // "left", "right", or ""
}

func parseImageAttributes(suffix string) (*ImageAttributes, error)
```

---

## US-005: Slide Transformer

**Description:** As a developer, I need to transform parsed slides into normalized objects with layout detection so that the frontend can render them consistently.

**Acceptance Criteria:**
- [ ] Create `Transformer` that processes `Presentation` from parser
- [ ] Implement layout auto-detection based on content heuristics:
  - Title slide: only H1, optional subtitle
  - Section header: only H2
  - Code focus: single code block taking >50% of content
  - Quote: blockquote as primary content
  - Two-column: content with `|||` separator
  - Default: standard content layout
- [ ] Apply local directives (override auto-detected layout if specified)
- [ ] Merge global config with slide-specific overrides
- [ ] Process image paths (resolve relative to markdown file location)
- [ ] Extract speaker notes from directives
- [ ] Output JSON-serializable slide objects for frontend
- [ ] Unit tests for layout detection and transformation

**Technical Specifications:**
```go
// internal/transformer/transformer.go
package transformer

type Transformer struct {
    baseDir string // Directory of the markdown file
}

type TransformedSlide struct {
    Index      int               `json:"index"`
    Layout     string            `json:"layout"`
    HTML       string            `json:"html"`
    Notes      string            `json:"notes"`
    Transition string            `json:"transition"`
    Fragments  []FragmentGroup   `json:"fragments"`
    Background *BackgroundConfig `json:"background,omitempty"`
    CodeBlocks []CodeBlockConfig `json:"codeBlocks"`
}

type TransformedPresentation struct {
    Config TransformedConfig   `json:"config"`
    Slides []TransformedSlide  `json:"slides"`
}

func (t *Transformer) Transform(p *parser.Presentation) (*TransformedPresentation, error)
func (t *Transformer) detectLayout(slide *parser.Slide) string
func (t *Transformer) resolveImagePath(src string) string
```

**Layout Detection Rules:**
| Layout | Detection Criteria |
|--------|-------------------|
| `title` | Only H1, optional paragraph (subtitle) |
| `section` | Only H2, optional paragraph |
| `code-focus` | Single code block, minimal other content |
| `quote` | Blockquote as primary element |
| `two-column` | Contains `|||` column separator |
| `big-stat` | Contains `{.big-stat}` class on number |
| `default` | Everything else |

---

## US-006: Driver Registry System

**Description:** As a developer, I need a driver registry system so that live code execution is extensible and maintainable.

**Acceptance Criteria:**
- [ ] Define `Driver` interface:
  ```go
  type Driver interface {
      Name() string
      Execute(ctx context.Context, code string, config map[string]any) (*Result, error)
  }

  type Result struct {
      Success bool        `json:"success"`
      Output  string      `json:"output"`
      Error   string      `json:"error,omitempty"`
      Data    any         `json:"data,omitempty"` // Structured data (e.g., table rows)
  }
  ```
- [ ] Create `Registry` for driver discovery and retrieval
- [ ] Implement timeout enforcement (default 10s, configurable)
- [ ] Pass driver configuration from frontmatter to driver instance
- [ ] Design for extensibility (community-provided custom drivers in future)
- [ ] Unit tests for registry operations

**Technical Specifications:**
```go
// internal/driver/registry.go
package driver

type Registry struct {
    drivers map[string]Driver
}

func NewRegistry() *Registry
func (r *Registry) Register(d Driver)
func (r *Registry) Get(name string) (Driver, bool)
func (r *Registry) Execute(ctx context.Context, name string, code string, config map[string]any) (*Result, error)
```

---

## US-007: Shell Driver

**Description:** As a presenter, I want to execute shell commands in my slides so that I can demo CLI tools, scripts, and system commands.

**Acceptance Criteria:**
- [ ] Implement `ShellDriver` that executes commands via `os/exec`
- [ ] Support multi-line scripts (execute as single script)
- [ ] Capture both stdout and stderr
- [ ] Respect timeout configuration (kill process on timeout)
- [ ] Set working directory to presentation file's directory
- [ ] Return structured result with exit code
- [ ] Unit tests with mock commands
- [ ] Integration tests with real shell execution

**Technical Specifications:**
```go
// internal/driver/shell.go
package driver

type ShellDriver struct {
    workDir string
    timeout time.Duration
}

func NewShellDriver(workDir string, timeout time.Duration) *ShellDriver

func (d *ShellDriver) Name() string { return "shell" }

func (d *ShellDriver) Execute(ctx context.Context, code string, config map[string]any) (*Result, error) {
    ctx, cancel := context.WithTimeout(ctx, d.timeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, "sh", "-c", code)
    cmd.Dir = d.workDir

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err := cmd.Run()
    // ... handle result
}
```

---

## US-008: SQLite Driver

**Description:** As a presenter, I want to execute SQLite queries in my slides so that I can demo database concepts without external dependencies.

**Acceptance Criteria:**
- [ ] Implement `SQLiteDriver` using `os/exec` with `sqlite3` CLI
- [ ] Support in-memory database (`:memory:`)
- [ ] Support file-based database (path relative to presentation)
- [ ] Format output as table (using sqlite3 column mode)
- [ ] Parse output into structured data (rows/columns)
- [ ] Handle SQL errors gracefully
- [ ] Unit tests with temporary databases
- [ ] Integration tests requiring sqlite3 binary

**Technical Specifications:**
```go
// internal/driver/sqlite.go
package driver

type SQLiteDriver struct {
    dbPath  string
    timeout time.Duration
}

func NewSQLiteDriver(dbPath string, timeout time.Duration) *SQLiteDriver

func (d *SQLiteDriver) Execute(ctx context.Context, code string, config map[string]any) (*Result, error) {
    args := []string{
        "-header",
        "-column",
        d.dbPath,
        code,
    }
    cmd := exec.CommandContext(ctx, "sqlite3", args...)
    // ...
}
```

---

## US-009: MySQL Driver

**Description:** As a presenter, I want to execute MySQL queries in my slides so that I can demo real database interactions.

**Acceptance Criteria:**
- [ ] Implement `MySQLDriver` using `os/exec` with `mysql` CLI
- [ ] Load connection config from frontmatter
- [ ] Support environment variable references for credentials
- [ ] Format output as table
- [ ] Parse output into structured data
- [ ] Handle connection errors gracefully
- [ ] Mask credentials in error messages
- [ ] Unit tests with mock responses
- [ ] Integration tests (skipped if mysql not available)

**Technical Specifications:**
```go
// internal/driver/mysql.go
package driver

type MySQLDriver struct {
    timeout time.Duration
}

type MySQLConfig struct {
    Host     string
    Port     int
    Database string
    Username string
    Password string
}

func (d *MySQLDriver) Execute(ctx context.Context, code string, config map[string]any) (*Result, error) {
    cfg := parseConfig(config)
    args := []string{
        "-h", cfg.Host,
        "-P", strconv.Itoa(cfg.Port),
        "-u", cfg.Username,
        fmt.Sprintf("-p%s", cfg.Password),
        "-D", cfg.Database,
        "-e", code,
        "--table",
    }
    cmd := exec.CommandContext(ctx, "mysql", args...)
    // ...
}
```

---

## US-010: PostgreSQL Driver

**Description:** As a presenter, I want to execute PostgreSQL queries in my slides so that I can demo Postgres-specific features.

**Acceptance Criteria:**
- [ ] Implement `PostgresDriver` using `os/exec` with `psql` CLI
- [ ] Load connection config from frontmatter
- [ ] Support environment variable references for credentials
- [ ] Use `PGPASSWORD` environment variable for password
- [ ] Format output as table
- [ ] Parse output into structured data
- [ ] Handle connection errors gracefully
- [ ] Unit tests with mock responses
- [ ] Integration tests (skipped if psql not available)

**Technical Specifications:**
```go
// internal/driver/postgres.go
package driver

type PostgresDriver struct {
    timeout time.Duration
}

func (d *PostgresDriver) Execute(ctx context.Context, code string, config map[string]any) (*Result, error) {
    cfg := parseConfig(config)
    cmd := exec.CommandContext(ctx, "psql",
        "-h", cfg.Host,
        "-p", strconv.Itoa(cfg.Port),
        "-U", cfg.Username,
        "-d", cfg.Database,
        "-c", code,
    )
    cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", cfg.Password))
    // ...
}
```

---

## US-010b: Custom Driver Support

**Description:** As a presenter, I want to define custom drivers so that I can execute any language or tool in my slides.

**Acceptance Criteria:**
- [ ] Support custom driver definition in frontmatter:
  ```yaml
  drivers:
    python:
      command: python3
      args: ["-c"]
    node:
      command: node
      args: ["-e"]
    php:
      command: php
      args: ["-r"]
  ```
- [ ] Custom drivers execute via shell with configured command + args + code
- [ ] Inherit timeout from global config or per-driver override
- [ ] Custom drivers registered in registry alongside built-in drivers
- [ ] Validate command exists on system (warn if not found)
- [ ] Unit tests for custom driver registration and execution

**Technical Specifications:**
```go
// internal/driver/custom.go
package driver

type CustomDriver struct {
    name    string
    command string
    args    []string
    timeout time.Duration
}

func NewCustomDriver(name string, config CustomDriverConfig) *CustomDriver

func (d *CustomDriver) Execute(ctx context.Context, code string, _ map[string]any) (*Result, error) {
    args := append(d.args, code)
    cmd := exec.CommandContext(ctx, d.command, args...)
    // ...
}
```

**Usage Example:**
````markdown
```python {driver: "python"}
print("Hello from Python!")
for i in range(5):
    print(f"Count: {i}")
```
````

---

## US-011: Dev Server Core

**Description:** As a presenter, I want a development server so that I can preview my presentation with live updates.

**Acceptance Criteria:**
- [ ] Create HTTP server using `net/http`
- [ ] Serve embedded frontend assets from `embedded/`
- [ ] Serve presentation JSON at `/api/presentation`
- [ ] Bind to `0.0.0.0` for network access (not just localhost)
- [ ] Support configurable port via `--port` flag (default 3000)
- [ ] Log requests with colorized output
- [ ] Graceful shutdown on SIGINT/SIGTERM
- [ ] Unit tests for HTTP handlers

**Technical Specifications:**
```go
// internal/server/server.go
package server

type Server struct {
    port        int
    presentation *transformer.TransformedPresentation
    hub         *WebSocketHub
    mux         *http.ServeMux
}

func New(port int) *Server

func (s *Server) SetPresentation(p *transformer.TransformedPresentation)
func (s *Server) Start() error
func (s *Server) Shutdown(ctx context.Context) error

// Routes:
// GET /                    → Serve index.html
// GET /presenter           → Serve presenter view
// GET /api/presentation    → Return presentation JSON
// GET /api/execute         → Execute code block (POST)
// GET /qr                  → QR code page
// WS  /ws                  → WebSocket for hot reload
```

---

## US-012: WebSocket Hot Reload

**Description:** As a presenter, I want instant preview updates when I edit my markdown so that I can iterate quickly.

**Acceptance Criteria:**
- [ ] Install WebSocket library: `nhooyr.io/websocket`
- [ ] Create WebSocket hub for managing connections
- [ ] Broadcast `reload` message to all clients on file change
- [ ] Broadcast `slide` message for slide sync (presenter mode)
- [ ] Implement auto-reconnect logic (client-side, see frontend)
- [ ] Support multiple concurrent connections
- [ ] Unit tests for hub message broadcasting

**Technical Specifications:**
```go
// internal/server/websocket.go
package server

type WebSocketHub struct {
    clients    map[*Client]bool
    broadcast  chan Message
    register   chan *Client
    unregister chan *Client
}

type Message struct {
    Type    string `json:"type"`    // "reload" | "slide" | "connected"
    Payload any    `json:"payload"`
}

type Client struct {
    conn *websocket.Conn
    send chan Message
}

func NewHub() *WebSocketHub
func (h *WebSocketHub) Run()
func (h *WebSocketHub) Broadcast(msg Message)
func (h *WebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request)
```

**Message Types:**
| Type | Payload | Description |
|------|---------|-------------|
| `connected` | `{clientId: string}` | Initial connection confirmation |
| `reload` | `{timestamp: number}` | File changed, reload presentation |
| `slide` | `{index: number}` | Navigate to slide (presenter sync) |

---

## US-013: File Watcher

**Description:** As a developer, I need a file watcher so that the dev server can detect changes and trigger hot reload.

**Acceptance Criteria:**
- [ ] Install fsnotify: `github.com/fsnotify/fsnotify`
- [ ] Watch markdown file for changes
- [ ] Watch presentation directory for image/asset changes
- [ ] Debounce rapid changes (100ms window)
- [ ] Trigger presentation re-parse on change
- [ ] Broadcast reload to WebSocket clients
- [ ] Handle file rename/move gracefully
- [ ] Unit tests for debounce logic

**Technical Specifications:**
```go
// internal/server/watcher.go
package server

type Watcher struct {
    fsWatcher *fsnotify.Watcher
    onChange  func()
    debounce  time.Duration
}

func NewWatcher(paths []string, onChange func()) (*Watcher, error)
func (w *Watcher) Start() error
func (w *Watcher) Stop() error
```

---

## US-014: Code Execution API

**Description:** As a frontend, I need an API to execute code blocks so that live code demos work during presentations.

**Acceptance Criteria:**
- [ ] Create POST endpoint `/api/execute`
- [ ] Accept JSON body: `{driver: string, code: string, connection: string}`
- [ ] Look up driver from registry
- [ ] Look up connection config from presentation config
- [ ] Execute code with timeout
- [ ] Return structured result as JSON
- [ ] Return 400 for invalid driver
- [ ] Return 500 for execution errors (with error message)
- [ ] Unit tests for all error cases

**Technical Specifications:**
```go
// internal/server/api.go
package server

type ExecuteRequest struct {
    Driver     string `json:"driver"`
    Code       string `json:"code"`
    Connection string `json:"connection"`
}

type ExecuteResponse struct {
    Success bool   `json:"success"`
    Output  string `json:"output"`
    Error   string `json:"error,omitempty"`
    Data    any    `json:"data,omitempty"`
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
    // Parse request
    // Get driver from registry
    // Get connection config from presentation
    // Execute with context timeout
    // Return JSON response
}
```

---

## US-015: QR Code Generation

**Description:** As a presenter, I want a QR code for the presenter URL so that I can quickly access it from my phone/tablet.

**Acceptance Criteria:**
- [ ] Install QR library: `github.com/skip2/go-qrcode`
- [ ] Generate QR code for presenter URL on server start
- [ ] Display ASCII QR code in terminal
- [ ] Create `/qr` endpoint serving QR code as HTML page
- [ ] Include presenter password in URL if configured
- [ ] Unit tests for URL generation

**Technical Specifications:**
```go
// internal/server/qr.go
package server

func (s *Server) generateQRCode() (string, error) {
    url := fmt.Sprintf("http://%s:%d/presenter", s.hostname, s.port)
    if s.presenterPassword != "" {
        url += "?key=" + s.presenterPassword
    }

    qr, err := qrcode.New(url, qrcode.Medium)
    if err != nil {
        return "", err
    }
    return qr.ToSmallString(false), nil
}
```

---

## US-016: Presenter Password Protection

**Description:** As a presenter, I want to optionally password-protect the presenter view so that others on the same network can't control my presentation.

**Acceptance Criteria:**
- [ ] Accept `--presenter-password` flag on `tap dev`
- [ ] When set, `/presenter` requires `?key=<password>` query param
- [ ] Return 403 Forbidden if key is missing or incorrect
- [ ] Include password in QR code URL
- [ ] Show warning in terminal about password protection status
- [ ] Unit tests for authentication flow

---

## US-017: Static Site Builder

**Description:** As a presenter, I want to build my presentation to static files so that I can deploy it anywhere.

**Acceptance Criteria:**
- [ ] Create `Builder` that outputs to `dist/` directory
- [ ] Generate `index.html` with embedded presentation JSON
- [ ] Bundle frontend assets (JS, CSS)
- [ ] Copy referenced images to `dist/assets/` with content hashes
- [ ] Rewrite image paths in HTML
- [ ] Generate graceful placeholder for driver-enabled code blocks
- [ ] Minify HTML/CSS/JS output
- [ ] Report build stats (file count, total size, time)
- [ ] Unit tests for path rewriting
- [ ] Integration tests for full build

**Technical Specifications:**
```go
// internal/builder/builder.go
package builder

type Builder struct {
    outputDir string
}

type BuildResult struct {
    OutputDir  string
    FileCount  int
    TotalSize  int64
    BuildTime  time.Duration
}

func New(outputDir string) *Builder
func (b *Builder) Build(p *transformer.TransformedPresentation) (*BuildResult, error)
func (b *Builder) copyAssets(p *transformer.TransformedPresentation) error
func (b *Builder) hashFile(path string) (string, error)
```

---

## US-018: PDF Exporter

**Description:** As a presenter, I want to export my presentation to PDF so that I can share it or use it offline.

**Acceptance Criteria:**
- [ ] Install Playwright: `github.com/playwright-community/playwright-go`
- [ ] Launch headless Chromium browser
- [ ] Navigate through all slides, capturing each as PDF page
- [ ] Support `--content` flag: `slides`, `notes`, `both`
- [ ] Generate single PDF file
- [ ] Handle slides-only export (default)
- [ ] Handle notes-only export (printable format)
- [ ] Handle combined export (slide + notes per page)
- [ ] Report progress during export
- [ ] Clean up browser on completion
- [ ] Integration tests with sample presentation

**Technical Specifications:**
```go
// internal/pdf/exporter.go
package pdf

type Exporter struct {
    browser playwright.Browser
}

type ExportOptions struct {
    Content string // "slides" | "notes" | "both"
    Output  string // Output file path
}

func New() (*Exporter, error)
func (e *Exporter) Export(presentation *transformer.TransformedPresentation, opts ExportOptions) error
func (e *Exporter) Close()
```

---

## US-019: TUI Framework Setup

**Description:** As a developer, I need a TUI framework setup so that interactive CLI features have a consistent foundation.

**Acceptance Criteria:**
- [ ] Install Bubble Tea: `github.com/charmbracelet/bubbletea`
- [ ] Install Lip Gloss: `github.com/charmbracelet/lipgloss`
- [ ] Create base styles for TUI components
- [ ] Create reusable components: spinner, progress bar, input field
- [ ] Define color scheme matching Tap branding
- [ ] Unit tests for component rendering

**Technical Specifications:**
```go
// internal/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

var (
    PrimaryColor   = lipgloss.Color("#7C3AED") // Purple
    SecondaryColor = lipgloss.Color("#10B981") // Green
    ErrorColor     = lipgloss.Color("#EF4444") // Red
    MutedColor     = lipgloss.Color("#6B7280") // Gray

    TitleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(PrimaryColor)

    // ... more styles
)
```

---

## US-020: `tap new` Command

**Description:** As a user, I want an interactive command to create a new presentation so that I can get started quickly.

**Acceptance Criteria:**
- [ ] Create Bubble Tea model for `new` command
- [ ] Prompt for presentation title (text input)
- [ ] Prompt for theme selection (list of 5 themes)
- [ ] Prompt for output filename (text input with default)
- [ ] Generate starter markdown file with:
  - Frontmatter with selected options
  - Example title slide
  - Example content slide with code block
  - Example two-column slide
- [ ] Display success message with next steps
- [ ] Handle Ctrl+C gracefully (cancel without creating file)
- [ ] Unit tests for markdown generation

---

## US-021: `tap dev` TUI

**Description:** As a presenter, I want a TUI for the dev server so that I have a unified development experience.

**Acceptance Criteria:**
- [ ] Create Bubble Tea model for `dev` command
- [ ] Display ASCII art logo on start
- [ ] Show server URL and QR code
- [ ] Display keyboard shortcuts
- [ ] Show file watcher status
- [ ] Show recent hot reload events
- [ ] Keyboard shortcuts:
  - `a` - Add slide (launches slide builder)
  - `o` - Open browser
  - `q` - Quit server
- [ ] Show WebSocket connection count
- [ ] Handle errors gracefully (display in TUI)
- [ ] Unit tests for key handling

**Technical Specifications:**
```go
// internal/tui/dev.go
package tui

type DevModel struct {
    server      *server.Server
    watcher     *server.Watcher
    logs        []LogEntry
    connections int
    ready       bool
}

func NewDevModel(file string, port int) DevModel
func (m DevModel) Init() tea.Cmd
func (m DevModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m DevModel) View() string
```

---

## US-022: `tap add` Slide Builder

**Description:** As a presenter, I want an interactive slide builder so that I can add new slides without leaving the terminal.

**Acceptance Criteria:**
- [ ] Create Bubble Tea model for slide builder
- [ ] Display layout picker with previews (ASCII representations)
- [ ] Available layouts: title, section, default, two-column, code-focus, quote, big-stat
- [ ] Prompt for content based on selected layout:
  - Title: title text, subtitle (optional)
  - Default: heading, body
  - Code focus: language, code content
  - Quote: quote text, attribution
- [ ] Generate markdown for selected layout
- [ ] Append to presentation file
- [ ] Return to dev TUI (if running) or exit
- [ ] Unit tests for markdown generation per layout

---

## US-023: `tap build` Command

**Description:** As a user, I want a build command so that I can generate deployable static files.

**Acceptance Criteria:**
- [ ] Parse presentation file
- [ ] Transform to frontend-ready format
- [ ] Invoke Builder to generate static files
- [ ] Display progress with spinner
- [ ] Report build results (files, size, time)
- [ ] Exit with error code on failure
- [ ] Support `--output` flag for custom output directory

---

## US-024: `tap serve` Command

**Description:** As a user, I want to preview my built presentation locally so that I can verify it before deploying.

**Acceptance Criteria:**
- [ ] Serve files from `dist/` directory (or specified directory)
- [ ] Use simple HTTP file server
- [ ] Support `--port` flag (default 3000)
- [ ] Display URL in terminal
- [ ] Handle missing directory gracefully

---

## US-025: `tap pdf` Command

**Description:** As a user, I want to export my presentation to PDF so that I can share it offline.

**Acceptance Criteria:**
- [ ] Start temporary dev server (headless, random port)
- [ ] Launch PDF exporter
- [ ] Support `--content` flag: `slides` (default), `notes`, `both`
- [ ] Support `--output` flag for custom output path
- [ ] Display progress during export
- [ ] Clean up server and browser on completion
- [ ] Report success with output file path

---

# Part 2: Svelte Frontend

## US-026: Frontend Project Setup

**Description:** As a developer, I need a Svelte + Vite project setup so that the frontend can be developed and built independently.

**Acceptance Criteria:**
- [ ] Initialize Svelte 5 project with Vite in `frontend/` directory
- [ ] Configure TypeScript
- [ ] Create directory structure:
  ```
  frontend/
  ├── src/
  │   ├── lib/
  │   │   ├── components/       # Reusable UI components
  │   │   ├── layouts/          # Slide layout components
  │   │   ├── themes/           # Theme CSS and configs
  │   │   ├── stores/           # Svelte stores
  │   │   ├── utils/            # Utility functions
  │   │   └── types.ts          # TypeScript types
  │   ├── routes/
  │   │   ├── +page.svelte      # Audience view
  │   │   └── presenter/
  │   │       └── +page.svelte  # Presenter view
  │   ├── app.html
  │   └── app.css
  ├── static/
  ├── vite.config.ts
  ├── svelte.config.js
  ├── tsconfig.json
  └── package.json
  ```
- [ ] Configure Vite for library mode (for embedding)
- [ ] Add build script that outputs to `../embedded/`
- [ ] TypeScript strict mode enabled
- [ ] ESLint + Prettier configured

**Technical Specifications:**
```json
// package.json
{
  "name": "tap-frontend",
  "type": "module",
  "scripts": {
    "dev": "vite dev",
    "build": "vite build",
    "build:embed": "vite build --outDir ../embedded"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "^4.0.0",
    "svelte": "^5.0.0",
    "typescript": "^5.0.0",
    "vite": "^6.0.0"
  }
}
```

---

## US-027: TypeScript Type Definitions

**Description:** As a developer, I need TypeScript types matching the Go backend so that frontend code is type-safe.

**Acceptance Criteria:**
- [ ] Define types matching all Go structs:
  ```typescript
  interface Presentation {
    config: PresentationConfig
    slides: Slide[]
  }

  interface PresentationConfig {
    title: string
    theme: string
    author?: string
    date?: string
    aspectRatio: '16:9' | '4:3' | '16:10'
    transition: Transition
    codeTheme: string
    fragments: boolean
  }

  interface Slide {
    index: number
    layout: Layout
    html: string
    notes?: string
    transition?: Transition
    fragments: FragmentGroup[]
    background?: BackgroundConfig
    codeBlocks: CodeBlock[]
  }
  ```
- [ ] Export all types from `src/lib/types.ts`
- [ ] Use strict TypeScript (no `any` types)

---

## US-028: Presentation Store

**Description:** As a frontend developer, I need a centralized store for presentation state so that components stay synchronized.

**Acceptance Criteria:**
- [ ] Create Svelte store for presentation data
- [ ] Store current slide index
- [ ] Store current fragment index within slide
- [ ] Store presentation config
- [ ] Store all slides
- [ ] Provide actions: nextSlide, prevSlide, goToSlide, nextFragment, prevFragment
- [ ] Persist current slide in URL hash (`#5`)
- [ ] Initialize from URL hash on load
- [ ] Unit tests for store actions

**Technical Specifications:**
```typescript
// src/lib/stores/presentation.ts
import { writable, derived } from 'svelte/store'

export const presentation = writable<Presentation | null>(null)
export const currentSlideIndex = writable(0)
export const currentFragmentIndex = writable(0)

export const currentSlide = derived(
  [presentation, currentSlideIndex],
  ([$presentation, $index]) => $presentation?.slides[$index] ?? null
)

export const totalSlides = derived(
  presentation,
  ($p) => $p?.slides.length ?? 0
)

export function nextSlide() { /* ... */ }
export function prevSlide() { /* ... */ }
export function goToSlide(index: number) { /* ... */ }
export function nextFragment() { /* ... */ }
export function prevFragment() { /* ... */ }
```

---

## US-029: WebSocket Client

**Description:** As a frontend, I need a WebSocket client so that hot reload and presenter sync work.

**Acceptance Criteria:**
- [ ] Create WebSocket client that connects to `/ws`
- [ ] Handle `reload` messages → refresh presentation data
- [ ] Handle `slide` messages → navigate to specified slide
- [ ] Implement auto-reconnect with exponential backoff
- [ ] Show subtle disconnection indicator when not connected
- [ ] Store connection status in Svelte store
- [ ] Unit tests for message handling

**Technical Specifications:**
```typescript
// src/lib/stores/websocket.ts
import { writable } from 'svelte/store'

export const connected = writable(false)

class WebSocketClient {
  private ws: WebSocket | null = null
  private reconnectAttempts = 0
  private maxReconnectDelay = 30000

  connect() {
    this.ws = new WebSocket(`ws://${location.host}/ws`)
    this.ws.onopen = () => {
      connected.set(true)
      this.reconnectAttempts = 0
    }
    this.ws.onclose = () => {
      connected.set(false)
      this.scheduleReconnect()
    }
    this.ws.onmessage = (event) => {
      const msg = JSON.parse(event.data)
      this.handleMessage(msg)
    }
  }

  private handleMessage(msg: Message) {
    switch (msg.type) {
      case 'reload':
        location.reload()
        break
      case 'slide':
        goToSlide(msg.payload.index)
        break
    }
  }

  private scheduleReconnect() {
    const delay = Math.min(1000 * 2 ** this.reconnectAttempts, this.maxReconnectDelay)
    setTimeout(() => this.connect(), delay)
    this.reconnectAttempts++
  }

  send(msg: Message) {
    this.ws?.send(JSON.stringify(msg))
  }
}

export const wsClient = new WebSocketClient()
```

---

## US-030: Keyboard Navigation

**Description:** As a presenter, I want keyboard navigation so that I can control my presentation without a mouse.

**Acceptance Criteria:**
- [ ] Handle navigation keys:
  - `ArrowRight`, `ArrowDown`, `Space`, `Enter` → next (fragment or slide)
  - `ArrowLeft`, `ArrowUp`, `Backspace` → previous (fragment or slide)
  - `Home` → first slide
  - `End` → last slide
  - Number keys → go to slide N (for quick jumps)
- [ ] Handle view controls:
  - `S` → open presenter view in new window
  - `O` → toggle slide overview
  - `F` → toggle fullscreen
  - `R` → reset timer (presenter view only)
  - `Escape` → close overview/exit fullscreen
- [ ] Prevent default browser behavior for these keys
- [ ] Only activate when no input is focused

**Technical Specifications:**
```typescript
// src/lib/utils/keyboard.ts
export function setupKeyboardNavigation() {
  window.addEventListener('keydown', (e) => {
    // Skip if user is typing in an input
    if (e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement) {
      return
    }

    switch (e.key) {
      case 'ArrowRight':
      case 'ArrowDown':
      case ' ':
      case 'Enter':
        e.preventDefault()
        advance()
        break
      case 'ArrowLeft':
      case 'ArrowUp':
      case 'Backspace':
        e.preventDefault()
        retreat()
        break
      // ... more handlers
    }
  })
}

function advance() {
  if (hasNextFragment()) {
    nextFragment()
  } else {
    nextSlide()
  }
}
```

---

## US-031: Slide Container Component

**Description:** As a frontend developer, I need a slide container component so that slides are rendered with proper aspect ratio and scaling.

**Acceptance Criteria:**
- [ ] Create `SlideContainer.svelte` component
- [ ] Apply aspect ratio from config (16:9, 4:3, 16:10)
- [ ] Scale slide to fit viewport while maintaining ratio
- [ ] Center slide in viewport
- [ ] Apply theme class to container
- [ ] Support fullscreen mode
- [ ] Handle window resize

**Technical Specifications:**
```svelte
<!-- src/lib/components/SlideContainer.svelte -->
<script lang="ts">
  import { presentation } from '$lib/stores/presentation'

  let container: HTMLElement
  let scale = 1

  const aspectRatios = {
    '16:9': 16 / 9,
    '4:3': 4 / 3,
    '16:10': 16 / 10
  }

  function calculateScale() {
    if (!container) return
    const ratio = aspectRatios[$presentation?.config.aspectRatio ?? '16:9']
    const slideWidth = 1920
    const slideHeight = slideWidth / ratio

    const scaleX = container.clientWidth / slideWidth
    const scaleY = container.clientHeight / slideHeight
    scale = Math.min(scaleX, scaleY)
  }

  $effect(() => {
    calculateScale()
    window.addEventListener('resize', calculateScale)
    return () => window.removeEventListener('resize', calculateScale)
  })
</script>

<div class="slide-viewport" bind:this={container}>
  <div
    class="slide-container theme-{$presentation?.config.theme}"
    style:transform="scale({scale})"
  >
    <slot />
  </div>
</div>
```

---

## US-032: Slide Renderer Component

**Description:** As a frontend developer, I need a slide renderer so that slide HTML content is displayed with proper styling.

**Acceptance Criteria:**
- [ ] Create `SlideRenderer.svelte` component
- [ ] Render slide HTML content safely
- [ ] Apply layout-specific styles
- [ ] Handle fragment visibility
- [ ] Support background images
- [ ] Apply slide-specific transitions

**Technical Specifications:**
```svelte
<!-- src/lib/components/SlideRenderer.svelte -->
<script lang="ts">
  import type { Slide } from '$lib/types'

  export let slide: Slide
  export let visibleFragments: number

  $: layoutClass = `layout-${slide.layout}`
  $: backgroundStyle = slide.background
    ? `background-image: url(${slide.background.url})`
    : ''
</script>

<div class="slide {layoutClass}" style={backgroundStyle}>
  {@html slide.html}
</div>

<style>
  .slide {
    width: 1920px;
    height: 1080px;
    padding: 80px;
    box-sizing: border-box;
  }

  .slide :global(h1) { /* ... */ }
  .slide :global(h2) { /* ... */ }
  .slide :global(pre) { /* ... */ }
</style>
```

---

## US-033: Layout Components

**Description:** As a frontend developer, I need layout components so that different slide types are rendered correctly.

**Acceptance Criteria:**
- [ ] Create layout components for each supported layout (10+ layouts):
  - `LayoutTitle.svelte` - centered title with optional subtitle
  - `LayoutSection.svelte` - large section header
  - `LayoutDefault.svelte` - standard content layout
  - `LayoutTwoColumn.svelte` - side-by-side columns
  - `LayoutThreeColumn.svelte` - three-column grid
  - `LayoutCodeFocus.svelte` - full-width code block
  - `LayoutQuote.svelte` - styled blockquote
  - `LayoutBigStat.svelte` - large number emphasis
  - `LayoutCover.svelte` - full-bleed background image
  - `LayoutSidebar.svelte` - main content with sidebar
  - `LayoutSplitMedia.svelte` - image + text side-by-side
  - `LayoutBlank.svelte` - empty canvas for custom content
- [ ] Each layout has appropriate grid/flex structure
- [ ] Layouts are theme-aware (apply theme-specific styles)

---

## US-034: Theme System Implementation

**Description:** As a frontend developer, I need a theme system so that presentations can have different visual styles.

**Acceptance Criteria:**
- [ ] Create theme CSS files for each built-in theme:
  - `minimal.css` - Clean, Apple-style, Helvetica
  - `gradient.css` - Modern, colorful gradients, glassmorphism
  - `terminal.css` - Hacker aesthetic, monospace, CRT effects
  - `brutalist.css` - Bold, geometric, high contrast
  - `keynote.css` - Professional, subtle shadows
- [ ] Each theme defines:
  - CSS custom properties for colors
  - Typography (font families, sizes)
  - Spacing scale
  - Code block styles
  - Default transition
- [ ] Theme applied via class on root container
- [ ] Support custom theme CSS in frontmatter

**Design System Constants:**

Typography Scale (all themes must implement):
| Element | Size |
|---------|------|
| Title | 8rem |
| H1 | 6rem |
| H2 | 4rem |
| Body | 2rem |
| Code | 1.5-2.5rem |

Animation Timing Constants:
| Speed | Duration | Use Case |
|-------|----------|----------|
| Fast | 200-300ms | Subtle transitions, hover states |
| Medium | 400-600ms | Standard transitions, fragment reveals |
| Slow | 800-1200ms | Emphasis animations, slide transitions |
| Stagger | 50-150ms | Delay between cascading items |

**Technical Specifications:**
```css
/* src/lib/themes/minimal.css */
.theme-minimal {
  --color-background: #ffffff;
  --color-text: #1a1a1a;
  --color-text-secondary: #6b7280;
  --color-accent: #3b82f6;
  --color-code-bg: #f3f4f6;

  --font-heading: 'Helvetica Neue', Helvetica, Arial, sans-serif;
  --font-body: 'Helvetica Neue', Helvetica, Arial, sans-serif;
  --font-code: 'SF Mono', Monaco, 'Courier New', monospace;

  --font-size-title: 8rem;
  --font-size-h1: 6rem;
  --font-size-h2: 4rem;
  --font-size-body: 2rem;
  --font-size-code: 1.5rem;

  --spacing-unit: 1rem;
  --transition-default: fade;

  /* Animation timing */
  --duration-fast: 250ms;
  --duration-medium: 400ms;
  --duration-slow: 800ms;
  --stagger-delay: 100ms;
}
```

---

## US-035: Slide Transitions

**Description:** As a presenter, I want smooth transitions between slides so that my presentation feels polished.

**Acceptance Criteria:**
- [ ] Implement 5 transition types using Svelte transitions:
  - `none` - instant switch
  - `fade` - cross-fade (default)
  - `slide` - horizontal slide
  - `push` - push out old slide
  - `zoom` - subtle zoom effect
- [ ] Apply transition from config (global default)
- [ ] Support per-slide transition override
- [ ] Transitions respect reduced motion preferences
- [ ] Transition duration configurable (default 400ms)

**Technical Specifications:**
```typescript
// src/lib/utils/transitions.ts
import { crossfade, fade, fly, scale } from 'svelte/transition'

export const transitions = {
  none: () => ({ duration: 0 }),

  fade: (node, { duration = 400 }) =>
    fade(node, { duration }),

  slide: (node, { duration = 400, direction = 1 }) =>
    fly(node, { x: direction * 100, duration }),

  push: (node, { duration = 400, direction = 1 }) =>
    fly(node, { x: direction * window.innerWidth, duration }),

  zoom: (node, { duration = 400 }) =>
    scale(node, { start: 0.95, duration })
}

export function getTransition(name: string) {
  return transitions[name] ?? transitions.fade
}
```

---

## US-036: Fragment Animation System

**Description:** As a presenter, I want incremental reveals so that I can control the pace of information.

**Acceptance Criteria:**
- [ ] Parse fragment groups from slide data
- [ ] Track current fragment index per slide
- [ ] Hide fragments beyond current index
- [ ] Animate fragment reveal with appropriate effect
- [ ] Support list auto-fragmentation
- [ ] Support `<!-- pause -->` markers
- [ ] Fragment navigation integrates with keyboard

**Technical Specifications:**
```svelte
<!-- src/lib/components/FragmentContainer.svelte -->
<script lang="ts">
  import { fade, fly } from 'svelte/transition'
  import type { FragmentGroup } from '$lib/types'

  export let fragments: FragmentGroup[]
  export let visibleCount: number
</script>

{#each fragments as fragment, i}
  {#if i < visibleCount}
    <div
      class="fragment"
      in:fade={{ duration: 300, delay: i * 50 }}
    >
      {@html fragment.html}
    </div>
  {/if}
{/each}
```

---

## US-037: Code Highlighting with Shiki

**Description:** As a presenter, I want beautiful syntax highlighting so that my code examples are readable.

**Acceptance Criteria:**
- [ ] Install Shiki: `npm install shiki`
- [ ] Build-time highlighting for static code blocks
- [ ] Runtime highlighting for live code (when drivers present)
- [ ] Support VS Code themes (configurable via `codeTheme`)
- [ ] **Lazy load Shiki themes on demand** (don't bundle all themes)
- [ ] Support line highlighting (`{1,3-5}` syntax)
- [ ] Support line numbers (optional)
- [ ] Support code block titles
- [ ] Support code diffs visualization (show added/removed lines)
- [ ] Support multi-step code reveals (progressive disclosure with fragments)
- [ ] Support per-slide font size override via directive
- [ ] **Auto-size large code blocks** to fit on screen gracefully

**Technical Specifications:**
```typescript
// src/lib/utils/highlighting.ts
import { createHighlighter, type Highlighter } from 'shiki'

let highlighter: Highlighter | null = null

export async function initHighlighter(theme: string) {
  highlighter = await createHighlighter({
    themes: [theme],
    langs: ['javascript', 'typescript', 'python', 'sql', 'go', 'rust', 'bash']
  })
}

export function highlight(code: string, lang: string, options?: HighlightOptions) {
  if (!highlighter) {
    throw new Error('Highlighter not initialized')
  }

  return highlighter.codeToHtml(code, {
    lang,
    theme: options?.theme ?? 'github-dark',
    transformers: [
      lineHighlighter(options?.highlightLines),
      lineNumbers(options?.showLineNumbers)
    ]
  })
}
```

---

## US-037b: Terminal Recording Playback (Asciinema)

**Description:** As a presenter, I want to embed terminal recordings so that I can show pre-recorded command-line demos.

**Acceptance Criteria:**
- [ ] Support Asciinema `.cast` file format
- [ ] Embed recordings via code block syntax: ` ```asciinema {src: "./demo.cast"} `
- [ ] Use `asciinema-player` library (loaded as external resource)
- [ ] Treat `.cast` files like images:
  - Resolve relative paths from markdown file directory
  - Copy to `dist/assets/` during build with content hash
  - Rewrite paths in output
- [ ] Playback controls: play, pause, speed adjustment
- [ ] Auto-play option via directive
- [ ] Styling matches theme's terminal aesthetic
- [ ] Graceful fallback if file missing (inline error like images)

**Technical Specifications:**
```typescript
// src/lib/components/AsciinemaPlayer.svelte
<script lang="ts">
  import 'asciinema-player/dist/bundle/asciinema-player.css'
  import * as AsciinemaPlayer from 'asciinema-player'
  import { onMount } from 'svelte'

  export let src: string
  export let autoPlay = false
  export let speed = 1
  export let theme: 'light' | 'dark' = 'dark'

  let container: HTMLElement

  onMount(() => {
    AsciinemaPlayer.create(src, container, {
      autoPlay,
      speed,
      theme,
      fit: 'width'
    })
  })
</script>

<div bind:this={container} class="asciinema-container"></div>
```

---

## US-038: Live Code Execution UI

**Description:** As a presenter, I want to execute code from my slides so that I can show live demos.

**Acceptance Criteria:**
- [ ] Create `LiveCodeBlock.svelte` component
- [ ] Display "Run" button on driver-enabled code blocks
- [ ] Show loading state during execution
- [ ] Display execution result below code
- [ ] Format tabular data as HTML table
- [ ] Show error state with muted styling (not bright red)
- [ ] Allow retry on error (click or keyboard shortcut)
- [ ] Keyboard shortcut to execute (Ctrl/Cmd+Enter)
- [ ] Keyboard shortcut to retry failed execution (Ctrl/Cmd+R when focused)

**Technical Specifications:**
```svelte
<!-- src/lib/components/LiveCodeBlock.svelte -->
<script lang="ts">
  import type { CodeBlock } from '$lib/types'

  export let codeBlock: CodeBlock

  let loading = false
  let result: ExecuteResult | null = null
  let error: string | null = null

  async function execute() {
    loading = true
    error = null

    try {
      const response = await fetch('/api/execute', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          driver: codeBlock.driver,
          code: codeBlock.code,
          connection: codeBlock.connection
        })
      })

      result = await response.json()
      if (!result.success) {
        error = result.error
      }
    } catch (e) {
      error = 'Execution failed'
    } finally {
      loading = false
    }
  }
</script>

<div class="live-code-block">
  <div class="code-content">
    {@html codeBlock.highlighted}
  </div>

  <button onclick={execute} disabled={loading}>
    {loading ? 'Running...' : 'Run'}
  </button>

  {#if result?.success}
    <div class="result">
      {#if result.data}
        <ResultTable data={result.data} />
      {:else}
        <pre>{result.output}</pre>
      {/if}
    </div>
  {/if}

  {#if error}
    <div class="error">{error}</div>
  {/if}
</div>
```

---

## US-039: Presenter View

**Description:** As a presenter, I want a dedicated presenter view so that I can see notes and upcoming slides.

**Acceptance Criteria:**
- [ ] Create presenter view at `/presenter` route
- [ ] Display current slide (compact)
- [ ] Display next slide preview
- [ ] Display speaker notes for current slide
- [ ] Display elapsed timer (click to reset)
- [ ] Display slide counter (e.g., "5 / 24")
- [ ] Touch-friendly controls for tablet
- [ ] Sync with audience view via WebSocket
- [ ] Navigation controls presenter view only

**Technical Specifications:**
```svelte
<!-- src/routes/presenter/+page.svelte -->
<script lang="ts">
  import { currentSlide, currentSlideIndex, totalSlides, nextSlide, prevSlide } from '$lib/stores/presentation'
  import { onMount } from 'svelte'

  let elapsed = 0
  let timerInterval: number

  onMount(() => {
    timerInterval = setInterval(() => elapsed++, 1000)
    return () => clearInterval(timerInterval)
  })

  function resetTimer() {
    elapsed = 0
  }

  function formatTime(seconds: number) {
    const m = Math.floor(seconds / 60)
    const s = seconds % 60
    return `${m}:${s.toString().padStart(2, '0')}`
  }
</script>

<div class="presenter-view">
  <div class="current-slide">
    <SlideRenderer slide={$currentSlide} />
  </div>

  <div class="sidebar">
    <div class="next-slide">
      <h3>Next</h3>
      {#if $currentSlideIndex < $totalSlides - 1}
        <SlideRenderer slide={slides[$currentSlideIndex + 1]} mini />
      {/if}
    </div>

    <div class="notes">
      <h3>Notes</h3>
      <div class="notes-content">
        {$currentSlide?.notes ?? 'No notes for this slide'}
      </div>
    </div>

    <div class="controls">
      <div class="timer" onclick={resetTimer}>
        {formatTime(elapsed)}
      </div>
      <div class="counter">
        {$currentSlideIndex + 1} / {$totalSlides}
      </div>
    </div>
  </div>
</div>
```

---

## US-040: Slide Overview Mode

**Description:** As a presenter, I want a slide overview so that I can quickly navigate to any slide.

**Acceptance Criteria:**
- [ ] Create overview modal component
- [ ] Display all slides as thumbnails in grid
- [ ] Highlight current slide
- [ ] Click thumbnail to navigate to slide
- [ ] Keyboard navigation in overview (arrow keys)
- [ ] Close on Escape or selecting a slide
- [ ] Toggle with `O` key

**Technical Specifications:**
```svelte
<!-- src/lib/components/SlideOverview.svelte -->
<script lang="ts">
  import { presentation, currentSlideIndex, goToSlide } from '$lib/stores/presentation'
  import { fade, scale } from 'svelte/transition'

  export let open = false

  function selectSlide(index: number) {
    goToSlide(index)
    open = false
  }
</script>

{#if open}
  <div class="overview-backdrop" transition:fade onclick={() => open = false}>
    <div class="overview-grid" transition:scale>
      {#each $presentation.slides as slide, i}
        <button
          class="thumbnail"
          class:current={i === $currentSlideIndex}
          onclick={() => selectSlide(i)}
        >
          <SlideRenderer {slide} mini />
          <span class="slide-number">{i + 1}</span>
        </button>
      {/each}
    </div>
  </div>
{/if}
```

---

## US-041: Progress Indicator

**Description:** As a viewer, I want a progress indicator so that I know how far through the presentation we are.

**Acceptance Criteria:**
- [ ] Create progress bar component
- [ ] Display at bottom of slide viewport
- [ ] Show percentage complete based on current slide
- [ ] Subtle styling that doesn't distract
- [ ] Hide in presenter view
- [ ] Optional (can be disabled in config)

---

## US-042: Animation Presets

**Description:** As a presenter, I want built-in animation presets so that my content has engaging motion.

**Acceptance Criteria:**
- [ ] Implement animation presets:
  - `typewriter` - character-by-character text reveal
  - `count-up` - number animation from 0 to target
  - `cascade` - staggered list item entrance
  - `spring` - spring physics for element motion
- [ ] Animations triggered on slide enter
- [ ] Animations can be applied via CSS classes
- [ ] Respect reduced motion preferences

**Technical Specifications:**
```typescript
// src/lib/utils/animations.ts
import { spring } from 'svelte/motion'

export function typewriter(node: HTMLElement, { speed = 50 }) {
  const text = node.textContent ?? ''
  const duration = text.length * speed

  return {
    duration,
    tick: (t: number) => {
      const chars = Math.round(text.length * t)
      node.textContent = text.slice(0, chars)
    }
  }
}

export function countUp(node: HTMLElement, { duration = 1000 }) {
  const target = parseInt(node.textContent ?? '0', 10)

  return {
    duration,
    tick: (t: number) => {
      node.textContent = Math.round(target * t).toLocaleString()
    }
  }
}
```

---

## US-043: Static Build Placeholder

**Description:** As a viewer of a static build, I want graceful handling of live code blocks so that I understand the limitation.

**Acceptance Criteria:**
- [ ] Detect when running in static mode (no WebSocket)
- [ ] Show placeholder message on driver-enabled code blocks
- [ ] Message: "Live execution available in presentation mode"
- [ ] Style placeholder to match theme
- [ ] Hide Run button in static mode

---

## US-044: Disconnection Indicator

**Description:** As a presenter, I want to know when the connection is lost so that I can troubleshoot.

**Acceptance Criteria:**
- [ ] Show subtle indicator when WebSocket disconnects
- [ ] Position in corner of slide (non-intrusive)
- [ ] Indicate reconnection attempts
- [ ] Hide when reconnected
- [ ] Don't show in static mode

---

# Part 3: Integration

## US-045: Frontend Build Integration

**Description:** As a developer, I need the frontend build to integrate with Go's embed system so that the final binary is self-contained.

**Acceptance Criteria:**
- [ ] Create build script that:
  1. Runs `npm run build` in `frontend/`
  2. Copies output to `embedded/`
  3. Runs `go generate` to update embedded assets
- [ ] Go binary embeds all frontend assets
- [ ] Dev mode serves embedded assets
- [ ] Build mode includes assets in static output
- [ ] Makefile target: `make build` handles full process

---

## US-046: End-to-End Presentation Flow

**Description:** As a user, I need the complete flow from markdown to rendered presentation to work seamlessly.

**Acceptance Criteria:**
- [ ] Create sample presentation in `testdata/sample.md`
- [ ] Verify flow:
  1. `tap dev testdata/sample.md` starts server
  2. Browser shows rendered slides
  3. File edit triggers hot reload
  4. Keyboard navigation works
  5. Live code execution works
  6. Presenter mode syncs with audience
- [ ] Integration test covering full flow
- [ ] E2E test with Playwright

---

## US-046b: Svelte Component Tests

**Description:** As a developer, I need unit tests for Svelte components to ensure UI components work correctly in isolation.

**Acceptance Criteria:**
- [ ] Set up Vitest with Svelte testing library
- [ ] Test cases for each component:
  - SlideRenderer: renders HTML content, applies layout classes
  - SlideContainer: calculates scale, handles resize
  - FragmentContainer: shows/hides fragments correctly
  - LiveCodeBlock: loading states, error states, result display
  - PresenterView: timer, notes, next slide preview
  - SlideOverview: grid layout, navigation
- [ ] Test animation triggers and timing
- [ ] Test theme class application
- [ ] Test code block rendering with Shiki
- [ ] Achieve >80% component coverage

---

## US-047: Browser-Based E2E Tests

**Description:** As a developer, I need E2E tests to verify the presentation experience works correctly in browsers.

**Acceptance Criteria:**
- [ ] Set up Playwright for E2E testing
- [ ] Test cases:
  - Slide navigation (keyboard)
  - Slide navigation (URL hash)
  - Fragment reveals
  - Presenter mode sync
  - Live code execution
  - Hot reload
  - Theme rendering
  - Responsive scaling
- [ ] Visual regression tests for each theme
- [ ] Cross-browser testing (Chrome, Firefox, Safari)
- [ ] CI integration

---

## US-048: Performance Benchmarks

**Description:** As a developer, I need performance benchmarks to ensure we meet speed targets.

**Acceptance Criteria:**
- [ ] Benchmark: Parse 100-slide presentation (<100ms)
- [ ] Benchmark: Build 50-slide presentation (<2s)
- [ ] Benchmark: Animation frame rate (60fps target)
- [ ] Benchmark: Hot reload latency (<200ms)
- [ ] Add benchmarks to CI
- [ ] Document performance baselines

---

## US-049: Cross-Platform Binary Build

**Description:** As a developer, I need automated cross-platform builds so that users can download binaries for their OS.

**Acceptance Criteria:**
- [ ] Configure GoReleaser for automated releases
- [ ] Build targets:
  - `darwin/amd64` (macOS Intel)
  - `darwin/arm64` (macOS Apple Silicon)
  - `linux/amd64`
  - `linux/arm64`
  - `windows/amd64`
- [ ] Include embedded frontend in all builds
- [ ] Generate checksums
- [ ] Create GitHub release with artifacts
- [ ] Test binaries on each platform

---

## US-050: Documentation and Examples

**Description:** As a user, I need documentation and examples so that I can learn how to use Tap effectively.

**Acceptance Criteria:**
- [ ] Create README.md with:
  - Quick start guide
  - Installation instructions
  - Basic usage examples
  - Command reference
- [ ] Create `examples/` directory with:
  - Basic presentation
  - Code-focused presentation
  - Live SQL demo presentation
  - Each theme showcase
- [ ] Add inline help to CLI commands
- [ ] Create CONTRIBUTING.md for contributors

---

# Functional Requirements Summary

| ID | Requirement |
|----|-------------|
| FR-001 | Parse markdown with YAML frontmatter |
| FR-002 | Split slides on `---` delimiter |
| FR-003 | Parse local directives in HTML comments |
| FR-004 | Resolve environment variables in config |
| FR-005 | Auto-detect slide layouts from content |
| FR-006 | Execute code via driver registry system |
| FR-007 | Support shell, SQLite, MySQL, PostgreSQL, and custom drivers |
| FR-008 | Serve presentation via HTTP with WebSocket |
| FR-009 | Hot reload on file changes |
| FR-010 | Build to static HTML/CSS/JS bundle |
| FR-011 | Export to PDF via Playwright |
| FR-012 | Interactive TUI for dev server |
| FR-013 | Interactive slide builder |
| FR-014 | 5 built-in themes with distinct aesthetics |
| FR-015 | 5 slide transitions |
| FR-016 | Fragment-based incremental reveals |
| FR-017 | Syntax highlighting via Shiki |
| FR-018 | Keyboard navigation |
| FR-019 | Presenter mode with notes and timer |
| FR-020 | Slide overview/thumbnail navigation |
| FR-021 | QR code for presenter URL |
| FR-022 | Password protection for presenter view |
| FR-023 | Single binary distribution |
| FR-024 | Image handling with sizing/positioning attributes |
| FR-025 | Code diffs visualization |
| FR-026 | Terminal recording playback (Asciinema) |
| FR-027 | 10+ layouts including three-column, sidebar, split-media |
| FR-028 | Graceful missing image handling (inline error, not build failure) |
| FR-029 | Custom driver support via frontmatter configuration |
| FR-030 | Auto-sizing for large code blocks to fit on screen |
| FR-031 | Lazy loading of Shiki themes on demand |

---

# Non-Goals (Out of Scope)

- **Remote control app** - No mobile app for presentation control (Phase 2)
- **Drawing/annotation** - No live drawing tools (Phase 2)
- **Collaborative editing** - No real-time multi-user editing (Phase 2)
- **Theme marketplace** - No online theme sharing (Phase 3)
- **Cloud sync** - No cloud storage integration (Phase 3)
- **Recording/streaming** - No video recording capability (Phase 3)
- **Web-based editor** - No browser-based authoring (Phase 4)
- **AI slide generation** - No AI content creation (Phase 4)
- **Telemetry** - Absolutely no data collection

---

# Technical Considerations

## Dependencies

**Go:**
- `github.com/spf13/cobra` - CLI framework
- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/yuin/goldmark` - Markdown parsing
- `nhooyr.io/websocket` - WebSocket server
- `github.com/fsnotify/fsnotify` - File watching
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - TUI styling
- `github.com/skip2/go-qrcode` - QR code generation
- `github.com/playwright-community/playwright-go` - PDF export
- `github.com/joho/godotenv` - Environment variable loading
- `github.com/fatih/color` - Terminal colors

**Frontend:**
- `svelte@5` - UI framework
- `vite@6` - Build tool
- `shiki` - Syntax highlighting
- `typescript@5` - Type safety

## Security Considerations

- Driver code execution has no restrictions (presenter controls their own slides)
- Database credentials stored in environment variables, never in output
- Presenter password transmitted via query parameter (acceptable for local network)
- No eval() or dynamic code execution in frontend
- Static builds contain no credentials

## Performance Targets

- Parse 100 slides: <100ms
- Build 50 slides: <2 seconds
- Hot reload latency: <200ms
- Animation frame rate: 60fps
- Binary size: <50MB
- Large presentation handling: 100+ slides without degradation

## Installation Methods

1. **Direct download:** Pre-built binaries from GitHub releases
2. **Homebrew:** `brew install tap-slides`
3. **Go install:** `go install github.com/tapsh/tap@latest`

## Licensing & Branding

- **License:** MIT
- **Branding:**
  - "Tap" in prose and documentation
  - `tap` for CLI commands
  - `tap.sh` for domain/website
- **Versioning:** Semantic versioning (semver)

---

# Success Metrics

- Time to first presentation: <5 minutes
- Lines of CSS for customization: <50 for most users
- Build time for 50 slides: <2 seconds
- Test coverage: >80%
- Zero external runtime dependencies for end users

---

# Resolved Questions

1. **Custom drivers:** Simple custom driver system supported in MVP (see US-010b)
2. **Shiki theme bundling:** Lazy load themes on demand
3. **Presenter password:** Query parameter approach is acceptable for MVP
4. **Large code blocks:** Graceful auto-sizing to fit on screen
5. **Asciinema files:** Bundled in static build like images, copied to `dist/assets/`
6. **Asciinema player:** Use `asciinema-player` library, loaded as external resource
