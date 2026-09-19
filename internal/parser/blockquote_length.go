package parser

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Rune-count boundaries for a blockquote's data-length class: "short" up
// to blockquoteLengthShortMax, "medium" up to blockquoteLengthMediumMax,
// "long" above that. These sit higher than the heading thresholds (see
// heading_length.go): a quote is body copy, not a title, so it reads at a
// much greater length before a theme's quote layout needs to step its
// type size down.
const (
	blockquoteLengthShortMax  = 80
	blockquoteLengthMediumMax = 180
)

// blockquoteLengthAttr is the attribute (and, once rendered, the data
// attribute) a blockquote carries its length class on.
const blockquoteLengthAttr = "data-length"

// blockquoteLengthTransformer assigns every blockquote a data-length
// attribute ("short", "medium", or "long") from its visible text length,
// counted in runes after inline markup is stripped away, the same way
// headingLengthTransformer does for headings. It runs as a goldmark AST
// transformer, once per parse, before rendering: goldmark's default
// blockquote renderer already writes any attribute a node carries whose
// name starts with "data-", so no custom renderer is needed. A blockquote
// written as raw HTML never becomes an *ast.Blockquote node, so it is left
// untouched by construction.
type blockquoteLengthTransformer struct{}

// NewBlockquoteLengthTransformer returns a parser.ASTTransformer that
// assigns every blockquote its data-length attribute.
func NewBlockquoteLengthTransformer() parser.ASTTransformer {
	return &blockquoteLengthTransformer{}
}

// Transform implements parser.ASTTransformer.
func (t *blockquoteLengthTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		quote, ok := n.(*ast.Blockquote)
		if !ok {
			return ast.WalkContinue, nil
		}
		runeCount := len([]rune(headingText(quote, source)))
		quote.SetAttributeString(blockquoteLengthAttr, []byte(blockquoteLengthClass(runeCount)))
		return ast.WalkContinue, nil
	})
}

// blockquoteLengthClass maps a rune count to its data-length class.
func blockquoteLengthClass(runeCount int) string {
	switch {
	case runeCount <= blockquoteLengthShortMax:
		return "short"
	case runeCount <= blockquoteLengthMediumMax:
		return "medium"
	default:
		return "long"
	}
}
