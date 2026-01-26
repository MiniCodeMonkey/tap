package parser

import (
	"testing"
)

func TestParseImageAttributes_Width(t *testing.T) {
	attrs := ParseImageAttributes("width=50%")

	if attrs.Width != "50%" {
		t.Errorf("expected width '50%%', got %q", attrs.Width)
	}
	if attrs.Position != "" {
		t.Errorf("expected empty position, got %q", attrs.Position)
	}
}

func TestParseImageAttributes_Position(t *testing.T) {
	attrs := ParseImageAttributes("position=left")

	if attrs.Position != "left" {
		t.Errorf("expected position 'left', got %q", attrs.Position)
	}
	if attrs.Width != "" {
		t.Errorf("expected empty width, got %q", attrs.Width)
	}
}

func TestParseImageAttributes_PositionRight(t *testing.T) {
	attrs := ParseImageAttributes("position=right")

	if attrs.Position != "right" {
		t.Errorf("expected position 'right', got %q", attrs.Position)
	}
}

func TestParseImageAttributes_PositionCenter(t *testing.T) {
	attrs := ParseImageAttributes("position=center")

	if attrs.Position != "center" {
		t.Errorf("expected position 'center', got %q", attrs.Position)
	}
}

func TestParseImageAttributes_Multiple(t *testing.T) {
	attrs := ParseImageAttributes("width=75%, position=right")

	if attrs.Width != "75%" {
		t.Errorf("expected width '75%%', got %q", attrs.Width)
	}
	if attrs.Position != "right" {
		t.Errorf("expected position 'right', got %q", attrs.Position)
	}
}

func TestParseImageAttributes_WithBraces(t *testing.T) {
	attrs := ParseImageAttributes("{width=50%}")

	if attrs.Width != "50%" {
		t.Errorf("expected width '50%%', got %q", attrs.Width)
	}
}

func TestParseImageAttributes_Empty(t *testing.T) {
	attrs := ParseImageAttributes("")

	if attrs.Width != "" {
		t.Errorf("expected empty width, got %q", attrs.Width)
	}
	if attrs.Position != "" {
		t.Errorf("expected empty position, got %q", attrs.Position)
	}
}

func TestParseImageAttributes_ExtraSpaces(t *testing.T) {
	attrs := ParseImageAttributes("  width = 50% ,  position = left  ")

	if attrs.Width != "50%" {
		t.Errorf("expected width '50%%', got %q", attrs.Width)
	}
	if attrs.Position != "left" {
		t.Errorf("expected position 'left', got %q", attrs.Position)
	}
}

func TestParseImageAttributes_PixelWidth(t *testing.T) {
	attrs := ParseImageAttributes("width=200px")

	if attrs.Width != "200px" {
		t.Errorf("expected width '200px', got %q", attrs.Width)
	}
}

func TestParseImageAttributes_CaseInsensitive(t *testing.T) {
	attrs := ParseImageAttributes("Width=50%, Position=LEFT")

	if attrs.Width != "50%" {
		t.Errorf("expected width '50%%', got %q", attrs.Width)
	}
	// Position is lowercased
	if attrs.Position != "left" {
		t.Errorf("expected position 'left', got %q", attrs.Position)
	}
}

func TestParseImageAttributes_UnknownAttribute(t *testing.T) {
	attrs := ParseImageAttributes("width=50%, unknown=value, position=right")

	if attrs.Width != "50%" {
		t.Errorf("expected width '50%%', got %q", attrs.Width)
	}
	if attrs.Position != "right" {
		t.Errorf("expected position 'right', got %q", attrs.Position)
	}
	// Unknown attributes are silently ignored
}

func TestParseImages_Simple(t *testing.T) {
	content := "![Alt text](image.png)"
	images := ParseImages(content)

	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}

	if images[0].AltText != "Alt text" {
		t.Errorf("expected alt text 'Alt text', got %q", images[0].AltText)
	}
	if images[0].URL != "image.png" {
		t.Errorf("expected URL 'image.png', got %q", images[0].URL)
	}
	if images[0].Attributes.Width != "" {
		t.Errorf("expected empty width, got %q", images[0].Attributes.Width)
	}
}

func TestParseImages_WithWidth(t *testing.T) {
	content := "![Screenshot](screenshot.png){width=50%}"
	images := ParseImages(content)

	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}

	if images[0].AltText != "Screenshot" {
		t.Errorf("expected alt text 'Screenshot', got %q", images[0].AltText)
	}
	if images[0].URL != "screenshot.png" {
		t.Errorf("expected URL 'screenshot.png', got %q", images[0].URL)
	}
	if images[0].Attributes.Width != "50%" {
		t.Errorf("expected width '50%%', got %q", images[0].Attributes.Width)
	}
}

func TestParseImages_WithPosition(t *testing.T) {
	content := "![Logo](logo.svg){position=left}"
	images := ParseImages(content)

	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}

	if images[0].URL != "logo.svg" {
		t.Errorf("expected URL 'logo.svg', got %q", images[0].URL)
	}
	if images[0].Attributes.Position != "left" {
		t.Errorf("expected position 'left', got %q", images[0].Attributes.Position)
	}
}

func TestParseImages_WithWidthAndPosition(t *testing.T) {
	content := "![Diagram](diagram.png){width=80%, position=center}"
	images := ParseImages(content)

	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}

	if images[0].Attributes.Width != "80%" {
		t.Errorf("expected width '80%%', got %q", images[0].Attributes.Width)
	}
	if images[0].Attributes.Position != "center" {
		t.Errorf("expected position 'center', got %q", images[0].Attributes.Position)
	}
}

func TestParseImages_Multiple(t *testing.T) {
	content := `# Slide with images

![First](first.png){width=50%}

Some text here.

![Second](second.jpg){position=right}

More text.

![Third](third.gif)`

	images := ParseImages(content)

	if len(images) != 3 {
		t.Fatalf("expected 3 images, got %d", len(images))
	}

	// First image
	if images[0].AltText != "First" {
		t.Errorf("first image: expected alt 'First', got %q", images[0].AltText)
	}
	if images[0].Attributes.Width != "50%" {
		t.Errorf("first image: expected width '50%%', got %q", images[0].Attributes.Width)
	}

	// Second image
	if images[1].AltText != "Second" {
		t.Errorf("second image: expected alt 'Second', got %q", images[1].AltText)
	}
	if images[1].Attributes.Position != "right" {
		t.Errorf("second image: expected position 'right', got %q", images[1].Attributes.Position)
	}

	// Third image
	if images[2].AltText != "Third" {
		t.Errorf("third image: expected alt 'Third', got %q", images[2].AltText)
	}
	if images[2].Attributes.Width != "" {
		t.Errorf("third image: expected empty width, got %q", images[2].Attributes.Width)
	}
}

func TestParseImages_NoImages(t *testing.T) {
	content := "# Title\n\nSome text without images."
	images := ParseImages(content)

	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

func TestParseImages_EmptyAlt(t *testing.T) {
	content := "![](photo.jpg){width=100%}"
	images := ParseImages(content)

	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}

	if images[0].AltText != "" {
		t.Errorf("expected empty alt text, got %q", images[0].AltText)
	}
	if images[0].URL != "photo.jpg" {
		t.Errorf("expected URL 'photo.jpg', got %q", images[0].URL)
	}
}

func TestParseImages_AbsoluteURL(t *testing.T) {
	content := "![External](https://example.com/image.png){width=50%}"
	images := ParseImages(content)

	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}

	if images[0].URL != "https://example.com/image.png" {
		t.Errorf("expected URL 'https://example.com/image.png', got %q", images[0].URL)
	}
	if images[0].Attributes.Width != "50%" {
		t.Errorf("expected width '50%%', got %q", images[0].Attributes.Width)
	}
}

func TestParseImages_RelativePath(t *testing.T) {
	content := "![Relative](./images/photo.png){width=60%}"
	images := ParseImages(content)

	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}

	if images[0].URL != "./images/photo.png" {
		t.Errorf("expected URL './images/photo.png', got %q", images[0].URL)
	}
}

func TestParseImages_Raw(t *testing.T) {
	content := "![Test](test.png){width=50%}"
	images := ParseImages(content)

	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}

	if images[0].Raw != "![Test](test.png){width=50%}" {
		t.Errorf("expected raw '![Test](test.png){width=50%%}', got %q", images[0].Raw)
	}
}

func TestExtractImageAttributes_Line(t *testing.T) {
	line := "![Alt](img.png){width=50%}"
	attrs := ExtractImageAttributes(line)

	if attrs.Width != "50%" {
		t.Errorf("expected width '50%%', got %q", attrs.Width)
	}
}

func TestExtractImageAttributes_NoAttributes(t *testing.T) {
	line := "![Alt](img.png)"
	attrs := ExtractImageAttributes(line)

	if attrs.Width != "" {
		t.Errorf("expected empty width, got %q", attrs.Width)
	}
	if attrs.Position != "" {
		t.Errorf("expected empty position, got %q", attrs.Position)
	}
}

func TestExtractImageAttributes_MultipleOnLine(t *testing.T) {
	line := "Text before ![Alt](img.png){width=50%, position=left}"
	attrs := ExtractImageAttributes(line)

	// Should extract attributes from the end of the line
	if attrs.Width != "50%" {
		t.Errorf("expected width '50%%', got %q", attrs.Width)
	}
	if attrs.Position != "left" {
		t.Errorf("expected position 'left', got %q", attrs.Position)
	}
}

func TestParseImages_SupportedFormats(t *testing.T) {
	formats := []string{"png", "jpg", "jpeg", "gif", "svg", "webp"}
	for _, format := range formats {
		content := "![Test](image." + format + "){width=50%}"
		images := ParseImages(content)

		if len(images) != 1 {
			t.Errorf("format %s: expected 1 image, got %d", format, len(images))
			continue
		}

		expectedURL := "image." + format
		if images[0].URL != expectedURL {
			t.Errorf("format %s: expected URL %q, got %q", format, expectedURL, images[0].URL)
		}
	}
}

func TestTransformImageAttributes_Width(t *testing.T) {
	content := "![](./images/test.png){width=50%}"
	result := transformImageAttributes(content)

	expected := `<img src="./images/test.png" alt="" style="width: 50%">`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestTransformImageAttributes_Position(t *testing.T) {
	content := "![Photo](photo.jpg){position=center}"
	result := transformImageAttributes(content)

	expected := `<img src="photo.jpg" alt="Photo" style="display: block; margin-left: auto; margin-right: auto">`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestTransformImageAttributes_WidthAndPosition(t *testing.T) {
	content := "![](img.png){width=50%, position=left}"
	result := transformImageAttributes(content)

	expected := `<img src="img.png" alt="" style="width: 50%; float: left; margin-right: 1em">`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestTransformImageAttributes_NoAttributes(t *testing.T) {
	content := "![Alt](image.png)"
	result := transformImageAttributes(content)

	// Should remain unchanged when no attributes
	if result != content {
		t.Errorf("expected unchanged %q, got %q", content, result)
	}
}

func TestTransformImageAttributes_InParagraph(t *testing.T) {
	content := "Some text before\n\n![](./images/test.png){width=50%}\n\nSome text after"
	result := transformImageAttributes(content)

	if !stringContains(result, `<img src="./images/test.png" alt="" style="width: 50%">`) {
		t.Errorf("expected transformed image in result, got %q", result)
	}
	if !stringContains(result, "Some text before") {
		t.Errorf("expected text before preserved, got %q", result)
	}
	if !stringContains(result, "Some text after") {
		t.Errorf("expected text after preserved, got %q", result)
	}
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
