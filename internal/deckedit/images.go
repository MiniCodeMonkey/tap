package deckedit

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrNotAnImage means a file does not have an image extension tap accepts.
var ErrNotAnImage = errors.New("not an image")

// imageExtensions are the file types tap image add accepts, in lower case.
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".svg": true, ".avif": true,
}

// linkUnsafePattern matches runs of characters that need escaping in a
// plain markdown link destination, or that break the link once it is
// rendered and read back off disk: whitespace; the parentheses that would
// otherwise close the link early; angle brackets, which would otherwise
// look like the <path> form; single and double quotes and the backtick,
// which goldmark percent-encodes in the rendered src, so the builder's
// asset copy looks for a name that no longer matches the file on disk;
// and "#", which a browser reads as the start of a URL fragment, so an
// image with one in its name never loads in tap dev or tap present even
// though the built output, which never opens the file through a browser
// URL, still finds it.
var linkUnsafePattern = regexp.MustCompile("[\\s()<>'\"`#]+")

// AddedImage is an image copied into a deck's images folder. Path is
// relative to the deck's folder.
type AddedImage struct {
	Path     string
	Markdown string
}

// ImagesDir is the images folder next to a deck.
func ImagesDir(deckPath string) string {
	return filepath.Join(filepath.Dir(deckPath), "images")
}

// EnsureImagesDir creates the images folder next to a deck when it does
// not exist, and returns its path.
func EnsureImagesDir(deckPath string) (string, error) {
	imagesDir := ImagesDir(deckPath)
	info, err := os.Stat(imagesDir)
	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("images path exists but is not a directory: %s", imagesDir)
		}
		return imagesDir, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check images directory: %w", err)
	}
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create images directory: %w", err)
	}
	return imagesDir, nil
}

// AddImage copies the image at sourcePath into the deck's images folder.
// Spaces, parentheses and other characters that would need escaping in a
// markdown link are replaced with "-" in the copied name, and -2, -3 and
// so on are added before the extension when the sanitized name is taken.
// A source already in the images folder under a name that needs no
// sanitizing is used where it is, without a copy.
func AddImage(deckPath, sourcePath string) (AddedImage, error) {
	if !imageExtensions[strings.ToLower(filepath.Ext(sourcePath))] {
		return AddedImage{}, fmt.Errorf("%w: %s (tap accepts png, jpg, jpeg, gif, webp, svg and avif)", ErrNotAnImage, sourcePath)
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return AddedImage{}, err
	}
	if sourceInfo.IsDir() {
		return AddedImage{}, fmt.Errorf("%w: %s is a folder", ErrNotAnImage, sourcePath)
	}

	imagesDir, err := EnsureImagesDir(deckPath)
	if err != nil {
		return AddedImage{}, err
	}
	baseName := filepath.Base(sourcePath)
	sanitizedName := sanitizeForLink(baseName)
	// A source already sitting in the images folder is used where it is,
	// without a copy, but only when its on-disk name already needs no
	// sanitizing: a pre-existing image named with a space or another
	// unsafe character still gets a fresh, safely named copy, so the link
	// tap writes is one goldmark and the builder actually resolve.
	if sanitizedName == baseName {
		if existing, err := os.Stat(filepath.Join(imagesDir, baseName)); err == nil && os.SameFile(existing, sourceInfo) {
			return newAddedImage(baseName, baseName), nil
		}
	}
	name, err := copyToFreeName(sourcePath, imagesDir, sanitizedName)
	if err != nil {
		return AddedImage{}, err
	}
	// The alt text comes from the sanitized name before any -2, -3 clash
	// numbering, so two images with the same name do not end up captioned
	// "diagram" and "diagram-2".
	return newAddedImage(name, sanitizedName), nil
}

func newAddedImage(name, altSourceName string) AddedImage {
	path := filepath.Join("images", name)
	return AddedImage{Path: path, Markdown: imageMarkdownWithAlt(path, altSourceName)}
}

// sanitizeForLink replaces runs of whitespace, parentheses and angle
// brackets in name's stem with "-", so the copied file's name works as a
// plain, unescaped markdown link destination.
func sanitizeForLink(name string) string {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	stem = linkUnsafePattern.ReplaceAllString(stem, "-")
	stem = strings.Trim(stem, "-")
	if stem == "" {
		stem = "image"
	}
	return stem + extension
}

// copyToFreeName copies sourcePath into directory under name, or under
// name-2, name-3 and so on when that file exists. It returns the name used.
func copyToFreeName(sourcePath, directory, name string) (string, error) {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for number := 1; ; number++ {
		candidate := name
		if number > 1 {
			candidate = fmt.Sprintf("%s-%d%s", stem, number, extension)
		}
		destinationPath := filepath.Join(directory, candidate)
		destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("cannot create %s: %w", destinationPath, err)
		}
		copyErr := copyInto(destination, sourcePath)
		if closeErr := destination.Close(); copyErr == nil {
			copyErr = closeErr
		}
		if copyErr != nil {
			_ = os.Remove(destinationPath)
			return "", copyErr
		}
		return candidate, nil
	}
}

func copyInto(destination io.Writer, sourcePath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	_, err = io.Copy(destination, source)
	return err
}

// ImageMarkdown is the markdown that shows the image at imagePath, a path
// relative to the deck's folder, as a plain relative link. The alt text
// is the file name without its extension, with "[" and "]" removed.
func ImageMarkdown(imagePath string) string {
	return imageMarkdownWithAlt(imagePath, filepath.Base(imagePath))
}

// imageMarkdownWithAlt is ImageMarkdown, but the alt text comes from
// altSourceName instead of imagePath's own file name.
func imageMarkdownWithAlt(imagePath, altSourceName string) string {
	destination := filepath.ToSlash(imagePath)
	alt := strings.TrimSuffix(altSourceName, filepath.Ext(altSourceName))
	alt = strings.NewReplacer("[", "", "]", "").Replace(alt)
	return fmt.Sprintf("![%s](%s)", alt, destination)
}
