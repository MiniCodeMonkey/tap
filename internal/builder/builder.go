// Package builder generates static files for tap presentations.
package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MiniCodeMonkey/tap/embedded"
	"github.com/MiniCodeMonkey/tap/internal/components"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// ErrAllSlidesSkipped is returned by Build when every slide in the
// presentation has skip: true, so the built deck would have no slides to
// show. Both export pdf and export images reject the same deck the same
// way (see internal/cli/export_pdf.go and internal/cli/export_images.go);
// callers turn this into the same user-facing invalid_deck error.
var ErrAllSlidesSkipped = errors.New("every slide has skip: true, so there is nothing to export")

// BuildResult contains statistics about the completed build.
type BuildResult struct {
	OutputDir string        // Output directory path
	BuildTime time.Duration // Total build duration
	FileCount int           // Number of files generated
	TotalSize int64         // Total size of all files in bytes
	Warnings  []string      // One entry per referenced file Build could not find
}

// Builder generates static files from a tap presentation.
type Builder struct {
	outputDir  string
	baseDir    string // Base directory for resolving relative paths
	components map[string]components.Result
}

// New creates a new Builder with the default output directory "dist".
func New() *Builder {
	return &Builder{
		outputDir: "dist",
	}
}

// NewWithOutput creates a new Builder with a custom output directory.
func NewWithOutput(outputDir string) *Builder {
	return &Builder{
		outputDir: outputDir,
	}
}

// SetBaseDir sets the base directory for resolving relative paths.
func (b *Builder) SetBaseDir(baseDir string) {
	b.baseDir = baseDir
}

// SetComponents provides the build result for every distinct component
// path the presentation's slides use (see internal/components.Resolve), so
// Build can write each bundle to dist/components/ and point the embedded
// presentation JSON at it.
func (b *Builder) SetComponents(resolved map[string]components.Result) {
	b.components = resolved
}

// SetOutputDir sets the output directory for the build.
func (b *Builder) SetOutputDir(outputDir string) {
	b.outputDir = outputDir
}

// OutputDir returns the configured output directory.
func (b *Builder) OutputDir() string {
	return b.outputDir
}

// Build generates static files for the given presentation.
// It copies the embedded Vite-built frontend (JS, CSS, fonts) and creates
// an index.html with the presentation JSON embedded, so themes render correctly.
func (b *Builder) Build(cfg *config.Config, pres *parser.Presentation) (*BuildResult, error) {
	startTime := time.Now()
	result := &BuildResult{
		OutputDir: b.outputDir,
	}

	// Transform presentation to frontend-ready format. Component bundle
	// URLs are relative ("components/<name>-<hash>.js"), the same way
	// image and asciinema paths below are made relative, so the built
	// folder works when served from any base path.
	trans := transformer.NewWithBaseDir(cfg, b.baseDir)
	trans.SetComponents(b.components)
	trans.SetComponentURLPrefix("components/")
	// A slide whose skip directive is true is left out of the built deck
	// entirely, not just hidden, so its content is not published. Check
	// this before creating the output directory or copying any assets, so
	// a deck that cannot be built leaves nothing behind, the same way
	// export pdf and export images do.
	transformed, _ := transformer.WithoutSkippedSlides(trans.Transform(pres))
	if len(transformed.Slides) == 0 {
		return nil, ErrAllSlidesSkipped
	}

	// Create output directory structure
	if err := os.MkdirAll(b.outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	assetsDir := filepath.Join(b.outputDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create assets directory: %w", err)
	}

	// Copy embedded frontend assets (JS, CSS, fonts) for proper theme rendering
	assetCount, assetSize, err := b.CopyEmbeddedAssets()
	if err != nil {
		return nil, fmt.Errorf("failed to copy frontend assets: %w", err)
	}
	result.FileCount += assetCount
	result.TotalSize += assetSize

	// Write every successfully built component bundle to dist/components/.
	componentCount, componentSize, err := b.writeComponentBundles()
	if err != nil {
		return nil, fmt.Errorf("failed to write component bundles: %w", err)
	}
	result.FileCount += componentCount
	result.TotalSize += componentSize

	// Find and copy all referenced images, building a path mapping
	pathMapping := make(map[string]string)
	for i := range transformed.Slides {
		slide := &transformed.Slides[i]
		images := extractImagePaths(slide.HTML)
		for _, imgPath := range images {
			if _, exists := pathMapping[imgPath]; exists {
				continue // Already processed
			}

			// Skip absolute URLs
			if isAbsoluteURL(imgPath) {
				continue
			}

			// Resolve the image path.
			// The transformer converts relative paths to /local/... URLs for the dev server.
			// Strip this prefix to resolve the actual file path on disk.
			resolvedPath := strings.TrimPrefix(imgPath, "/local/")

			sourcePath, reportedPath, err := b.resolveImageSourcePath(resolvedPath)
			if err != nil {
				// A file genuinely missing after decoding is reported,
				// not silently dropped, so a broken image has a reason
				// instead of just disappearing.
				result.Warnings = append(result.Warnings, fmt.Sprintf("image not found: %s", reportedPath))
				continue
			}

			// A deck cannot reach a file outside its own folder, whether
			// through "../", that path's encoded form, an absolute path,
			// or a symlink inside the folder that targets something
			// outside it.
			confinedPath, err := AssetWithinBaseDir(b.baseDir, sourcePath)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("image resolves outside the deck's folder, skipped: %s", reportedPath))
				continue
			}

			// Copy the image with content hash
			hashedPath, size, err := b.copyWithHash(confinedPath, assetsDir)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("image not found: %s", reportedPath))
				continue
			}

			pathMapping[imgPath] = hashedPath
			result.TotalSize += size
			result.FileCount++
		}
	}

	// Find and copy all referenced .cast files for asciinema blocks
	for i := range transformed.Slides {
		slide := &transformed.Slides[i]
		castPaths := extractAsciinemaPaths(slide.HTML)
		for _, castPath := range castPaths {
			if _, exists := pathMapping[castPath]; exists {
				continue
			}
			if isAbsoluteURL(castPath) {
				continue
			}

			resolvedPath := strings.TrimPrefix(castPath, "/local/")

			sourcePath := resolvedPath
			if !filepath.IsAbs(resolvedPath) && b.baseDir != "" {
				sourcePath = filepath.Join(b.baseDir, resolvedPath)
			}
			if _, statErr := os.Stat(sourcePath); statErr != nil {
				continue
			}

			// Recording paths are not decoded the way image paths are
			// (see resolveImageSourcePath); this only refuses a plain or
			// absolute traversal attempt, not one written in its
			// encoded form, since that form is never decoded here.
			confinedPath, err := AssetWithinBaseDir(b.baseDir, sourcePath)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("recording resolves outside the deck's folder, skipped: %s", resolvedPath))
				continue
			}

			hashedPath, size, err := b.copyWithHash(confinedPath, assetsDir)
			if err != nil {
				continue
			}

			pathMapping[castPath] = hashedPath
			result.TotalSize += size
			result.FileCount++
		}
	}

	// Rewrite image and asciinema paths in transformed slides
	for i := range transformed.Slides {
		slide := &transformed.Slides[i]
		slide.HTML = rewriteImagePaths(slide.HTML, pathMapping)
		slide.HTML = rewriteAsciinemaPaths(slide.HTML, pathMapping)
	}

	// Generate index.html with embedded presentation JSON
	indexPath := filepath.Join(b.outputDir, "index.html")
	indexSize, err := b.generateIndexHTML(indexPath, transformed)
	if err != nil {
		return nil, fmt.Errorf("failed to generate index.html: %w", err)
	}
	result.FileCount++
	result.TotalSize += indexSize

	result.BuildTime = time.Since(startTime)
	return result, nil
}

// writeComponentBundles writes every successfully built component bundle's
// JavaScript, CSS when it has any, and any emitted (not inlined) asset, to
// dist/components/, using the same "<name>-<hash>.<ext>" file names the dev
// server serves. A component that failed to build has no Bundle and is
// skipped here; its slide JSON carries the error instead (see the
// transformer), and the caller has already failed the build for that case
// before reaching Build.
func (b *Builder) writeComponentBundles() (int, int64, error) {
	if len(b.components) == 0 {
		return 0, 0, nil
	}

	componentsDir := filepath.Join(b.outputDir, "components")
	if err := os.MkdirAll(componentsDir, 0755); err != nil {
		return 0, 0, fmt.Errorf("failed to create components directory: %w", err)
	}

	count := 0
	var totalSize int64
	for _, result := range b.components {
		bundle := result.Bundle
		if bundle == nil {
			continue
		}
		base := bundle.Name + "-" + bundle.Hash

		jsPath := filepath.Join(componentsDir, base+".js")
		if err := os.WriteFile(jsPath, bundle.JavaScript, 0644); err != nil {
			return count, totalSize, fmt.Errorf("failed to write %s: %w", jsPath, err)
		}
		count++
		totalSize += int64(len(bundle.JavaScript))

		if len(bundle.CSS) > 0 {
			cssPath := filepath.Join(componentsDir, base+".css")
			if err := os.WriteFile(cssPath, bundle.CSS, 0644); err != nil {
				return count, totalSize, fmt.Errorf("failed to write %s: %w", cssPath, err)
			}
			count++
			totalSize += int64(len(bundle.CSS))
		}

		for _, asset := range bundle.Assets {
			assetPath := filepath.Join(componentsDir, asset.Name)
			if err := os.WriteFile(assetPath, asset.Content, 0644); err != nil {
				return count, totalSize, fmt.Errorf("failed to write %s: %w", assetPath, err)
			}
			count++
			totalSize += int64(len(asset.Content))
		}
	}
	return count, totalSize, nil
}

// imgSrcPattern matches img src attributes in HTML.
var imgSrcPattern = regexp.MustCompile(`(<img\s[^>]*src=["'])([^"']+)(["'][^>]*>)`)

// asciinemaBlockPattern matches asciinema code blocks and captures the
// opening tag and the content. The renderer always adds a
// data-code-block-index attribute after the class (and may add others
// later), so this matches on the class alone and tolerates any other
// attributes the tag carries, in any order.
var asciinemaBlockPattern = regexp.MustCompile(`(<code class="language-asciinema"[^>]*>)([\s\S]*?)</code>`)

// ascinemaSrcPattern matches "src: path" lines in asciinema block content.
var ascinemaSrcPattern = regexp.MustCompile(`(?m)^src:\s*(?:&quot;|"|')?([^"'&\n]+)(?:&quot;|"|')?$`)

// extractImagePaths finds all image src attributes in HTML.
func extractImagePaths(html string) []string {
	var paths []string
	matches := imgSrcPattern.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			paths = append(paths, match[2])
		}
	}
	return paths
}

// rewriteImagePaths replaces image paths in HTML using the provided mapping.
func rewriteImagePaths(html string, pathMapping map[string]string) string {
	return imgSrcPattern.ReplaceAllStringFunc(html, func(match string) string {
		submatches := imgSrcPattern.FindStringSubmatch(match)
		if len(submatches) != 4 {
			return match
		}

		prefix := submatches[1]
		src := submatches[2]
		suffix := submatches[3]

		if newPath, exists := pathMapping[src]; exists {
			return prefix + newPath + suffix
		}
		return match
	})
}

// copyWithHash copies a file to the destination directory with a content hash in the filename.
// Returns the relative path to the copied file and its size.
func (b *Builder) copyWithHash(sourcePath, destDir string) (string, int64, error) {
	// Open source file
	src, err := os.Open(sourcePath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Read file content to compute hash
	content, err := io.ReadAll(src)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read source file: %w", err)
	}

	// Compute content hash (first 8 chars of SHA256)
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])[:8]

	// Build destination filename with hash
	ext := filepath.Ext(sourcePath)
	baseName := strings.TrimSuffix(filepath.Base(sourcePath), ext)
	hashedName := fmt.Sprintf("%s.%s%s", baseName, hashStr, ext)
	destPath := filepath.Join(destDir, hashedName)

	// Write to destination
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return "", 0, fmt.Errorf("failed to write destination file: %w", err)
	}

	// Return relative path from output directory
	relPath := filepath.Join("assets", hashedName)
	return relPath, int64(len(content)), nil
}

// extractAsciinemaPaths finds all .cast file src paths in asciinema code blocks.
func extractAsciinemaPaths(html string) []string {
	var paths []string
	blocks := asciinemaBlockPattern.FindAllStringSubmatch(html, -1)
	for _, block := range blocks {
		if len(block) < 3 {
			continue
		}
		content := block[2]
		srcMatches := ascinemaSrcPattern.FindStringSubmatch(content)
		if len(srcMatches) >= 2 {
			paths = append(paths, strings.TrimSpace(srcMatches[1]))
		}
	}
	return paths
}

// rewriteAsciinemaPaths replaces .cast file paths in asciinema code block content.
func rewriteAsciinemaPaths(html string, pathMapping map[string]string) string {
	return asciinemaBlockPattern.ReplaceAllStringFunc(html, func(match string) string {
		submatches := asciinemaBlockPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		openTag := submatches[1]
		content := submatches[2]
		// Replace src paths in the content
		newContent := ascinemaSrcPattern.ReplaceAllStringFunc(content, func(srcLine string) string {
			srcMatches := ascinemaSrcPattern.FindStringSubmatch(srcLine)
			if len(srcMatches) < 2 {
				return srcLine
			}
			oldPath := strings.TrimSpace(srcMatches[1])
			if newPath, exists := pathMapping[oldPath]; exists {
				return "src: " + newPath
			}
			return srcLine
		})
		return openTag + newContent + `</code>`
	})
}

// isAbsoluteURL checks if the path is an absolute URL (http:// or https://).
func isAbsoluteURL(path string) bool {
	lowerPath := strings.ToLower(path)
	return strings.HasPrefix(lowerPath, "http://") || strings.HasPrefix(lowerPath, "https://")
}

// resolveImageSourcePath turns resolvedPath, an image src already
// stripped of its "/local/" dev-server prefix, into the file to open.
// It tries the path exactly as written first: a file genuinely named
// with something that looks like an escape, such as a literal "%20",
// must resolve to itself, not to whatever decoding it would produce.
// Only when nothing exists at the literal path does it fall back to
// decodeAssetPath's undoing of the renderer's own escaping. reportedPath,
// for a caller's warning, is the last path tried: the literal one when
// nothing needed decoding, the decoded one when the literal path did not
// exist.
func (b *Builder) resolveImageSourcePath(resolvedPath string) (sourcePath, reportedPath string, err error) {
	candidates := []string{resolvedPath}
	if decoded := decodeAssetPath(resolvedPath); decoded != resolvedPath {
		candidates = append(candidates, decoded)
	}

	for _, candidate := range candidates {
		reportedPath = candidate
		candidateSource := candidate
		if !filepath.IsAbs(candidate) && b.baseDir != "" {
			candidateSource = filepath.Join(b.baseDir, candidate)
		}
		if _, statErr := os.Stat(candidateSource); statErr == nil {
			return candidateSource, candidate, nil
		}
	}
	return "", reportedPath, fmt.Errorf("no file at %s", reportedPath)
}

// AssetWithinBaseDir resolves symlinks in both baseDir and sourcePath and
// confirms the resolved source sits inside the resolved base directory.
// sourcePath must already exist, so EvalSymlinks on it either succeeds or
// reports a real filesystem error. This is what stops a deck from
// reaching a file outside its own folder: a literal "../", that path's
// percent-encoded form once resolveImageSourcePath has decoded it, an
// absolute path, or a symlink that lives inside the deck's folder but
// targets something outside it. An empty baseDir (only tests pass one
// this way; every real command sets one through SetBaseDir, or a caller
// outside internal/builder passes the deck's own folder) has no folder
// to enforce, so nothing is refused.
func AssetWithinBaseDir(baseDir, sourcePath string) (string, error) {
	if baseDir == "" {
		return sourcePath, nil
	}

	resolvedBase, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		resolvedBase = baseDir
	}
	resolvedBase, err = filepath.Abs(resolvedBase)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}

	resolvedSource, err := filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve asset path: %w", err)
	}
	resolvedSource, err = filepath.Abs(resolvedSource)
	if err != nil {
		return "", fmt.Errorf("failed to resolve asset path: %w", err)
	}

	rel, err := filepath.Rel(resolvedBase, resolvedSource)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s resolves outside %s", sourcePath, resolvedBase)
	}
	return resolvedSource, nil
}

// decodeAssetPath undoes the escaping the renderer applies to an <img>
// src before writing it into a slide's HTML: HTML entity escaping first
// (the outer layer, applied when the attribute value is written), then
// percent-encoding (the inner layer, applied to the link destination
// itself). The result is the real file name on disk, in whatever script
// the author gave it.
func decodeAssetPath(src string) string {
	return percentDecode(html.UnescapeString(src))
}

// percentDecode decodes "%XX" escapes in s. A "%" not followed by two hex
// digits is left as it is instead of failing the whole string: a name a
// person typed by hand, rather than one tap sanitized, can hold a literal
// "%" that was never an escape.
func percentDecode(s string) string {
	var decoded strings.Builder
	decoded.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if hi, ok := hexDigit(s[i+1]); ok {
				if lo, ok := hexDigit(s[i+2]); ok {
					decoded.WriteByte(hi<<4 | lo)
					i += 2
					continue
				}
			}
		}
		decoded.WriteByte(s[i])
	}
	return decoded.String()
}

// hexDigit is the value of a single hex digit character, or false when b
// is not one.
func hexDigit(b byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}

// CopyEmbeddedAssets copies all embedded frontend assets to the output directory.
// This includes JS, CSS, and other assets from the Vite build in the assets/ subdirectory.
// This is useful for builds that need the full frontend application.
func (b *Builder) CopyEmbeddedAssets() (int, int64, error) {
	files, err := embedded.ListAll()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list embedded assets: %w", err)
	}

	var totalSize int64
	count := 0

	for _, file := range files {
		// Skip index.html as we generate our own with embedded JSON
		if file == "index.html" {
			continue
		}

		content, err := embedded.GetFile(file)
		if err != nil {
			return count, totalSize, fmt.Errorf("failed to read embedded file %s: %w", file, err)
		}

		destPath := filepath.Join(b.outputDir, file)

		// Create parent directories if needed (for assets/ subdirectory)
		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return count, totalSize, fmt.Errorf("failed to create directory for %s: %w", file, err)
		}

		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return count, totalSize, fmt.Errorf("failed to write %s: %w", file, err)
		}

		count++
		totalSize += int64(len(content))
	}

	return count, totalSize, nil
}

// generateIndexHTML creates the index.html file by injecting presentation JSON
// into the real Vite-built frontend template, so all themes, fonts, and styles work.
func (b *Builder) generateIndexHTML(path string, pres *transformer.TransformedPresentation) (int64, error) {
	// Serialize presentation to JSON
	presJSON, err := json.Marshal(pres)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal presentation: %w", err)
	}

	// Read the embedded index.html template from the Vite build
	templateHTML, err := embedded.GetIndexHTML()
	if err != nil {
		return 0, fmt.Errorf("failed to read embedded index.html: %w", err)
	}

	// Set the title
	title := pres.Config.Title
	if title == "" {
		title = "Tap Presentation"
	}
	html := strings.Replace(string(templateHTML), "<title>Tap Presentation</title>", "<title>"+title+"</title>", 1)

	// Inject embedded presentation JSON before the closing </body> tag.
	// The React App checks for this element and uses it instead of fetching /api/presentation.
	dataScript := fmt.Sprintf(`<script id="presentation-data" type="application/json">%s</script>`, string(presJSON))
	html = strings.Replace(html, "</body>", dataScript+"\n</body>", 1)

	// Write to file
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return 0, fmt.Errorf("failed to write index.html: %w", err)
	}

	return int64(len(html)), nil
}
