package deckedit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
)

// aiImagePattern matches an ai-prompt comment followed, on the next line,
// by the image it produced. Group 1 is the prompt, group 2 the image path.
var aiImagePattern = regexp.MustCompile(`<!--\s*ai-prompt:\s*(.+?)\s*-->\n[ \t]*!\[\]\(([^)]+)\)`)

// AIImage is an AI-generated image in a deck: the prompt that made it and
// the path the deck links to, relative to the deck's folder.
type AIImage struct {
	Prompt    string
	ImagePath string
}

// ParseAIImages returns the AI-generated images in content, in order.
func ParseAIImages(content string) []AIImage {
	matches := aiImagePattern.FindAllStringSubmatch(content, -1)
	if matches == nil {
		return nil
	}
	images := make([]AIImage, 0, len(matches))
	for _, match := range matches {
		images = append(images, AIImage{Prompt: match[1], ImagePath: match[2]})
	}
	return images
}

// AIImageMarkdown is the text that records an AI-generated image in a
// deck: the prompt comment and the image link.
func AIImageMarkdown(prompt, imagePath string) string {
	return fmt.Sprintf("<!-- ai-prompt: %s -->\n![](%s)", prompt, imagePath)
}

// InsertAIImage returns content with an AI-generated image added at the
// end of the slide at slideIndex.
func InsertAIImage(content string, slideIndex int, prompt, imagePath string) (string, error) {
	return InsertIntoSlide(content, slideIndex, AIImageMarkdown(prompt, imagePath))
}

// ReplaceAIImage returns content with the AI-generated image that has
// oldPrompt and oldImagePath replaced in place by one with newPrompt and
// newImagePath. It fails when content has no such image.
func ReplaceAIImage(content, oldPrompt, oldImagePath, newPrompt, newImagePath string) (string, error) {
	pattern, err := regexp.Compile(fmt.Sprintf(`<!--\s*ai-prompt:\s*%s\s*-->\n[ \t]*!\[\]\(%s\)`,
		regexp.QuoteMeta(oldPrompt), regexp.QuoteMeta(oldImagePath)))
	if err != nil {
		return "", fmt.Errorf("failed to compile replacement pattern: %w", err)
	}
	if !pattern.MatchString(content) {
		return "", fmt.Errorf("could not find the existing image reference to replace")
	}
	return pattern.ReplaceAllLiteralString(content, AIImageMarkdown(newPrompt, newImagePath)), nil
}

// GenerateImageFilename names a generated image by its content:
// "generated-<first 8 hex characters of its SHA-256>.<extension>".
func GenerateImageFilename(imageData []byte, contentType string) string {
	hash := sha256.Sum256(imageData)
	return fmt.Sprintf("generated-%s.%s", hex.EncodeToString(hash[:])[:8], GetExtensionFromContentType(contentType))
}

// GetExtensionFromContentType returns the file extension for an image
// MIME type, and "png" for any type it does not know.
func GetExtensionFromContentType(contentType string) string {
	switch contentType {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}
