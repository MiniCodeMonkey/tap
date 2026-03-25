// Package builder generates static files for tap presentations.
package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MiniCodeMonkey/tap/embedded"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// BuildResult contains statistics about the completed build.
type BuildResult struct {
	OutputDir string        // Output directory path
	BuildTime time.Duration // Total build duration
	FileCount int           // Number of files generated
	TotalSize int64         // Total size of all files in bytes
}

// Builder generates static files from a tap presentation.
type Builder struct {
	outputDir string
	baseDir   string // Base directory for resolving relative paths
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

	// Transform presentation to frontend-ready format
	trans := transformer.NewWithBaseDir(cfg, b.baseDir)
	transformed := trans.Transform(pres)

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
			resolvedPath := imgPath
			if strings.HasPrefix(resolvedPath, "/local/") {
				resolvedPath = strings.TrimPrefix(resolvedPath, "/local/")
			}

			sourcePath := resolvedPath
			if !filepath.IsAbs(resolvedPath) && b.baseDir != "" {
				sourcePath = filepath.Join(b.baseDir, resolvedPath)
			}

			// Copy the image with content hash
			hashedPath, size, err := b.copyWithHash(sourcePath, assetsDir)
			if err != nil {
				// Skip images that can't be found (might be external URLs or invalid)
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

			resolvedPath := castPath
			if strings.HasPrefix(resolvedPath, "/local/") {
				resolvedPath = strings.TrimPrefix(resolvedPath, "/local/")
			}

			sourcePath := resolvedPath
			if !filepath.IsAbs(resolvedPath) && b.baseDir != "" {
				sourcePath = filepath.Join(b.baseDir, resolvedPath)
			}

			hashedPath, size, err := b.copyWithHash(sourcePath, assetsDir)
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

// imgSrcPattern matches img src attributes in HTML.
var imgSrcPattern = regexp.MustCompile(`(<img\s[^>]*src=["'])([^"']+)(["'][^>]*>)`)

// asciinemaBlockPattern matches asciinema code blocks and captures the content.
var asciinemaBlockPattern = regexp.MustCompile(`<code class="language-asciinema">([\s\S]*?)</code>`)

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
		if len(block) < 2 {
			continue
		}
		content := block[1]
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
		if len(submatches) < 2 {
			return match
		}
		content := submatches[1]
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
		return `<code class="language-asciinema">` + newContent + `</code>`
	})
}

// isAbsoluteURL checks if the path is an absolute URL (http:// or https://).
func isAbsoluteURL(path string) bool {
	lowerPath := strings.ToLower(path)
	return strings.HasPrefix(lowerPath, "http://") || strings.HasPrefix(lowerPath, "https://")
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
	// The Svelte App.svelte checks for this element and uses it instead of fetching /api/presentation.
	dataScript := fmt.Sprintf(`<script id="presentation-data" type="application/json">%s</script>`, string(presJSON))
	html = strings.Replace(html, "</body>", dataScript+"\n</body>", 1)

	// Write to file
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return 0, fmt.Errorf("failed to write index.html: %w", err)
	}

	return int64(len(html)), nil
}
