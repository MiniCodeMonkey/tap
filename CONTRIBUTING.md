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

The config is `.golangci.yml` in the **v2** schema, so you need
golangci-lint v2. staticcheck's `QF1012` is excluded deliberately; leave it
that way rather than rewriting the lines it flags.

## Continuous integration

`.github/workflows/ci.yml` runs on every push and pull request against
`main`, as four jobs:

- **Go Tests** - builds the frontend, runs `go vet ./...` and `go test
  ./...` (with a real Chromium, installed by playwright-go), then
  `golangci-lint`. Reproduce locally with:

  ```bash
  cd frontend && npm ci && npm run build && cd ..
  go vet ./...
  go test ./...
  go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run ./...
  ```

- **Frontend Tests** - unit tests, the TypeScript check, ESLint, and the
  theme tokens check. Reproduce from `frontend/`:

  ```bash
  npm ci
  npm run test
  npm run check
  npm run lint
  npm run tokens:check
  ```

- **E2E Tests** - builds the frontend and the `tap` binary (`make
  build`), then runs the Playwright suite in `frontend/`, which starts
  its own dev server on port 3100. Reproduce with:

  ```bash
  make build
  cd frontend && npx playwright install --with-deps chromium
  npx playwright test
  ```

- **Theme Checks** - runs `frontend/e2e-themes/` against every theme,
  with `TAP_THEME_SNAPSHOTS=off` so it skips only the visual snapshot
  comparisons (the checked-in baselines were rendered on macOS and don't
  match Linux font rendering); overflow, clipping, minimum text size,
  contrast, and theme isolation all still run. Reproduce locally with the
  two servers from "Testing the Theme Suite" above, then:

  ```bash
  cd frontend
  TAP_THEME_SNAPSHOTS=off BASE_URL=http://localhost:5300 npx playwright test -c playwright.themes.config.ts
  ```

  Drop `TAP_THEME_SNAPSHOTS=off` to also check snapshots, but only on
  macOS, against the committed baselines.

## Testing the Theme Suite

Beyond the main Go and frontend test suites, `frontend/e2e-themes/` runs a
per-theme check suite (overflow, contrast, minimum text size, isolation,
and visual snapshots) against a running Go server and a running Vite
server, each on their own pair of ports so multiple runs can work on
different themes in parallel without fighting over a snapshot folder.
Start both, then run the suite with `BASE_URL` pointing at the Vite server:

The dev server checks both the request's `Host` header against a
local-and-private allow-list and the websocket's `Origin` against that
host. The proxy setup below satisfies both, because the browser talks to
the Vite server on `localhost` and Vite forwards with a matching `Host`, so
this workflow needs no extra flag.

You need `tap dev --allow-origin <value>` (repeatable) when something else
is true: a dev server that talks to the hub directly rather than through
the proxy (pass its origin, `http://localhost:<port>`), or reaching `tap
dev` through a custom DNS name or a tunnel such as ngrok or Tailscale (pass
the host). LAN IP addresses, `localhost`, and `.local` names are already
allowed.

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

The temporary servers behind `tap export pdf` and `tap export images` bind
`127.0.0.1`, since only tap's own headless browser talks to them. `tap dev`
and `tap present` also bind `127.0.0.1` by default; `--lan` switches them
to `0.0.0.0` so a presenter can open the deck from another device.

## Changelog preparation

`scripts/prepare-changelog.sh` holds the changelog logic the release
workflow runs, so you can test a release's changelog handling without
triggering a release. It covers four cases, each with a Go test in
`scripts/prepare_changelog_test.go`:

- **First run for a version:** the `## [Unreleased]` section's contents
  move into a new `## [<version>] - <date>` section, and `Unreleased` is
  left empty.
- **Second run for the same version:** the existing section for that
  version is reused rather than duplicated, so re-running the workflow is
  safe.
- **An empty `Unreleased` section with no section for the version:** the
  script fails, rather than cutting a release with no notes.
- **Reusing a section that is the last thing in the file:** the notes come
  out whole, with no dropped final line.

It takes the version, the changelog to rewrite in place, and the file to
write the extracted notes to. Set `CHANGELOG_DATE` to pin the new header's
date, which is how the tests keep fixtures independent of today.

Run it against a copy, never the real file:

```bash
cp CHANGELOG.md /tmp/CHANGELOG-test.md
CHANGELOG_DATE=2026-01-01 scripts/prepare-changelog.sh 9.9.9 \
  /tmp/CHANGELOG-test.md /tmp/notes.md

# second run for the same version reuses the section it already wrote
CHANGELOG_DATE=2026-01-01 scripts/prepare-changelog.sh 9.9.9 \
  /tmp/CHANGELOG-test.md /tmp/notes.md

# a new version with an empty Unreleased section fails, exit status 1
scripts/prepare-changelog.sh 8.8.8 /tmp/CHANGELOG-test.md /tmp/notes.md
```

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
│   ├── pdf/          # Headless browser capture for tap export pdf and tap export images
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

## The desktop app

`desktop/` holds Tap Desktop, the native macOS app. It bundles the `tap`
binary built from the same commit. See `desktop/README.md` for the build and
test commands. The Xcode project is generated from `desktop/project.yml` with
XcodeGen, so add new files to the folder and run `make -C desktop project`.

Every scenario in `docs/superpowers/specs/tap-desktop-features/` that the app
claims in `desktop/scenarios.txt` must have a test named after it. CI runs
`make -C desktop check-scenarios`, the package tests and the hosted tests. The
UI tests and the benchmarks run locally.
