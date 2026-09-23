package layouts

import (
	"fmt"
	"strings"
)

// Field is one value the slide wizard asks for when it writes a new slide.
type Field struct {
	Name        string
	Placeholder string
	Multiline   bool
}

// Template describes a new slide in one layout: what the wizard asks for,
// and, through RenderSlide, the markdown written from the answers.
type Template struct {
	Name        string
	Description string
	Fields      []Field
}

// templates is every layout's template, in the order the wizard lists
// them.
var templates = []Template{
	{Name: "title", Description: "Title slide with centered heading", Fields: []Field{
		{Name: "Title", Placeholder: "My Title"},
		{Name: "Subtitle", Placeholder: "Optional subtitle"},
	}},
	{Name: "section", Description: "Section header for topic transitions", Fields: []Field{
		{Name: "Section Title", Placeholder: "Section Name"},
	}},
	{Name: "default", Description: "Standard content slide", Fields: []Field{
		{Name: "Header", Placeholder: "Slide Header"},
		{Name: "Content", Placeholder: "Bullet points or paragraphs", Multiline: true},
	}},
	{Name: "two-column", Description: "Side-by-side content columns", Fields: []Field{
		{Name: "Header", Placeholder: "Optional Header"},
		{Name: "Left Column", Placeholder: "Left side content", Multiline: true},
		{Name: "Right Column", Placeholder: "Right side content", Multiline: true},
	}},
	{Name: "code-focus", Description: "Full-width code block", Fields: []Field{
		{Name: "Language", Placeholder: "go, python, javascript..."},
		{Name: "Code", Placeholder: "Your code here", Multiline: true},
	}},
	{Name: "quote", Description: "Styled blockquote with attribution", Fields: []Field{
		{Name: "Quote", Placeholder: "The quote text"},
		{Name: "Author", Placeholder: "Author name"},
	}},
	{Name: "big-stat", Description: "Large number with description", Fields: []Field{
		{Name: "Statistic", Placeholder: "99%"},
		{Name: "Description", Placeholder: "Description of the statistic"},
	}},
	{Name: "three-column", Description: "Three columns side by side", Fields: []Field{
		{Name: "Header", Placeholder: "Optional Header"},
		{Name: "Left Column", Placeholder: "Left column content", Multiline: true},
		{Name: "Center Column", Placeholder: "Center column content", Multiline: true},
		{Name: "Right Column", Placeholder: "Right column content", Multiline: true},
	}},
	{Name: "sidebar", Description: "Main content with a sidebar", Fields: []Field{
		{Name: "Header", Placeholder: "Slide Header"},
		{Name: "Content", Placeholder: "Main content", Multiline: true},
		{Name: "Sidebar", Placeholder: "Notes or references", Multiline: true},
	}},
	{Name: "split-media", Description: "Content beside an image or video", Fields: []Field{
		{Name: "Header", Placeholder: "Slide Header"},
		{Name: "Content", Placeholder: "What the image shows", Multiline: true},
		{Name: "Media", Placeholder: "images/screenshot.png"},
	}},
	{Name: "cover", Description: "Full-screen background image with a title", Fields: []Field{
		{Name: "Title", Placeholder: "Big statement"},
		{Name: "Background", Placeholder: "images/hero.jpg"},
	}},
	{Name: "blank", Description: "No layout styling, full control", Fields: []Field{
		{Name: "Content", Placeholder: "Anything", Multiline: true},
	}},
}

// Templates returns every layout's template, in the order the wizard lists
// them.
func Templates() []Template {
	return append([]Template(nil), templates...)
}

// Names returns every layout name, sorted.
func Names() []string {
	loadRegistry()
	return append([]string(nil), layoutNames...)
}

// RenderSlide returns the markdown of a new slide in the named layout,
// with no slide separator in front of it. values fill the template's
// fields in order; an empty or missing value takes the field's default.
func RenderSlide(name string, values []string) (string, error) {
	var b strings.Builder
	switch name {
	case "title":
		b.WriteString(fmt.Sprintf("# %s\n", getValueOrDefault(values, 0, "Title")))
		if subtitle := getValueOrDefault(values, 1, ""); subtitle != "" {
			b.WriteString(fmt.Sprintf("\n%s\n", subtitle))
		}
	case "section":
		b.WriteString(fmt.Sprintf("## %s\n", getValueOrDefault(values, 0, "Section")))
	case "default":
		b.WriteString(fmt.Sprintf("## %s\n\n", getValueOrDefault(values, 0, "Header")))
		b.WriteString(formatContent(getValueOrDefault(values, 1, "- Point one\n- Point two")))
		b.WriteString("\n")
	case "two-column":
		writeOptionalHeader(&b, getValueOrDefault(values, 0, ""))
		b.WriteString("::left\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 1, "Left content")))
		b.WriteString("\n\n::right\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 2, "Right content")))
		b.WriteString("\n")
	case "code-focus":
		b.WriteString(layoutDirective("code-focus"))
		language := getValueOrDefault(values, 0, "")
		code := getValueOrDefault(values, 1, "// Your code here")
		b.WriteString(fmt.Sprintf("```%s\n%s\n```\n", language, code))
	case "quote":
		b.WriteString(layoutDirective("quote"))
		b.WriteString(fmt.Sprintf("> %q\n", getValueOrDefault(values, 0, "Your quote here")))
		if author := getValueOrDefault(values, 1, ""); author != "" {
			b.WriteString(fmt.Sprintf(">\n> -- %s\n", author))
		}
	case "big-stat":
		b.WriteString(layoutDirective("big-stat"))
		b.WriteString(fmt.Sprintf("# %s\n\n%s\n", getValueOrDefault(values, 0, "100%"), getValueOrDefault(values, 1, "Description")))
	case "three-column":
		writeOptionalHeader(&b, getValueOrDefault(values, 0, ""))
		b.WriteString("::left\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 1, "Left content")))
		b.WriteString("\n\n::center\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 2, "Center content")))
		b.WriteString("\n\n::right\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 3, "Right content")))
		b.WriteString("\n")
	case "sidebar":
		b.WriteString(layoutDirective("sidebar"))
		b.WriteString(fmt.Sprintf("## %s\n\n", getValueOrDefault(values, 0, "Header")))
		b.WriteString(formatContent(getValueOrDefault(values, 1, "Main content")))
		b.WriteString("\n\n::sidebar\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 2, "- Note one\n- Note two")))
		b.WriteString("\n")
	case "split-media":
		b.WriteString(layoutDirective("split-media"))
		b.WriteString(fmt.Sprintf("## %s\n\n", getValueOrDefault(values, 0, "Header")))
		b.WriteString(formatContent(getValueOrDefault(values, 1, "Describe the image")))
		b.WriteString("\n\n::media\n\n")
		if media := getValueOrDefault(values, 2, ""); media != "" {
			b.WriteString(fmt.Sprintf("![](%s)\n", media))
		} else {
			b.WriteString("Image or video\n")
		}
	case "cover":
		if background := getValueOrDefault(values, 1, ""); background != "" {
			b.WriteString(fmt.Sprintf("<!--\nlayout: cover\nbackground: %s\n-->\n\n", background))
		} else {
			b.WriteString(layoutDirective("cover"))
		}
		b.WriteString(fmt.Sprintf("# %s\n", getValueOrDefault(values, 0, "Title")))
	case "blank":
		b.WriteString(layoutDirective("blank"))
		b.WriteString(formatContent(getValueOrDefault(values, 0, "Content")))
		b.WriteString("\n")
	default:
		return "", fmt.Errorf("unknown layout %q (valid layouts: %s)", name, strings.Join(Names(), ", "))
	}
	return b.String(), nil
}

// layoutDirective is the comment that sets a slide's layout, followed by a
// blank line.
func layoutDirective(name string) string {
	return fmt.Sprintf("<!--\nlayout: %s\n-->\n\n", name)
}

// writeOptionalHeader writes "## header" and a blank line, or nothing when
// header is empty.
func writeOptionalHeader(b *strings.Builder, header string) {
	if header != "" {
		b.WriteString(fmt.Sprintf("## %s\n\n", header))
	}
}

// getValueOrDefault returns values[index], or defaultValue when it is
// missing or empty.
func getValueOrDefault(values []string, index int, defaultValue string) string {
	if index < len(values) && values[index] != "" {
		return values[index]
	}
	return defaultValue
}

// formatContent trims each line of content and drops lines that are only
// whitespace, keeping the line breaks.
func formatContent(content string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	for index, line := range lines {
		result.WriteString(strings.TrimSpace(line))
		if index < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}
