package deckedit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// aiImagePattern matches an ai-prompt comment followed, on the next line,
// by the image it produced. Group 1 is the prompt, group 2 the image path.
var aiImagePattern = regexp.MustCompile(`<!--\s*ai-prompt:\s*(.+?)\s*-->\n[ \t]*!\[\]\(([^)]+)\)`)

// AIImage is an AI-generated image in a deck: the prompt that made it, the
// path the deck links to, relative to the deck's folder, and the choices
// recorded with it: the aspect ratio and whether the theme's brief was
// prepended (both empty for an image made before they were recorded).
type AIImage struct {
	Prompt     string
	ImagePath  string
	Aspect     string
	MatchTheme bool
}

// AIImageAspectRatios are the aspect ratios Gemini's image config accepts,
// and the only aspects a comment records.
var AIImageAspectRatios = []string{"1:1", "16:9", "9:16", "4:3", "3:4"}

// aiPromptOptionSeparator separates the prompt from its recorded choices
// inside the comment: "a fox | aspect: 16:9 | match-theme".
const aiPromptOptionSeparator = " | "

// ParseAIImages returns the AI-generated images in content, in order.
func ParseAIImages(content string) []AIImage {
	matches := aiImagePattern.FindAllStringSubmatch(content, -1)
	if matches == nil {
		return nil
	}
	images := make([]AIImage, 0, len(matches))
	for _, match := range matches {
		prompt, aspect, matchTheme := splitAIPromptOptions(match[1])
		images = append(images, AIImage{Prompt: prompt, ImagePath: match[2], Aspect: aspect, MatchTheme: matchTheme})
	}
	return images
}

// aiPromptComment is the comment's text (without the surrounding
// "<!-- ai-prompt: ... -->"): the prompt, with the choices after the words.
func aiPromptComment(prompt, aspect string, matchTheme bool) string {
	comment := prompt
	if aspect != "" {
		comment += aiPromptOptionSeparator + "aspect: " + aspect
	}
	if matchTheme {
		comment += aiPromptOptionSeparator + "match-theme"
	}
	return comment
}

// splitAIPromptOptions takes the recorded choices off the end of a
// comment's text. Only trailing tokens that are exactly a choice are taken
// ("match-theme", or "aspect: " and one of AIImageAspectRatios), so a
// prompt that itself contains " | " keeps its words.
func splitAIPromptOptions(text string) (prompt, aspect string, matchTheme bool) {
	parts := strings.Split(text, aiPromptOptionSeparator)
	for len(parts) > 1 {
		last := parts[len(parts)-1]
		switch {
		case last == "match-theme":
			matchTheme = true
		case strings.HasPrefix(last, "aspect: ") && slices.Contains(AIImageAspectRatios, strings.TrimPrefix(last, "aspect: ")):
			aspect = strings.TrimPrefix(last, "aspect: ")
		default:
			return strings.Join(parts, aiPromptOptionSeparator), aspect, matchTheme
		}
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, aiPromptOptionSeparator), aspect, matchTheme
}

// PromptReadsAsChoices reports whether a prompt ends in words a comment
// would read back as a recorded choice (" | match-theme", " | aspect:
// 16:9"), which would not survive the round trip through the deck.
func PromptReadsAsChoices(prompt string) bool {
	_, aspect, matchTheme := splitAIPromptOptions(prompt)
	return aspect != "" || matchTheme
}

// AIImageMarkdown is the text that records an AI-generated image in a
// deck: the prompt comment, with the choices after the words, and the
// image link. Without choices the comment is the prompt alone.
func AIImageMarkdown(prompt, imagePath, aspect string, matchTheme bool) string {
	return fmt.Sprintf("<!-- ai-prompt: %s -->\n![](%s)", aiPromptComment(prompt, aspect, matchTheme), imagePath)
}

// InsertAIImage returns content with an AI-generated image added at the
// end of the slide at slideIndex.
func InsertAIImage(content string, slideIndex int, prompt, imagePath, aspect string, matchTheme bool) (string, error) {
	return InsertIntoSlide(content, slideIndex, AIImageMarkdown(prompt, imagePath, aspect, matchTheme))
}

// ReplaceAIImage returns content with the AI-generated image that has
// old's prompt, image path and recorded choices replaced in place by one
// with newPrompt, newImagePath, aspect and matchTheme. It fails when
// content has no such image.
func ReplaceAIImage(content string, old AIImage, newPrompt, newImagePath, aspect string, matchTheme bool) (string, error) {
	pattern, err := regexp.Compile(fmt.Sprintf(`<!--\s*ai-prompt:\s*%s\s*-->\n[ \t]*!\[\]\(%s\)`,
		regexp.QuoteMeta(aiPromptComment(old.Prompt, old.Aspect, old.MatchTheme)), regexp.QuoteMeta(old.ImagePath)))
	if err != nil {
		return "", fmt.Errorf("failed to compile replacement pattern: %w", err)
	}
	if !pattern.MatchString(content) {
		return "", fmt.Errorf("could not find the existing image reference to replace")
	}
	return pattern.ReplaceAllLiteralString(content, AIImageMarkdown(newPrompt, newImagePath, aspect, matchTheme)), nil
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
