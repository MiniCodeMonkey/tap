package deckedit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/gemini"
)

// ImageGenerator makes an image from a text prompt. *gemini.Client is one.
type ImageGenerator interface {
	GenerateImage(ctx context.Context, prompt string) (*gemini.ImageResult, error)
}

// NewImageGenerator returns the generator that tap image generate, tap
// image regenerate and the TUI i key use: the Gemini client, configured
// from GEMINI_API_KEY. Tests replace it with a fake, so no test calls the
// Gemini API.
var NewImageGenerator = func() (ImageGenerator, error) {
	client, err := gemini.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Placement says where a generated image goes: at the end of the slide at
// SlideIndex, or, when Replacing is set, in place of that AI image.
type Placement struct {
	DeckPath   string
	SlideIndex int
	Prompt     string
	Replacing  *AIImage
}

// PlacedImage is the result of PlaceGeneratedImage. Path is relative to
// the deck's folder. DeletedPath is the replaced image's file, removed
// from disk. DeleteError is set when that file could not be removed; the
// deck is already updated then.
type PlacedImage struct {
	Path        string
	Markdown    string
	DeletedPath string
	DeleteError error
}

// SaveImage writes a generated image into the deck's images folder, named
// by its content, and returns its path relative to the deck's folder. The
// file is new, so there is nothing on disk it could destroy; it is
// written directly, with the same 0644 permission every other image tap
// writes uses.
func SaveImage(deckPath string, image gemini.ImageResult) (string, error) {
	imagesDir, err := EnsureImagesDir(deckPath)
	if err != nil {
		return "", fmt.Errorf("failed to ensure images directory: %w", err)
	}
	filename := GenerateImageFilename(image.Data, image.ContentType)
	if err := os.WriteFile(filepath.Join(imagesDir, filename), image.Data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write image file: %w", err)
	}
	return filepath.Join("images", filename), nil
}

// PlaceGeneratedImage saves image into the deck's images folder and
// records it in the deck with its prompt: added at the end of the slide,
// or in place of the image it replaces. A replaced image's file is
// deleted, unless the new image has the same file name. The deck is
// written atomically and through a symlink, the same as every other
// change to a deck's files.
func PlaceGeneratedImage(placement Placement, image gemini.ImageResult) (PlacedImage, error) {
	path, err := SaveImage(placement.DeckPath, image)
	if err != nil {
		return PlacedImage{}, err
	}
	placed := PlacedImage{Path: path, Markdown: AIImageMarkdown(placement.Prompt, path)}

	info, err := os.Stat(placement.DeckPath)
	if err != nil {
		return placed, fmt.Errorf("failed to stat markdown file: %w", err)
	}
	content, err := os.ReadFile(placement.DeckPath)
	if err != nil {
		return placed, fmt.Errorf("failed to read markdown file: %w", err)
	}
	var updated string
	if placement.Replacing == nil {
		updated, err = InsertAIImage(string(content), placement.SlideIndex, placement.Prompt, path)
	} else {
		updated, err = ReplaceAIImage(string(content), placement.Replacing.Prompt, placement.Replacing.ImagePath, placement.Prompt, path)
	}
	if err != nil {
		return placed, fmt.Errorf("failed to update markdown: %w", err)
	}
	if err := config.WriteFileAtomically(placement.DeckPath, []byte(updated), info.Mode().Perm()); err != nil {
		return placed, fmt.Errorf("failed to write markdown file: %w", err)
	}

	if placement.Replacing != nil && filepath.Clean(placement.Replacing.ImagePath) != filepath.Clean(path) {
		if err := deleteImage(placement.DeckPath, placement.Replacing.ImagePath); err != nil {
			placed.DeleteError = err
		} else {
			placed.DeletedPath = placement.Replacing.ImagePath
		}
	}
	return placed, nil
}

// deleteImage removes an image given by its path relative to the deck's
// folder. A file that is already gone is not an error.
func deleteImage(deckPath, imagePath string) error {
	err := os.Remove(filepath.Join(filepath.Dir(deckPath), imagePath))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete old image: %w", err)
	}
	return nil
}
