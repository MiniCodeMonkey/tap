# Contributing to Tap

## Development Setup

```bash
# Clone the repository
git clone https://github.com/MiniCodeMonkey/tap.git
cd tap

# Install dependencies
make deps
cd frontend && npm install && cd ..

# Build
make build

# Run tests
make test
```

The frontend is React 19 with Vite. `make build` compiles it into
`embedded/dist`, which the Go binary embeds, so a frontend change is not
visible to `tap dev` or `tap build` until you rebuild.

From `frontend/`:

```bash
npm run check    # TypeScript
npm run test     # unit tests (Vitest)
npm run lint     # ESLint
npm run build    # production bundle
npm run test:e2e # end-to-end suite (Playwright)
npm run tokens   # regenerate internal/themes/tokens.json from each theme.css
npm run tokens:check   # fail if tokens.json is out of date
```

`internal/themes/tokens.json` is generated and committed. Change a theme's
root token block and you must run `npm run tokens` and commit the result;
a Go test and `npm run tokens:check` both guard it.

## Making Changes

1. Create a branch for your changes
2. Make your changes
3. Run tests: `make test`
4. Run linter: `make lint`
5. Submit a pull request

> **Known setup note:** `make lint` runs `golangci-lint`, which some local
> toolchains fail to run when it was built against an older Go version than
> `go.mod` targets. If that happens, fall back to `go vet ./...` and
> `go test ./...` for the Go side.

## Testing the Theme Suite

Beyond the main Go and frontend test suites, `frontend/e2e-themes/` runs a
per-theme check suite (overflow, contrast, minimum text size, isolation,
and visual snapshots) against a running Go server and a running Vite
server, each on their own pair of ports so multiple runs can work on
different themes in parallel without fighting over a snapshot folder.
Start both, then run the suite with `BASE_URL` pointing at the Vite server:

```bash
# terminal 1, from the repo root: the Go dev server
go run ./cmd/tap dev testdata/themes.md --port 3300 --headless

# terminal 2, from frontend/: the Vite dev server, proxying to the server above
cd frontend && TAP_API_PORT=3300 npx vite --port 5300

# terminal 3, from frontend/: the theme suite
cd frontend && BASE_URL=http://localhost:5300 npm run test:themes
```

Add `THEME=<slug>` to run the suite against a single theme instead of all
of them. See `docs/reference/theme-porting.md` for the full workflow.

## WebSocket hub state retention

The dev server's hub keeps the deck's last-known slide/fragment state for
`server.DefaultStateRetention` (10 minutes) after the last client
disconnects, so reloading the only open window doesn't lose the current
fragment or step. Internal/CI knob, not a user-facing flag: set
`TAP_HUB_STATE_RETENTION` (a Go duration string, e.g. `0s`, `30s`, `5m`) on
the process running `tap dev` to override it. The main `frontend/e2e/`
suite shares one dev server across every spec file, so its
`playwright.config.ts` webServer command sets `TAP_HUB_STATE_RETENTION=0s`
to keep specs isolated - without it, a spec's viewer disconnecting would
leave state a later spec's "no state" assertions could see.

## Ports

`tap dev` and `tap serve` bind before printing anything. A busy explicit
`--port` fails with exit status 1; a busy default port falls forward to the
next free one, up to 20 above it, and the URL that actually bound is
printed. Two dev servers can therefore run side by side with no flags,
though naming a port keeps it obvious which is which.

The temporary servers behind `tap pdf` and `tap screenshot` bind
`127.0.0.1`, since only tap's own headless browser talks to them. `tap dev`
keeps `0.0.0.0` so a presenter can open it from another device.

## The end-to-end suite's server

`frontend/e2e/` runs against its own `tap dev` on **port 3100**, started by
Playwright's `webServer` block with `reuseExistingServer: false`, so the
suite always starts and owns the server it tests. Running a `tap dev` of
your own on the default port 3000 does not disturb the tests.

## Changelog

We maintain a changelog at `CHANGELOG.md` in the repository root. When making changes:

1. Add your changes under the `[Unreleased]` section
2. Use these categories:
   - **Added** - New features
   - **Changed** - Changes to existing functionality
   - **Deprecated** - Features that will be removed
   - **Removed** - Removed features
   - **Fixed** - Bug fixes
   - **Security** - Security fixes

Example:

```markdown
## [Unreleased]

### Added

- New feature description here

### Fixed

- Bug fix description here
```

## Release Process

Releases are fully automated via GitHub Actions.

### Creating a Release

1. Ensure your changes are documented in `CHANGELOG.md` under `[Unreleased]`
2. Go to **Actions** → **Release** → **Run workflow**
3. Enter the version number (e.g., `1.0.0`)
4. Click **Run workflow**

That's it! The workflow automatically:

- Validates the version format
- Extracts release notes from `[Unreleased]`
- Updates `CHANGELOG.md` with the versioned section
- Commits and pushes the changelog update
- Creates the git tag
- Builds binaries for all platforms (macOS, Linux, Windows)
- Creates the GitHub release with binaries attached

### Version Format

Use [Semantic Versioning](https://semver.org/):

- `1.0.0` - Stable release
- `1.0.0-beta.1` - Pre-release (marked as prerelease on GitHub)
- `1.0.0-rc.1` - Release candidate

### Homebrew

The release workflow automatically updates the Homebrew tap for stable releases (not pre-releases).

**One-time setup:**

1. Create the tap repo (e.g., `MiniCodeMonkey/homebrew-tap`)
2. Copy `.github/homebrew-formula-template.rb` to `Formula/tap.rb` in the tap repo
3. Add a repository secret `HOMEBREW_TAP_TOKEN`:
   - Create a [Personal Access Token](https://github.com/settings/tokens) with `repo` scope
   - Add it as a secret in the main tap repo: Settings → Secrets → Actions
4. (Optional) Set repository variable `HOMEBREW_TAP_REPO` if not using `MiniCodeMonkey/homebrew-tap`

Users can then install with:
```bash
brew install MiniCodeMonkey/tap/tap
```

### Local Release (Alternative)

For local releases without GitHub Actions:

```bash
# Create release (updates changelog, tags, creates GH release)
make tag VERSION=1.0.0

# Build and upload binaries
make release VERSION=1.0.0
gh release upload v1.0.0 bin/tap-*
```

Requires [GitHub CLI](https://cli.github.com/) installed and authenticated.

## Project Structure

```
tap/
├── cmd/tap/          # CLI entrypoint
├── internal/         # Internal packages
│   ├── cli/          # Command implementations
│   ├── parser/       # Markdown parser (slots, fragments, directives, notes)
│   ├── transformer/  # Parsed slides to the frontend's slide JSON
│   ├── layouts/      # The built-in layout and slot list, plus validation
│   ├── components/   # esbuild bundler for deck-supplied React components
│   ├── themes/       # The built-in theme list and generated tokens.json
│   ├── builder/      # HTML builder
│   ├── server/       # Dev server
│   ├── pdf/          # Headless browser capture for tap pdf and tap screenshot
│   ├── tui/          # Terminal UI
│   └── gemini/       # Gemini API client
├── frontend/         # React frontend (frontend/src/lib/themes/ has the 21 themes)
├── docs/             # VitePress documentation
├── scripts/          # Build and release scripts
└── examples/         # Example presentations
```

## Code Style

- Go: Follow standard Go conventions, run `golangci-lint`
- TypeScript/React: Prettier formatting
- Markdown: One sentence per line in documentation
