// Package transformer converts parsed presentations into frontend-ready format.
package transformer

import (
	"github.com/tapsh/tap/internal/config"
	"github.com/tapsh/tap/internal/parser"
)

// TransformedPresentation is the JSON-serializable output for the frontend.
type TransformedPresentation struct {
	Config config.Config      `json:"config"`
	Slides []TransformedSlide `json:"slides"`
}

// TransformedSlide represents a slide ready for frontend rendering.
type TransformedSlide struct {
	Background *BackgroundConfig      `json:"background,omitempty"`
	Layout     string                 `json:"layout"`
	HTML       string                 `json:"html"`
	Transition string                 `json:"transition,omitempty"`
	Notes      string                 `json:"notes,omitempty"`
	CodeBlocks []TransformedCodeBlock `json:"codeBlocks,omitempty"`
	Fragments  []TransformedFragment  `json:"fragments,omitempty"`
	Index      int                    `json:"index"`
}

// TransformedCodeBlock represents a code block ready for frontend rendering.
type TransformedCodeBlock struct {
	Language   string `json:"language"`
	Code       string `json:"code"`
	Driver     string `json:"driver,omitempty"`
	Connection string `json:"connection,omitempty"`
}

// TransformedFragment represents a fragment group for incremental reveals.
type TransformedFragment struct {
	Content string `json:"content"`
	Index   int    `json:"index"`
}

// BackgroundConfig holds background styling for a slide.
type BackgroundConfig struct {
	Value string `json:"value"`
	Type  string `json:"type"` // "color", "image", or "gradient"
}

// Transformer converts parser.Presentation to TransformedPresentation.
type Transformer struct {
	config *config.Config
}

// New creates a new Transformer with the given configuration.
func New(cfg *config.Config) *Transformer {
	return &Transformer{
		config: cfg,
	}
}

// Transform converts a parsed Presentation into a TransformedPresentation
// suitable for JSON serialization and frontend consumption.
func (t *Transformer) Transform(pres *parser.Presentation) *TransformedPresentation {
	result := &TransformedPresentation{
		Config: *t.config,
		Slides: make([]TransformedSlide, 0, len(pres.Slides)),
	}

	for _, slide := range pres.Slides {
		transformed := t.transformSlide(slide)
		result.Slides = append(result.Slides, transformed)
	}

	return result
}

// transformSlide converts a single parser.Slide to TransformedSlide.
func (t *Transformer) transformSlide(slide parser.Slide) TransformedSlide {
	transformed := TransformedSlide{
		Index:  slide.Index,
		HTML:   slide.HTML,
		Layout: t.resolveLayout(slide),
		Notes:  slide.Directives.Notes,
	}

	// Set transition (per-slide directive overrides global config)
	if slide.Directives.Transition != "" {
		transformed.Transition = slide.Directives.Transition
	} else {
		transformed.Transition = t.config.Transition
	}

	// Transform fragments
	if len(slide.Fragments) > 0 {
		transformed.Fragments = make([]TransformedFragment, len(slide.Fragments))
		for i, frag := range slide.Fragments {
			transformed.Fragments[i] = TransformedFragment{
				Content: frag.Content,
				Index:   frag.Index,
			}
		}
	}

	// Transform code blocks
	if len(slide.CodeBlocks) > 0 {
		transformed.CodeBlocks = make([]TransformedCodeBlock, len(slide.CodeBlocks))
		for i, block := range slide.CodeBlocks {
			transformed.CodeBlocks[i] = TransformedCodeBlock{
				Language:   block.Language,
				Code:       block.Code,
				Driver:     block.Meta.Driver,
				Connection: block.Meta.Connection,
			}
		}
	}

	// Transform background
	if slide.Directives.Background != "" {
		transformed.Background = t.parseBackground(slide.Directives.Background)
	}

	return transformed
}

// resolveLayout determines the layout for a slide.
// If a layout directive is specified, it takes precedence.
// Otherwise, returns "default" (layout detection is handled in US-021).
func (t *Transformer) resolveLayout(slide parser.Slide) string {
	if slide.Directives.Layout != "" {
		return slide.Directives.Layout
	}
	return "default"
}

// parseBackground parses a background directive value and determines its type.
func (t *Transformer) parseBackground(value string) *BackgroundConfig {
	// Detect background type based on value format
	bgType := "color"

	// Check for image (URL or file path)
	if isImageURL(value) {
		bgType = "image"
	} else if isGradient(value) {
		bgType = "gradient"
	}

	return &BackgroundConfig{
		Value: value,
		Type:  bgType,
	}
}

// isImageURL checks if the value looks like an image URL or file path.
func isImageURL(value string) bool {
	// Check for common image extensions
	imageExtensions := []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp"}
	for _, ext := range imageExtensions {
		if len(value) > len(ext) && value[len(value)-len(ext):] == ext {
			return true
		}
	}

	// Check for URL protocols
	if len(value) > 8 && (value[:7] == "http://" || value[:8] == "https://") {
		return true
	}

	return false
}

// isGradient checks if the value looks like a CSS gradient.
func isGradient(value string) bool {
	gradientPrefixes := []string{"linear-gradient(", "radial-gradient(", "conic-gradient("}
	for _, prefix := range gradientPrefixes {
		if len(value) >= len(prefix) && value[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
