# Contributing to Tap

## Development Setup

```bash
# Clone the repository
git clone https://github.com/tap-slides/tap.git
cd tap

# Install dependencies
make deps
cd frontend && npm install && cd ..

# Build
make build

# Run tests
make test
```

## Making Changes

1. Create a branch for your changes
2. Make your changes
3. Run tests: `make test`
4. Run linter: `make lint`
5. Submit a pull request

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
│   ├── parser/       # Markdown parser
│   ├── builder/      # HTML builder
│   ├── server/       # Dev server
│   ├── tui/          # Terminal UI
│   └── gemini/       # Gemini API client
├── frontend/         # Svelte frontend
├── docs/             # VitePress documentation
├── scripts/          # Build and release scripts
├── themes/           # Theme definitions
└── examples/         # Example presentations
```

## Code Style

- Go: Follow standard Go conventions, run `golangci-lint`
- TypeScript/Svelte: Prettier formatting
- Markdown: One sentence per line in documentation
