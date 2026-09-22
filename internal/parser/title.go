package parser

import (
	"regexp"
	"strings"
)

// titleHeadingPattern matches one ATX heading line and captures its text.
var titleHeadingPattern = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)

// titleEmphasisPattern matches paired bold and code markers around their
// text. Underscores are left alone on purpose: a heading on these slides
// is far more likely to hold get_user_by_id or __init__ than _italics_,
// and mangling an identifier is worse than keeping a stray marker.
var titleEmphasisPattern = regexp.MustCompile("\\*\\*(.+?)\\*\\*|`(.+?)`")

// SlideTitle returns the text of a slide's first ATX heading, with bold
// and code markers removed, or "" when the slide has no heading. Fenced
// code is skipped, because a comment inside a code block is not a heading
// however much it looks like one.
func SlideTitle(content string) string {
	var fences fenceTracker
	for _, line := range strings.Split(content, "\n") {
		if fences.advance(line) {
			continue
		}
		match := titleHeadingPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if title := strings.TrimSpace(stripTitleEmphasis(match[1])); title != "" {
			return title
		}
	}
	return ""
}

// stripTitleEmphasis removes paired markers and keeps the text they
// wrapped.
func stripTitleEmphasis(heading string) string {
	return titleEmphasisPattern.ReplaceAllStringFunc(heading, func(match string) string {
		groups := titleEmphasisPattern.FindStringSubmatch(match)
		for _, group := range groups[1:] {
			if group != "" {
				return group
			}
		}
		return match
	})
}
