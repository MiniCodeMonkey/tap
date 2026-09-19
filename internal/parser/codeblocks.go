package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// codeBlockScanner is a plain goldmark instance used only to parse markdown
// into an AST for parseCodeBlocksFromMarkdown, which extracts code blocks
// without rendering HTML or going through a full Parser (a slide's
// directives, slots, or fragments are not relevant to that extraction).
var codeBlockScanner = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
		extension.Linkify,
	),
)

// codeBlockIndexAttr is the AST attribute (and, once rendered, the data
// attribute on the <code> element) that carries a fenced code block's
// per-slide position. It is assigned in document order during the same AST
// walk that collects CodeBlock values, so it survives a layout rendering
// slots (and therefore <pre> elements) in a different order than the source
// - the frontend pairs a live code or map block with its <pre> by this
// index, not by DOM position.
const codeBlockIndexAttr = "data-code-block-index"

// componentIndexAttr is the AST attribute (and, once rendered, the data
// attribute on the placeholder <div>) that carries an inline component
// fence's per-slide position, assigned the same way codeBlockIndexAttr is.
const componentIndexAttr = "data-component-index"

// componentFencePattern matches a ```component fence's info string: the
// word "component" followed by the component file path, e.g.
// "component ./charts/LatencyDrop.jsx". The path is everything after the
// first run of whitespace, trailing whitespace trimmed, so a path
// containing spaces (e.g. "component ./my slides/X.jsx") is captured
// whole instead of only up to its first space, the same way a "layout:"
// directive's value is a plain YAML string with no such restriction.
var componentFencePattern = regexp.MustCompile(`^component\s+(\S.*)$`)

// collectCodeBlocks walks a parsed AST for *ast.FencedCodeBlock nodes in
// document order (which also covers a fence indented inside a list item and
// a ~~~ fence - goldmark's block parser produces the same node kind for
// both). A fence whose info string is "component <path>" becomes a
// Component instead of a CodeBlock: it is assigned the next component
// index (nextComponentIndex) rather than a code block index, and its props
// JSON body is parsed and validated. Every other fence is assigned the next
// code block index (nextCodeIndex) and becomes a CodeBlock, built from the
// same node the renderer uses to write the code element. chunkStartLine and
// lineNumbersExact are forwarded to buildComponent so a props JSON error
// can name the real deck file line (see renderSlot). It returns the
// collected blocks and components plus the next free code block and
// component indices, so a caller threading fragments or multiple slots can
// chain calls the way nextFragmentIndex is threaded through renderSlot, and
// an error when a component fence's props body is not valid JSON or is not
// a JSON object.
func collectCodeBlocks(doc ast.Node, source []byte, nextCodeIndex int, nextComponentIndex int, chunkStartLine int, lineNumbersExact bool) ([]CodeBlock, []Component, int, int, error) {
	var blocks []CodeBlock
	var components []Component
	var walkErr error
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || walkErr != nil {
			return ast.WalkContinue, nil
		}
		fcb, ok := n.(*ast.FencedCodeBlock)
		if !ok {
			return ast.WalkContinue, nil
		}

		info := ""
		if fcb.Info != nil {
			info = strings.TrimSpace(string(fcb.Info.Segment.Value(source)))
		}

		if match := componentFencePattern.FindStringSubmatch(info); match != nil {
			index := nextComponentIndex
			nextComponentIndex++
			component, err := buildComponent(fcb, source, match[1], index, chunkStartLine, lineNumbersExact)
			if err != nil {
				walkErr = err
				return ast.WalkStop, nil
			}
			fcb.SetAttributeString(componentIndexAttr, []byte(intToString(index)))
			components = append(components, component)
			return ast.WalkContinue, nil
		}

		index := nextCodeIndex
		nextCodeIndex++
		fcb.SetAttributeString(codeBlockIndexAttr, []byte(intToString(index)))
		blocks = append(blocks, buildCodeBlock(fcb, source))
		return ast.WalkContinue, nil
	})
	return blocks, components, nextCodeIndex, nextComponentIndex, walkErr
}

// buildComponent builds a Component from a ```component fence AST node: its
// path (already extracted from the info string) and its body, parsed as
// props JSON. An empty body means "{}"; a body that is not valid JSON, or
// whose top-level value is not a JSON object, is a parse error. When
// lineNumbersExact is true, chunkStartLine plus the fence's line within
// source (fencedCodeBlockLine) gives the real line in the deck file, and
// the error names it; otherwise the real line cannot be reconstructed
// (content upstream of this chunk was rewritten in a way that shifts line
// numbers, such as notes comment extraction or the asciinema metadata
// rewrite), and the error names the component fence's path instead. A
// wrong line number is worse than none.
func buildComponent(fcb *ast.FencedCodeBlock, source []byte, path string, index int, chunkStartLine int, lineNumbersExact bool) (Component, error) {
	var body strings.Builder
	lines := fcb.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		body.Write(line.Value(source))
	}

	propsText := strings.TrimSpace(body.String())
	if propsText == "" {
		propsText = "{}"
	}

	where := func() string {
		if lineNumbersExact {
			return fmt.Sprintf("line %d", chunkStartLine+fencedCodeBlockLine(source, fcb)-1)
		}
		return fmt.Sprintf("in the component fence for %s", path)
	}

	var probe interface{}
	if err := json.Unmarshal([]byte(propsText), &probe); err != nil {
		return Component{}, fmt.Errorf("%s: invalid component props JSON: %w", where(), err)
	}
	if _, ok := probe.(map[string]interface{}); !ok {
		return Component{}, fmt.Errorf("%s: component props must be a JSON object", where())
	}

	return Component{Index: index, Source: path, Props: json.RawMessage(propsText)}, nil
}

// fencedCodeBlockLine returns the 1-based line, within source, that a fenced
// code block's opening fence starts on.
func fencedCodeBlockLine(source []byte, fcb *ast.FencedCodeBlock) int {
	offset := 0
	if fcb.Info != nil {
		offset = fcb.Info.Segment.Start
	} else if fcb.Lines().Len() > 0 {
		offset = fcb.Lines().At(0).Start
	}
	if offset > len(source) {
		offset = len(source)
	}
	return bytes.Count(source[:offset], []byte("\n")) + 1
}

// buildCodeBlock builds a CodeBlock (language, code, highlight/driver meta)
// from a fenced code block AST node, the same node the renderer uses to
// write the code element, so the extraction and the rendering can never
// disagree about which block or which meta they're looking at.
func buildCodeBlock(fcb *ast.FencedCodeBlock, source []byte) CodeBlock {
	block := CodeBlock{
		Language: string(fcb.Language(source)),
	}

	var code strings.Builder
	lines := fcb.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		code.Write(line.Value(source))
	}
	block.Code = strings.TrimSuffix(code.String(), "\n")

	if fcb.Info != nil {
		info := fcb.Info.Segment.Value(source)
		if _, meta, ok := splitCodeFenceInfo(string(info)); ok {
			if isHighlightLinesSpec(meta) {
				block.Meta.HighlightLines = normalizeHighlightLinesSpec(meta)
			} else {
				block.Meta = parseCodeBlockMeta(meta)
			}
		}
	}

	return block
}

// renderHTMLWithCodeBlocks parses content, assigns a running
// data-code-block-index to every fenced code block and a running
// data-component-index to every ```component fence, starting at
// nextCodeIndex and nextComponentIndex respectively, renders the AST to
// HTML with the parser's configured renderer (so the custom fenced code
// block renderer still writes highlight-lines, the code block index, or the
// component placeholder), and returns the collected CodeBlock and Component
// values alongside the next free indices for the caller to keep threading
// across slots and fragments.
func (p *Parser) renderHTMLWithCodeBlocks(content []byte, nextCodeIndex int, nextComponentIndex int, chunkStartLine int, lineNumbersExact bool) (string, int, int, []CodeBlock, []Component, error) {
	reader := text.NewReader(content)
	doc := p.md.Parser().Parse(reader)

	blocks, components, nextCodeIndex, nextComponentIndex, err := collectCodeBlocks(doc, content, nextCodeIndex, nextComponentIndex, chunkStartLine, lineNumbersExact)
	if err != nil {
		return "", nextCodeIndex, nextComponentIndex, nil, nil, err
	}

	var buf bytes.Buffer
	if err := p.md.Renderer().Render(&buf, content, doc); err != nil {
		return "", nextCodeIndex, nextComponentIndex, nil, nil, err
	}

	return buf.String(), nextCodeIndex, nextComponentIndex, blocks, components, nil
}

// parseCodeBlocksFromMarkdown extracts fenced code blocks directly from
// markdown content, without slide splitting or HTML rendering. It parses
// the content with a fresh AST pass and walks it exactly as
// renderHTMLWithCodeBlocks does, so standalone callers (tests, benchmarks)
// see the same result full slide parsing would produce. ```component
// fences are skipped (see collectCodeBlocks); any error from them is
// discarded since this helper has no slide context to attach it to.
func parseCodeBlocksFromMarkdown(content string) []CodeBlock {
	reader := text.NewReader([]byte(content))
	doc := codeBlockScanner.Parser().Parse(reader)
	blocks, _, _, _, _ := collectCodeBlocks(doc, []byte(content), 0, 0, 1, false)
	return blocks
}
