package deckedit

import (
	"strings"
	"testing"
)

func TestParseAIImages(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedCount int
		expectedItems []AIImage
	}{
		{
			name:          "no AI images",
			content:       "# Slide Title\n\nSome content here",
			expectedCount: 0,
			expectedItems: nil,
		},
		{
			name:          "single AI image",
			content:       "# Slide Title\n\n<!-- ai-prompt: a beautiful sunset -->\n![](images/generated-abc123.png)",
			expectedCount: 1,
			expectedItems: []AIImage{
				{Prompt: "a beautiful sunset", ImagePath: "images/generated-abc123.png"},
			},
		},
		{
			name:          "multiple AI images",
			content:       "# Slide Title\n\n<!-- ai-prompt: first image -->\n![](images/first.png)\n\nSome text\n\n<!-- ai-prompt: second image -->\n![](images/second.png)",
			expectedCount: 2,
			expectedItems: []AIImage{
				{Prompt: "first image", ImagePath: "images/first.png"},
				{Prompt: "second image", ImagePath: "images/second.png"},
			},
		},
		{
			name:          "AI prompt without image",
			content:       "# Slide Title\n\n<!-- ai-prompt: orphan prompt -->\n\nSome text but no image",
			expectedCount: 0,
			expectedItems: nil,
		},
		{
			name:          "regular image without AI prompt",
			content:       "# Slide Title\n\n![Alt text](images/regular.png)",
			expectedCount: 0,
			expectedItems: nil,
		},
		{
			name:          "AI prompt with spaces",
			content:       "<!--   ai-prompt:   a detailed prompt with spaces   -->\n![](images/test.png)",
			expectedCount: 1,
			expectedItems: []AIImage{
				{Prompt: "a detailed prompt with spaces", ImagePath: "images/test.png"},
			},
		},
		{
			name:          "AI image with relative path",
			content:       "<!-- ai-prompt: test -->\n![](./images/local.png)",
			expectedCount: 1,
			expectedItems: []AIImage{
				{Prompt: "test", ImagePath: "./images/local.png"},
			},
		},
		{
			name:          "AI image with whitespace between comment and image",
			content:       "<!-- ai-prompt: test -->\n   \n![](images/test.png)",
			expectedCount: 0,
			expectedItems: nil, // Should not match if there's extra whitespace (blank line)
		},
		{
			name:          "AI image with newline and leading spaces",
			content:       "<!-- ai-prompt: test -->\n  ![](images/test.png)",
			expectedCount: 1,
			expectedItems: []AIImage{
				{Prompt: "test", ImagePath: "images/test.png"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			images := ParseAIImages(tt.content)

			if len(images) != tt.expectedCount {
				t.Errorf("expected %d AI images, got %d", tt.expectedCount, len(images))
			}

			for i, expected := range tt.expectedItems {
				if i >= len(images) {
					break
				}
				if images[i].Prompt != expected.Prompt {
					t.Errorf("image %d: expected prompt %q, got %q", i, expected.Prompt, images[i].Prompt)
				}
				if images[i].ImagePath != expected.ImagePath {
					t.Errorf("image %d: expected path %q, got %q", i, expected.ImagePath, images[i].ImagePath)
				}
			}
		})
	}
}

func TestGenerateImageFilename(t *testing.T) {
	tests := []struct {
		name           string
		imageData      []byte
		contentType    string
		expectedExt    string
		expectedPrefix string
	}{
		{
			name:           "PNG image",
			imageData:      []byte("fake png data"),
			contentType:    "image/png",
			expectedExt:    ".png",
			expectedPrefix: "generated-",
		},
		{
			name:           "JPEG image",
			imageData:      []byte("fake jpeg data"),
			contentType:    "image/jpeg",
			expectedExt:    ".jpg",
			expectedPrefix: "generated-",
		},
		{
			name:           "JPG content type",
			imageData:      []byte("fake jpg data"),
			contentType:    "image/jpg",
			expectedExt:    ".jpg",
			expectedPrefix: "generated-",
		},
		{
			name:           "GIF image",
			imageData:      []byte("fake gif data"),
			contentType:    "image/gif",
			expectedExt:    ".gif",
			expectedPrefix: "generated-",
		},
		{
			name:           "WebP image",
			imageData:      []byte("fake webp data"),
			contentType:    "image/webp",
			expectedExt:    ".webp",
			expectedPrefix: "generated-",
		},
		{
			name:           "Unknown content type defaults to PNG",
			imageData:      []byte("unknown data"),
			contentType:    "application/octet-stream",
			expectedExt:    ".png",
			expectedPrefix: "generated-",
		},
		{
			name:           "Empty content type defaults to PNG",
			imageData:      []byte("empty type data"),
			contentType:    "",
			expectedExt:    ".png",
			expectedPrefix: "generated-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := GenerateImageFilename(tt.imageData, tt.contentType)

			// Check prefix
			if !strings.HasPrefix(filename, tt.expectedPrefix) {
				t.Errorf("expected filename to start with %q, got %q", tt.expectedPrefix, filename)
			}

			// Check extension
			if !strings.HasSuffix(filename, tt.expectedExt) {
				t.Errorf("expected filename to end with %q, got %q", tt.expectedExt, filename)
			}

			// Check hash length (8 characters)
			// Format is "generated-{8 chars}.{ext}"
			hashStart := len("generated-")
			hashEnd := strings.LastIndex(filename, ".")
			if hashEnd-hashStart != 8 {
				t.Errorf("expected 8-character hash, got %d characters", hashEnd-hashStart)
			}

			// Check hash is valid hex
			hash := filename[hashStart:hashEnd]
			for _, c := range hash {
				if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
					t.Errorf("hash contains invalid hex character: %c", c)
				}
			}
		})
	}
}

func TestGenerateImageFilename_DifferentDataDifferentHash(t *testing.T) {
	data1 := []byte("first image data")
	data2 := []byte("second image data")

	filename1 := GenerateImageFilename(data1, "image/png")
	filename2 := GenerateImageFilename(data2, "image/png")

	if filename1 == filename2 {
		t.Error("different data should produce different filenames")
	}
}

func TestGenerateImageFilename_SameDataSameHash(t *testing.T) {
	data := []byte("identical image data")

	filename1 := GenerateImageFilename(data, "image/png")
	filename2 := GenerateImageFilename(data, "image/png")

	if filename1 != filename2 {
		t.Errorf("same data should produce same filename: %q != %q", filename1, filename2)
	}
}

func TestGetExtensionFromContentType(t *testing.T) {
	tests := []struct {
		contentType string
		expected    string
	}{
		{"image/png", "png"},
		{"image/jpeg", "jpg"},
		{"image/jpg", "jpg"},
		{"image/gif", "gif"},
		{"image/webp", "webp"},
		{"application/octet-stream", "png"},
		{"", "png"},
		{"text/html", "png"},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			result := GetExtensionFromContentType(tt.contentType)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestInsertImageIntoSlide_SingleSlide(t *testing.T) {
	content := `# My Slide

Some content here`

	result, err := InsertAIImage(content, 0, "A test prompt", "images/generated-abc123.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Should contain the AI prompt comment
	if !strings.Contains(result, "<!-- ai-prompt: A test prompt -->") {
		t.Error("result should contain AI prompt comment")
	}

	// Should contain the image reference
	if !strings.Contains(result, "![](images/generated-abc123.png)") {
		t.Error("result should contain image reference")
	}

	// AI prompt should come before image
	commentIdx := strings.Index(result, "<!-- ai-prompt:")
	imageIdx := strings.Index(result, "![](images/")
	if commentIdx >= imageIdx {
		t.Error("AI prompt comment should come before image reference")
	}

	// Original content should be preserved
	if !strings.Contains(result, "# My Slide") {
		t.Error("original heading should be preserved")
	}
	if !strings.Contains(result, "Some content here") {
		t.Error("original content should be preserved")
	}
}

func TestInsertImageIntoSlide_MultipleSlides(t *testing.T) {
	content := `# First Slide

Content one

---

# Second Slide

Content two

---

# Third Slide

Content three`

	// Insert into second slide
	result, err := InsertAIImage(content, 1, "Second slide image", "images/second.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Should contain the AI prompt comment
	if !strings.Contains(result, "<!-- ai-prompt: Second slide image -->") {
		t.Error("result should contain AI prompt comment")
	}

	// All slides should still be present
	if !strings.Contains(result, "# First Slide") {
		t.Error("first slide should be preserved")
	}
	if !strings.Contains(result, "# Second Slide") {
		t.Error("second slide should be preserved")
	}
	if !strings.Contains(result, "# Third Slide") {
		t.Error("third slide should be preserved")
	}

	// Separators should still be present
	if strings.Count(result, "---") != 2 {
		t.Errorf("expected 2 slide separators, got %d", strings.Count(result, "---"))
	}

	// Image should be in the second slide section (between first and second ---)
	parts := strings.Split(result, "---")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if !strings.Contains(parts[1], "![](images/second.png)") {
		t.Error("image should be in second slide section")
	}
	if strings.Contains(parts[0], "![](images/second.png)") {
		t.Error("image should not be in first slide section")
	}
	if strings.Contains(parts[2], "![](images/second.png)") {
		t.Error("image should not be in third slide section")
	}
}

func TestInsertImageIntoSlide_WithFrontmatter(t *testing.T) {
	content := `---
title: My Presentation
theme: paper
---

# First Slide

Content here

---

# Second Slide

More content`

	result, err := InsertAIImage(content, 0, "First slide prompt", "images/first.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Frontmatter should be preserved
	if !strings.Contains(result, "title: My Presentation") {
		t.Error("frontmatter title should be preserved")
	}
	if !strings.Contains(result, "theme: paper") {
		t.Error("frontmatter theme should be preserved")
	}

	// Image should be added to first slide
	if !strings.Contains(result, "<!-- ai-prompt: First slide prompt -->") {
		t.Error("AI prompt comment should be present")
	}
	if !strings.Contains(result, "![](images/first.png)") {
		t.Error("image reference should be present")
	}
}

func TestInsertImageIntoSlide_InvalidSlideIndex(t *testing.T) {
	content := `# Only Slide

Content`

	// Try to insert into non-existent slide
	_, err := InsertAIImage(content, 5, "prompt", "images/test.png")
	if err == nil {
		t.Error("expected error for invalid slide index")
	}
	if !strings.Contains(err.Error(), "invalid slide index") {
		t.Errorf("error should mention 'invalid slide index', got: %v", err)
	}

	// Try negative index
	_, err = InsertAIImage(content, -1, "prompt", "images/test.png")
	if err == nil {
		t.Error("expected error for negative slide index")
	}
}

func TestInsertImageIntoSlide_EmptySlidesSkipped(t *testing.T) {
	content := `# First Slide

Content

---



---

# Third Slide

More content`

	// Empty slide (index 1 would be empty, but it's skipped)
	// So slide index 1 should be "Third Slide"
	result, err := InsertAIImage(content, 1, "Third slide image", "images/third.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Image should be associated with "Third Slide"
	parts := strings.Split(result, "---")
	// Third part should contain the image
	found := false
	for _, part := range parts {
		if strings.Contains(part, "# Third Slide") && strings.Contains(part, "![](images/third.png)") {
			found = true
			break
		}
	}
	if !found {
		t.Error("image should be in the Third Slide section")
	}
}

func TestInsertImageIntoSlide_PreservesExistingImages(t *testing.T) {
	content := `# Slide With Images

Some text

<!-- ai-prompt: existing prompt -->
![](images/existing.png)

More text`

	result, err := InsertAIImage(content, 0, "new prompt", "images/new.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Existing image should be preserved
	if !strings.Contains(result, "![](images/existing.png)") {
		t.Error("existing image should be preserved")
	}
	if !strings.Contains(result, "<!-- ai-prompt: existing prompt -->") {
		t.Error("existing AI prompt should be preserved")
	}

	// New image should be added
	if !strings.Contains(result, "![](images/new.png)") {
		t.Error("new image should be added")
	}
	if !strings.Contains(result, "<!-- ai-prompt: new prompt -->") {
		t.Error("new AI prompt should be added")
	}
}

func TestInsertImageIntoSlide_LastSlide(t *testing.T) {
	content := `# First

Content

---

# Second

Content

---

# Last Slide

Final content`

	result, err := InsertAIImage(content, 2, "last prompt", "images/last.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Image should be in last slide
	if !strings.Contains(result, "<!-- ai-prompt: last prompt -->") {
		t.Error("AI prompt should be present")
	}
	if !strings.Contains(result, "![](images/last.png)") {
		t.Error("image reference should be present")
	}

	// Should not create extra separators
	if strings.Count(result, "---") != 2 {
		t.Errorf("expected 2 slide separators, got %d", strings.Count(result, "---"))
	}
}

func TestInsertImageIntoSlide_SpecialCharactersInPrompt(t *testing.T) {
	content := `# Slide

Content`

	// Prompt with special characters
	prompt := "A beautiful sunset with \"quotes\" and special chars: <>&"
	result, err := InsertAIImage(content, 0, prompt, "images/special.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Prompt should be preserved exactly
	expectedComment := "<!-- ai-prompt: A beautiful sunset with \"quotes\" and special chars: <>&"
	if !strings.Contains(result, expectedComment) {
		t.Errorf("expected prompt with special characters to be preserved, got:\n%s", result)
	}
}

func TestReplaceImageInContent(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		oldPrompt    string
		oldImagePath string
		newPrompt    string
		newImagePath string
		expected     string
		wantErr      bool
	}{
		{
			name: "simple replacement",
			content: `# Test Slide

<!-- ai-prompt: old prompt -->
![](images/old.png)

More content`,
			oldPrompt:    "old prompt",
			oldImagePath: "images/old.png",
			newPrompt:    "new prompt",
			newImagePath: "images/new.png",
			expected: `# Test Slide

<!-- ai-prompt: new prompt -->
![](images/new.png)

More content`,
			wantErr: false,
		},
		{
			name: "replacement with leading spaces on image line",
			content: `# Test Slide

<!-- ai-prompt: test prompt -->
  ![](images/test.png)

Content`,
			oldPrompt:    "test prompt",
			oldImagePath: "images/test.png",
			newPrompt:    "updated prompt",
			newImagePath: "images/updated.png",
			expected: `# Test Slide

<!-- ai-prompt: updated prompt -->
![](images/updated.png)

Content`,
			wantErr: false,
		},
		{
			name: "replacement in multi-slide markdown",
			content: `---
theme: paper
---

# Slide 1

Content one

---

# Slide 2

<!-- ai-prompt: middle slide image -->
![](images/middle.png)

---

# Slide 3

Content three`,
			oldPrompt:    "middle slide image",
			oldImagePath: "images/middle.png",
			newPrompt:    "regenerated image",
			newImagePath: "images/regen.png",
			expected: `---
theme: paper
---

# Slide 1

Content one

---

# Slide 2

<!-- ai-prompt: regenerated image -->
![](images/regen.png)

---

# Slide 3

Content three`,
			wantErr: false,
		},
		{
			name: "prompt not found",
			content: `# Test Slide

<!-- ai-prompt: different prompt -->
![](images/test.png)`,
			oldPrompt:    "nonexistent prompt",
			oldImagePath: "images/test.png",
			newPrompt:    "new prompt",
			newImagePath: "images/new.png",
			expected:     "",
			wantErr:      true,
		},
		{
			name: "image path not found",
			content: `# Test Slide

<!-- ai-prompt: test prompt -->
![](images/test.png)`,
			oldPrompt:    "test prompt",
			oldImagePath: "images/different.png",
			newPrompt:    "new prompt",
			newImagePath: "images/new.png",
			expected:     "",
			wantErr:      true,
		},
		{
			name: "replacement with special characters in prompt",
			content: `# Test Slide

<!-- ai-prompt: a (special) prompt with [brackets] and $symbols -->
![](images/special.png)`,
			oldPrompt:    "a (special) prompt with [brackets] and $symbols",
			oldImagePath: "images/special.png",
			newPrompt:    "a new (special) prompt",
			newImagePath: "images/new-special.png",
			expected: `# Test Slide

<!-- ai-prompt: a new (special) prompt -->
![](images/new-special.png)`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ReplaceAIImage(tt.content, tt.oldPrompt, tt.oldImagePath, tt.newPrompt, tt.newImagePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReplaceAIImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ReplaceAIImage() got:\n%s\n\nwant:\n%s", result, tt.expected)
			}
		})
	}
}
