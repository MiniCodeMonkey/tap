package parser

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// highlightLinesSpecPattern matches a code block's meta content when it is a
// line-highlight spec rather than driver/connection metadata: digits,
// commas, dashes and spaces only, e.g. "3", "3-4", "1,3-5". This is what
// distinguishes "```php {3-4}" from "```sql {driver: mysql}".
var highlightLinesSpecPattern = regexp.MustCompile(`^[\d,\s-]+$`)

// isHighlightLinesSpec reports whether meta content (the text inside a code
// fence's trailing {...}) is a line-highlight spec.
func isHighlightLinesSpec(content string) bool {
	trimmed := strings.TrimSpace(content)
	return trimmed != "" && highlightLinesSpecPattern.MatchString(trimmed)
}

// normalizeHighlightLinesSpec strips whitespace from a line-highlight spec
// so "1, 3-5" and "1,3-5" produce the same data attribute value.
func normalizeHighlightLinesSpec(content string) string {
	return strings.ReplaceAll(strings.TrimSpace(content), " ", "")
}

// fencedCodeBlockRenderer renders ast.FencedCodeBlock the same way
// goldmark's default html.Renderer does, except it also reads the full,
// unparsed info string (goldmark's default renderer keeps only the first
// token as the language class and silently drops the rest) and, when one of
// its "{...}" groups is a line-highlight spec, carries it onto the rendered
// <code> tag as a data-highlight-lines attribute. The frontend's
// highlightCodeBlocksInElement reads that attribute to pass highlightLines
// through to Shiki.
type fencedCodeBlockRenderer struct {
	Writer html.Writer
}

// NewFencedCodeBlockRenderer returns a NodeRenderer that overrides fenced
// code block rendering to preserve a line-highlight spec from the info
// string. Register it with a lower priority number than goldmark's default
// html.Renderer (1000, see goldmark.New) so it wins registration for
// KindFencedCodeBlock.
func NewFencedCodeBlockRenderer() renderer.NodeRenderer {
	return &fencedCodeBlockRenderer{Writer: html.DefaultWriter}
}

func (r *fencedCodeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
}

func (r *fencedCodeBlockRenderer) renderFencedCodeBlock(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	n, ok := node.(*ast.FencedCodeBlock)
	if !ok {
		return ast.WalkContinue, nil
	}
	if index, ok := n.AttributeString(componentIndexAttr); ok {
		if entering {
			_, _ = w.WriteString(`<div class="deck-component" data-component-index="`)
			if indexBytes, ok := index.([]byte); ok {
				_, _ = w.Write(indexBytes)
			}
			_, _ = w.WriteString(`"></div>`)
		}
		return ast.WalkContinue, nil
	}
	if entering {
		_, _ = w.WriteString("<pre><code")
		language := n.Language(source)
		if language != nil {
			_, _ = w.WriteString(" class=\"language-")
			r.Writer.Write(w, language)
			_, _ = w.WriteString("\"")
		}
		if n.Info != nil {
			if meta := parseFenceMeta(string(n.Info.Segment.Value(source))); meta.HighlightLines != "" {
				_, _ = w.WriteString(" data-highlight-lines=\"")
				_, _ = w.WriteString(meta.HighlightLines)
				_, _ = w.WriteString("\"")
			}
		}
		if index, ok := n.AttributeString(codeBlockIndexAttr); ok {
			if indexBytes, ok := index.([]byte); ok {
				_, _ = w.WriteString(" " + codeBlockIndexAttr + "=\"")
				_, _ = w.Write(indexBytes)
				_, _ = w.WriteString("\"")
			}
		}
		_ = w.WriteByte('>')
		l := n.Lines().Len()
		for i := 0; i < l; i++ {
			line := n.Lines().At(i)
			r.Writer.RawWrite(w, line.Value(source))
		}
	} else {
		_, _ = w.WriteString("</code></pre>\n")
	}
	return ast.WalkContinue, nil
}

// splitCodeFenceInfo splits a fence info string into its language part
// and the content of each "{...}" group that ends it, in order. So
// "sql {driver: sqlite} {2-3}" gives "sql" and ["driver: sqlite", "2-3"].
func splitCodeFenceInfo(infoString string) (language string, groups []string) {
	remaining := strings.TrimSpace(infoString)
	for {
		match := metaPattern.FindStringSubmatch(remaining)
		if match == nil {
			break
		}
		groups = append([]string{match[1]}, groups...)
		remaining = strings.TrimSpace(remaining[:len(remaining)-len(match[0])])
	}
	return remaining, groups
}

// parseFenceMeta reads every "{...}" group of a fence info string. A group
// that is a line-highlight spec sets HighlightLines. Any other group sets
// Driver and Connection. The groups can come in either order.
func parseFenceMeta(infoString string) CodeBlockMeta {
	_, groups := splitCodeFenceInfo(infoString)
	var meta CodeBlockMeta
	for _, group := range groups {
		groupMeta := parseCodeBlockMeta(group)
		if groupMeta.HighlightLines != "" {
			meta.HighlightLines = groupMeta.HighlightLines
		}
		if groupMeta.Driver != "" {
			meta.Driver = groupMeta.Driver
		}
		if groupMeta.Connection != "" {
			meta.Connection = groupMeta.Connection
		}
	}
	return meta
}
