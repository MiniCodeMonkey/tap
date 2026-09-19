package parser

import (
	"regexp"
	"strings"
)

// blockScalarKeyPattern matches a YAML mapping key introducing a block
// scalar value, e.g. "notes: |" or "notes: >-". Lines indented deeper than
// the key are the scalar's content, not further "key: value" pairs, and
// must not have their hex-looking values quoted.
var blockScalarKeyPattern = regexp.MustCompile(`^(\s*)[A-Za-z0-9_-]+:\s*[|>][+-]?\s*$`)

// hexColorLinePattern matches a bare "key: #hex" line, where the value is a
// 3, 4, 6, or 8 digit hex color (#rgb, #rgba, #rrggbb, #rrggbbaa) followed
// by nothing but trailing whitespace.
var hexColorLinePattern = regexp.MustCompile(`^(\s*)([A-Za-z0-9_-]+)(:\s*)(#(?:[0-9A-Fa-f]{8}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{4}|[0-9A-Fa-f]{3}))(\s*)$`)

// quoteHexColorValues quotes bare hex color values in "key: #hex" lines of
// a directive comment before it is handed to the YAML parser. YAML treats
// an unquoted "#" as the start of a comment, so "background: #111111"
// otherwise parses as an empty background. Lines inside a block scalar
// (e.g. the body of "notes: |") are left alone, and a value that is
// already quoted is left alone too, since the pattern only matches a bare
// "#" value.
func quoteHexColorValues(content string) string {
	lines := strings.Split(content, "\n")
	blockScalarIndent := -1 // -1 means not currently inside a block scalar

	for i, line := range lines {
		if blockScalarIndent >= 0 {
			if strings.TrimSpace(line) == "" {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if indent > blockScalarIndent {
				continue // still inside the block scalar
			}
			blockScalarIndent = -1 // dedented back out; reprocess this line below
		}

		if match := blockScalarKeyPattern.FindStringSubmatch(line); match != nil {
			blockScalarIndent = len(match[1])
			continue
		}

		if match := hexColorLinePattern.FindStringSubmatch(line); match != nil {
			lines[i] = match[1] + match[2] + match[3] + `"` + match[4] + `"` + match[5]
		}
	}

	return strings.Join(lines, "\n")
}
