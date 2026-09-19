package parser

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// directiveKeyNames lists every directive YAML key parseDirectives
// recognizes, other than "notes" (handled separately since its value can
// span multiple lines and does not have to be valid YAML). It is derived
// from directiveFields, the single source of truth applyDirectiveFields
// also iterates, so this list and the fields actually applied cannot drift
// apart. Used to tell a directive line apart from a notes line when a
// directive comment does not parse as YAML as a whole.
var directiveKeyNames = func() []string {
	names := make([]string, len(directiveFields))
	for i, field := range directiveFields {
		names[i] = field.key
	}
	return names
}()

// directiveKeyLinePattern matches a line that opens one of directiveKeyNames,
// e.g. "layout: big-stat" or "  scroll-speed: 3000", capturing the key and
// the rest of the line as its value.
var directiveKeyLinePattern = regexp.MustCompile(
	`^\s*(` + strings.Join(directiveKeyNames, "|") + `):(.*)$`,
)

// singleTokenDirectiveKeys lists the directive keys whose values are
// always a single token with no whitespace: a layout name or path, a
// transition name, a boolean, or an integer. This is used only to tell a
// directive line apart from notes prose inside a notes run (see
// splitMixedDirectiveComment); background, tag, and badge naturally take
// multi-word or path-like values, so any value counts as a directive for
// those keys.
var singleTokenDirectiveKeys = map[string]bool{
	"layout":       true,
	"transition":   true,
	"fragments":    true,
	"scroll":       true,
	"scroll-speed": true,
	"steps":        true,
}

// isPlausibleDirectiveValue reports whether value is a plausible value for
// key to appear inside a notes run: any value for background, tag, or
// badge, and a single whitespace-free token for the other directive keys.
func isPlausibleDirectiveValue(key, value string) bool {
	if !singleTokenDirectiveKeys[key] {
		return true
	}
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && !strings.ContainsAny(trimmed, " \t")
}

// matchDirectiveKeyLine reports whether line opens one of directiveKeyNames
// and, if so, returns that key and the rest of the line (trimmed) as its
// value.
func matchDirectiveKeyLine(line string) (key, value string, ok bool) {
	m := directiveKeyLinePattern.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return m[1], strings.TrimSpace(m[2]), true
}

// isNotesComment reports whether an HTML comment's inner content is a
// speaker-notes comment: its content starts with "notes:" after optional
// leading whitespace.
func isNotesComment(commentBody string) bool {
	return strings.HasPrefix(strings.TrimSpace(commentBody), "notes:")
}

// notesTextFromComment extracts the notes text from a comment body already
// known to satisfy isNotesComment. It first tries to parse the body as YAML
// with a "notes" key (this is how "notes: single line" and "notes: |"
// block scalars already work); when that fails, for example because the
// text contains an unquoted colon or starts with a quote, it falls back to
// treating everything after the "notes:" prefix as free text. Either way,
// surrounding blank lines are trimmed and inner line breaks are kept.
func notesTextFromComment(commentBody string) string {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(commentBody), &data); err == nil {
		if notes, ok := data["notes"].(string); ok {
			return strings.Trim(notes, " \t\n")
		}
	}

	trimmed := strings.TrimSpace(commentBody)
	rest := strings.TrimPrefix(trimmed, "notes:")
	return strings.Trim(rest, " \t\n")
}

// splitMixedDirectiveComment is used when the first directive comment as a
// whole fails to parse as YAML, which happens when it mixes ordinary
// directives with free-text notes that are not valid YAML themselves (for
// example, notes containing a colon or starting with a quote). It walks
// the comment line by line, in either order: a line starting with a known
// directive key (see directiveKeyNames) becomes its own directive line,
// and the "notes:" line plus every following line up to the next
// directive line become the free-text notes. hasNotes is false when no
// "notes:" line is found, in which case the caller keeps its existing
// behavior for an unparseable, non-notes comment. directiveData is the
// YAML map parsed from the recovered directive lines alone, or nil when
// there were none or they still didn't parse.
//
// Inside a notes run (after a "notes:" line and before the next accepted
// directive line), a line that merely starts with a directive key is only
// treated as a directive when its value is plausible for that key (see
// isPlausibleDirectiveValue); otherwise it is notes prose that happens to
// start with a word like "layout:". Before any "notes:" line, a directive
// key line is always a directive, regardless of its value.
func splitMixedDirectiveComment(commentBody string) (hasNotes bool, notesText string, directiveData map[string]interface{}) {
	lines := strings.Split(commentBody, "\n")
	var directiveLines []string
	var notesLines []string
	inNotes := false

	for _, line := range lines {
		if key, value, ok := matchDirectiveKeyLine(line); ok && (!inNotes || isPlausibleDirectiveValue(key, value)) {
			directiveLines = append(directiveLines, line)
			inNotes = false
			continue
		}

		switch {
		case isNotesComment(line):
			notesLines = append(notesLines, line)
			inNotes = true
		case inNotes:
			notesLines = append(notesLines, line)
		default:
			directiveLines = append(directiveLines, line)
		}
	}

	if len(notesLines) == 0 {
		return false, "", nil
	}

	notesText = notesTextFromComment(strings.Join(notesLines, "\n"))

	if len(directiveLines) > 0 {
		directiveYAML := quoteHexColorValues(strings.Join(directiveLines, "\n"))
		var data map[string]interface{}
		if err := yaml.Unmarshal([]byte(directiveYAML), &data); err == nil {
			directiveData = data
		}
	}

	return true, notesText, directiveData
}

// extractNotesComments removes every notes comment from slide content and
// returns the cleaned content along with the extracted notes texts in
// document order. Comments inside fenced code blocks are left alone (they
// are code, not notes), reusing the same fence tracking splitSlots uses. A
// non-notes comment, such as "<!-- pause -->", is left untouched.
func extractNotesComments(content string) (string, []string) {
	lines := strings.Split(content, "\n")
	var outLines []string
	var notes []string
	fenceLength := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		backticks := countLeadingBackticks(line)
		if fenceLength == 0 && backticks >= 3 {
			fenceLength = backticks
			outLines = append(outLines, line)
			continue
		}
		if fenceLength > 0 {
			if backticks >= fenceLength {
				if strings.TrimSpace(line[backticks:]) == "" {
					fenceLength = 0
				}
			}
			outLines = append(outLines, line)
			continue
		}

		// A comment does not have to start a line: "Some text <!-- notes: ...
		// -->" is text content followed by a notes comment, not a line that
		// merely happens to contain "<!--" somewhere past its own start.
		startOnLine := strings.Index(line, "<!--")
		if startOnLine == -1 {
			outLines = append(outLines, line)
			continue
		}
		before := line[:startOnLine]
		firstCommentLine := line[startOnLine:]

		// Gather lines until the closing "-->" is found.
		commentLines := []string{firstCommentLine}
		full := firstCommentLine
		end := i
		for !strings.Contains(full, "-->") && end+1 < len(lines) {
			end++
			commentLines = append(commentLines, lines[end])
			full += "\n" + lines[end]
		}

		if !strings.Contains(full, "-->") {
			// No closing marker found; not a well-formed comment, leave as-is.
			outLines = append(outLines, before+commentLines[0])
			outLines = append(outLines, commentLines[1:]...)
			i = end
			continue
		}

		// HTML (and goldmark) end a comment at the first "-->", not the
		// last, and comments do not nest. Because the gathering loop above
		// stops growing "full" as soon as any "-->" appears in it, this is
		// already the first occurrence, and everything after it is on the
		// closing line (commentLines[end]), never a later line.
		startIdx := strings.Index(full, "<!--")
		closeIdx := strings.Index(full, "-->")
		inner := full[startIdx+len("<!--") : closeIdx]
		leftover := full[closeIdx+len("-->"):]

		if isNotesComment(inner) {
			notes = append(notes, notesTextFromComment(inner))
			// Whitespace-only text before the comment is just indentation
			// (matches the old column-0 behavior of dropping the line
			// entirely); real text before it stays as content.
			keepBefore := ""
			if strings.TrimSpace(before) != "" {
				keepBefore = before
			}
			if keepBefore != "" || leftover != "" {
				outLines = append(outLines, keepBefore+leftover)
			}
			i = end
			continue
		}

		outLines = append(outLines, before+commentLines[0])
		outLines = append(outLines, commentLines[1:]...)
		i = end
	}

	return strings.Join(outLines, "\n"), notes
}
