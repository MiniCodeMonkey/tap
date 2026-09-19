package parser

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Rune-count boundaries for a heading's data-length class: "short" up to
// headingLengthShortMax, "medium" up to headingLengthMediumMax, "long"
// above that. Every theme has to size its title type for a worst-case
// title (see docs/reference/theme-porting.md), which makes a short title
// look small by comparison; data-length gives a theme a hook to scale a
// short heading up.
const (
	headingLengthShortMax  = 24
	headingLengthMediumMax = 60
)

// headingLengthAttr is the attribute (and, once rendered, the data
// attribute) a heading carries its length class on.
const headingLengthAttr = "data-length"

// headingLengthTransformer assigns every h1-h3 heading a data-length
// attribute ("short", "medium", or "long") from its visible text length,
// counted in runes after inline markup (bold, code spans, links) is
// stripped away. It runs as a goldmark AST transformer, once per parse,
// before rendering: goldmark's default heading renderer already writes any
// attribute a node carries whose name starts with "data-" (see
// html.RenderAttributes), so no custom renderer is needed the way the
// fenced code block renderer in highlight_lines.go is. Auto heading IDs
// (parser.WithAutoHeadingID) are a separate attribute goldmark's own
// extension sets on the same node, so they render alongside data-length
// untouched. A heading written as raw HTML (`<h1>...</h1>` in the
// markdown source) never becomes an *ast.Heading node in the first place,
// so it is left untouched by construction, not by any special case here.
type headingLengthTransformer struct{}

// NewHeadingLengthTransformer returns a parser.ASTTransformer that assigns
// every h1-h3 heading its data-length attribute.
func NewHeadingLengthTransformer() parser.ASTTransformer {
	return &headingLengthTransformer{}
}

// Transform implements parser.ASTTransformer.
func (t *headingLengthTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := n.(*ast.Heading)
		if !ok || heading.Level > 3 {
			return ast.WalkContinue, nil
		}
		runeCount := len([]rune(headingText(heading, source)))
		heading.SetAttributeString(headingLengthAttr, []byte(headingLengthClass(runeCount)))
		return ast.WalkContinue, nil
	})
}

// headingLengthClass maps a rune count to its data-length class.
func headingLengthClass(runeCount int) string {
	switch {
	case runeCount <= headingLengthShortMax:
		return "short"
	case runeCount <= headingLengthMediumMax:
		return "medium"
	default:
		return "long"
	}
}

// headingText collects a node's visible text: every *ast.Text and
// *ast.String segment's value, and an autolink's label, in document order.
// Walking the full subtree rather than only direct children means text
// nested inside emphasis, strong, and code span markup is still counted;
// the markup nodes themselves (which carry no text of their own, only
// text-bearing children) contribute nothing extra.
func headingText(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := c.(type) {
		case *ast.Text:
			b.Write(v.Segment.Value(source))
		case *ast.String:
			b.Write(v.Value)
		case *ast.AutoLink:
			b.Write(v.Label(source))
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}
