// Package parser provides markdown parsing functionality for tap presentations.
package parser

import (
	"regexp"
	"strings"
)

// ImageAttributes contains parsed attributes for markdown images.
// Supports width and position attributes via {width=50%} and {position=left} syntax.
type ImageAttributes struct {
	// Width specifies the image width (e.g., "50%", "200px").
	Width string
	// Position specifies the image alignment: "left", "right", or "center".
	Position string
}

// imageAttrPattern matches image attribute blocks like {width=50%} or {position=left}
// Captures the content inside the braces.
// Example: ![Alt](img.png){width=50%} or ![Alt](img.png){width=50%, position=left}
var imageAttrPattern = regexp.MustCompile(`\{([^}]+)\}\s*$`)

// imageMarkdownPattern matches markdown image syntax with optional attributes.
// Captures: (1) alt text, (2) URL, (3) optional attributes with braces
// Example: ![Alt text](image.png){width=50%}
var imageMarkdownPattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)(\{[^}]+\})?`)

// ParseImageAttributes parses attribute string like "width=50%" or "width=50%, position=left"
// and returns an ImageAttributes struct with the parsed values.
func ParseImageAttributes(attrString string) ImageAttributes {
	attrs := ImageAttributes{}

	if attrString == "" {
		return attrs
	}

	// Remove surrounding braces if present
	attrString = strings.TrimPrefix(attrString, "{")
	attrString = strings.TrimSuffix(attrString, "}")

	// Split on comma for multiple attributes
	parts := strings.Split(attrString, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Parse key=value pairs
		if idx := strings.Index(part, "="); idx != -1 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])

			switch strings.ToLower(key) {
			case "width":
				attrs.Width = value
			case "position":
				attrs.Position = strings.ToLower(value)
			}
		}
	}

	return attrs
}

// ImageInfo contains information about a parsed image in markdown content.
type ImageInfo struct {
	// AltText is the image alt text.
	AltText string
	// URL is the image source URL or path.
	URL string
	// Attributes contains parsed width and position attributes.
	Attributes ImageAttributes
	// Raw is the original raw markdown string for this image.
	Raw string
}

// ParseImages extracts all images from markdown content along with their attributes.
// It parses the ![Alt](url){attributes} syntax.
func ParseImages(content string) []ImageInfo {
	images := []ImageInfo{}

	matches := imageMarkdownPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		info := ImageInfo{
			AltText: match[1],
			URL:     match[2],
			Raw:     match[0],
		}

		// Parse attributes if present (match[3] includes the braces)
		if len(match) > 3 && match[3] != "" {
			info.Attributes = ParseImageAttributes(match[3])
		}

		images = append(images, info)
	}

	return images
}

// ExtractImageAttributes extracts attributes from an image markdown line.
// Given a line like "![Alt](img.png){width=50%}", it returns the ImageAttributes.
// If no attributes are found, returns an empty ImageAttributes.
func ExtractImageAttributes(line string) ImageAttributes {
	match := imageAttrPattern.FindStringSubmatch(line)
	if match == nil || len(match) < 2 {
		return ImageAttributes{}
	}

	return ParseImageAttributes(match[1])
}
