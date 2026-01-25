#!/usr/bin/env bash
#
# Release script for Tap
# Updates CHANGELOG.md, creates a git tag, and publishes a GitHub release.
#
# Usage: ./scripts/release.sh <version>
# Example: ./scripts/release.sh 1.0.0

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Get the project root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CHANGELOG="$PROJECT_ROOT/CHANGELOG.md"

# Validate arguments
if [[ $# -lt 1 ]]; then
    echo -e "${RED}Error: Version argument required${NC}"
    echo "Usage: $0 <version>"
    echo "Example: $0 1.0.0"
    exit 1
fi

VERSION="$1"
TAG="v$VERSION"
DATE=$(date +%Y-%m-%d)

# Validate version format (semver)
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
    echo -e "${RED}Error: Invalid version format '$VERSION'${NC}"
    echo "Expected semantic version (e.g., 1.0.0, 1.0.0-beta.1)"
    exit 1
fi

# Check for uncommitted changes
if ! git diff --quiet || ! git diff --cached --quiet; then
    echo -e "${RED}Error: You have uncommitted changes${NC}"
    echo "Please commit or stash your changes before releasing."
    exit 1
fi

# Check if tag already exists
if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo -e "${RED}Error: Tag $TAG already exists${NC}"
    exit 1
fi

# Check if gh CLI is available
if ! command -v gh &> /dev/null; then
    echo -e "${RED}Error: GitHub CLI (gh) is not installed${NC}"
    echo "Install it from: https://cli.github.com/"
    exit 1
fi

# Check if authenticated with gh
if ! gh auth status &> /dev/null; then
    echo -e "${RED}Error: Not authenticated with GitHub CLI${NC}"
    echo "Run: gh auth login"
    exit 1
fi

echo -e "${GREEN}Preparing release $TAG${NC}"

# Check if CHANGELOG has [Unreleased] section
if ! grep -q "## \[Unreleased\]" "$CHANGELOG"; then
    echo -e "${RED}Error: No [Unreleased] section found in CHANGELOG.md${NC}"
    exit 1
fi

# Extract unreleased changes for release notes
RELEASE_NOTES=$(sed -n '/## \[Unreleased\]/,/## \[/p' "$CHANGELOG" | sed '1d;$d' | sed '/^$/d')

if [[ -z "$RELEASE_NOTES" ]]; then
    echo -e "${YELLOW}Warning: No changes found under [Unreleased]${NC}"
    read -p "Continue with empty release notes? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo -e "${GREEN}Release notes:${NC}"
echo "$RELEASE_NOTES"
echo ""
read -p "Proceed with release? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Release cancelled."
    exit 0
fi

# Update CHANGELOG.md
echo -e "${GREEN}Updating CHANGELOG.md...${NC}"

# Create the new version header
NEW_HEADER="## [$VERSION] - $DATE"

# Replace [Unreleased] with new version and add fresh [Unreleased] section
sed -i.bak "s/## \[Unreleased\]/## [Unreleased]\n\n$NEW_HEADER/" "$CHANGELOG"
rm -f "$CHANGELOG.bak"

# Commit the changelog update
echo -e "${GREEN}Committing changelog...${NC}"
git add "$CHANGELOG"
git commit -m "chore: release $TAG"

# Create the git tag
echo -e "${GREEN}Creating tag $TAG...${NC}"
git tag -a "$TAG" -m "Release $TAG"

# Push commit and tag
echo -e "${GREEN}Pushing to remote...${NC}"
git push
git push origin "$TAG"

# Create GitHub release
echo -e "${GREEN}Creating GitHub release...${NC}"
gh release create "$TAG" \
    --title "$TAG" \
    --notes "$RELEASE_NOTES"

echo ""
echo -e "${GREEN}Release $TAG complete!${NC}"
echo ""
echo "Next steps:"
echo "  - View release: gh release view $TAG"
echo "  - Build binaries: make release VERSION=$VERSION"
echo "  - Upload binaries: gh release upload $TAG bin/*"
