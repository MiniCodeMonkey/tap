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
// token as the language class and silently drops the rest) and, when its
// trailing "{...}" is a line-highlight spec, carries it onto the rendered
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
	n := node.(*ast.FencedCodeBlock)
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
			info := n.Info.Segment.Value(source)
			if _, meta, ok := splitCodeFenceInfo(string(info)); ok && isHighlightLinesSpec(meta) {
				_, _ = w.WriteString(" data-highlight-lines=\"")
				_, _ = w.WriteString(normalizeHighlightLinesSpec(meta))
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

// splitCodeFenceInfo splits a fence info string, e.g. "php {3-4}", into its
// language part and the content of a trailing "{...}", mirroring the
// splitting parseCodeBlocks does. ok is false when there is no "{...}" meta.
func splitCodeFenceInfo(infoString string) (language string, meta string, ok bool) {
	trimmed := strings.TrimSpace(infoString)
	match := metaPattern.FindStringSubmatch(trimmed)
	if match == nil {
		return trimmed, "", false
	}
	language = strings.TrimSpace(trimmed[:len(trimmed)-len(match[0])])
	return language, match[1], true
}
