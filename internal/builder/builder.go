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

	"github.com/tapsh/tap/internal/config"
	"github.com/tapsh/tap/internal/parser"
	"github.com/tapsh/tap/internal/transformer"
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
// It creates index.html with embedded presentation JSON,
// copies referenced images to assets/ with content hashes,
// and rewrites image paths in the HTML.
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

			// Resolve the image path
			sourcePath := imgPath
			if !filepath.IsAbs(imgPath) && b.baseDir != "" {
				sourcePath = filepath.Join(b.baseDir, imgPath)
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

	// Rewrite image paths in transformed slides
	for i := range transformed.Slides {
		slide := &transformed.Slides[i]
		slide.HTML = rewriteImagePaths(slide.HTML, pathMapping)
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

// isAbsoluteURL checks if the path is an absolute URL (http:// or https://).
func isAbsoluteURL(path string) bool {
	lowerPath := strings.ToLower(path)
	return strings.HasPrefix(lowerPath, "http://") || strings.HasPrefix(lowerPath, "https://")
}

// generateIndexHTML creates the index.html file with embedded presentation JSON.
func (b *Builder) generateIndexHTML(path string, pres *transformer.TransformedPresentation) (int64, error) {
	// Serialize presentation to JSON
	presJSON, err := json.Marshal(pres)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal presentation: %w", err)
	}

	// Generate HTML with embedded JSON
	html := generateStaticHTML(pres.Config.Title, string(presJSON))

	// Write to file
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return 0, fmt.Errorf("failed to write index.html: %w", err)
	}

	return int64(len(html)), nil
}

// generateStaticHTML creates the HTML shell with embedded presentation JSON.
func generateStaticHTML(title string, presJSON string) string {
	if title == "" {
		title = "Tap Presentation"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        html, body {
            width: 100%%;
            height: 100%%;
            overflow: hidden;
            background: #000;
        }
        .slide-container {
            width: 100%%;
            height: 100%%;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .slide {
            width: 100%%;
            max-width: 1920px;
            aspect-ratio: 16 / 9;
            background: #fff;
            padding: 4rem;
            overflow: hidden;
        }
        .slide h1 { font-size: 4rem; margin-bottom: 1rem; }
        .slide h2 { font-size: 3rem; margin-bottom: 0.75rem; }
        .slide h3 { font-size: 2rem; margin-bottom: 0.5rem; }
        .slide p { font-size: 1.5rem; line-height: 1.6; margin-bottom: 1rem; }
        .slide ul, .slide ol { font-size: 1.5rem; margin-left: 2rem; margin-bottom: 1rem; }
        .slide li { margin-bottom: 0.5rem; }
        .slide pre { background: #1e1e1e; color: #d4d4d4; padding: 1rem; border-radius: 0.5rem; overflow-x: auto; margin-bottom: 1rem; }
        .slide code { font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace; font-size: 1.25rem; }
        .slide img { max-width: 100%%; max-height: 80%%; object-fit: contain; }
        .slide blockquote { border-left: 4px solid #7C3AED; padding-left: 1.5rem; font-style: italic; font-size: 1.75rem; }
        .navigation {
            position: fixed;
            bottom: 1rem;
            right: 1rem;
            display: flex;
            gap: 0.5rem;
            z-index: 100;
        }
        .navigation button {
            padding: 0.5rem 1rem;
            font-size: 1rem;
            cursor: pointer;
            border: none;
            background: rgba(0,0,0,0.5);
            color: #fff;
            border-radius: 0.25rem;
        }
        .navigation button:hover {
            background: rgba(0,0,0,0.7);
        }
        .slide-counter {
            position: fixed;
            bottom: 1rem;
            left: 1rem;
            color: rgba(255,255,255,0.7);
            font-size: 0.875rem;
            z-index: 100;
        }
        .static-mode-notice {
            display: none;
            position: fixed;
            bottom: 4rem;
            left: 50%%;
            transform: translateX(-50%%);
            background: rgba(0,0,0,0.8);
            color: #fff;
            padding: 0.5rem 1rem;
            border-radius: 0.25rem;
            font-size: 0.875rem;
            z-index: 100;
        }
        .code-block-static .static-mode-notice {
            display: block;
        }
    </style>
</head>
<body>
    <div class="slide-container">
        <div class="slide" id="slide-content"></div>
    </div>
    <div class="navigation">
        <button onclick="prevSlide()">← Prev</button>
        <button onclick="nextSlide()">Next →</button>
    </div>
    <div class="slide-counter" id="slide-counter"></div>

    <script id="presentation-data" type="application/json">%s</script>
    <script>
        const data = JSON.parse(document.getElementById('presentation-data').textContent);
        let currentSlide = parseInt(location.hash.slice(1)) || 0;

        function render() {
            const slide = data.slides[currentSlide];
            if (!slide) return;
            document.getElementById('slide-content').innerHTML = slide.html;
            document.getElementById('slide-counter').textContent =
                (currentSlide + 1) + ' / ' + data.slides.length;
            location.hash = currentSlide;
        }

        function nextSlide() {
            if (currentSlide < data.slides.length - 1) {
                currentSlide++;
                render();
            }
        }

        function prevSlide() {
            if (currentSlide > 0) {
                currentSlide--;
                render();
            }
        }

        document.addEventListener('keydown', (e) => {
            if (e.key === 'ArrowRight' || e.key === ' ') nextSlide();
            if (e.key === 'ArrowLeft') prevSlide();
            if (e.key === 'Home') { currentSlide = 0; render(); }
            if (e.key === 'End') { currentSlide = data.slides.length - 1; render(); }
        });

        window.addEventListener('hashchange', () => {
            const hash = parseInt(location.hash.slice(1));
            if (!isNaN(hash) && hash >= 0 && hash < data.slides.length) {
                currentSlide = hash;
                render();
            }
        });

        render();
    </script>
</body>
</html>
`, title, presJSON)
}
