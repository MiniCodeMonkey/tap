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

Releases are managed through the `make tag` command, which automates the entire process.

### Prerequisites

- [GitHub CLI](https://cli.github.com/) installed and authenticated (`gh auth login`)
- No uncommitted changes in your working directory
- Changes documented in `CHANGELOG.md` under `[Unreleased]`

### Creating a Release

```bash
# Create a release (e.g., version 1.0.0)
make tag VERSION=1.0.0
```

This command will:

1. **Validate** the version format (must be semver: `X.Y.Z` or `X.Y.Z-label`)
2. **Check** for uncommitted changes (fails if any exist)
3. **Extract** release notes from the `[Unreleased]` section in `CHANGELOG.md`
4. **Show** the release notes and ask for confirmation
5. **Update** `CHANGELOG.md`:
   - Converts `[Unreleased]` to `[X.Y.Z] - YYYY-MM-DD`
   - Adds a fresh `[Unreleased]` section
6. **Commit** the changelog update
7. **Create** a git tag `vX.Y.Z`
8. **Push** the commit and tag to origin
9. **Create** a GitHub release with the extracted notes

### Building and Uploading Binaries

After creating the release:

```bash
# Build binaries for all platforms
make release VERSION=1.0.0

# Upload binaries to the GitHub release
gh release upload v1.0.0 bin/tap-*
```

### Version Format

Use [Semantic Versioning](https://semver.org/):

- `1.0.0` - Major.Minor.Patch
- `1.0.0-beta.1` - Pre-release versions
- `1.0.0-rc.1` - Release candidates

### Example Workflow

```bash
# 1. Make sure all changes are committed
git status

# 2. Verify changelog has unreleased changes
cat CHANGELOG.md

# 3. Create the release
make tag VERSION=1.2.0

# 4. Build binaries
make release VERSION=1.2.0

# 5. Upload binaries to GitHub
gh release upload v1.2.0 bin/tap-*

# 6. Verify the release
gh release view v1.2.0
```

### Troubleshooting

| Error | Solution |
|-------|----------|
| "You have uncommitted changes" | Commit or stash changes first |
| "Tag vX.Y.Z already exists" | Choose a different version number |
| "No [Unreleased] section found" | Add changes to CHANGELOG.md first |
| "Not authenticated with GitHub CLI" | Run `gh auth login` |

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
