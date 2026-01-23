// Package parser provides markdown parsing functionality for tap presentations.
package parser

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Parser handles markdown parsing for presentations.
type Parser struct {
	md goldmark.Markdown
}

// New creates a new Parser with goldmark configured for presentation parsing.
// It enables the following extensions:
//   - Table: GFM tables
//   - Strikethrough: ~~strikethrough~~ text
//   - TaskList: - [x] checkboxes
//   - Linkify: auto-link URLs
func New() *Parser {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
			extension.Linkify,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML in markdown
		),
	)

	return &Parser{
		md: md,
	}
}

// Markdown returns the underlying goldmark.Markdown instance.
func (p *Parser) Markdown() goldmark.Markdown {
	return p.md
}
