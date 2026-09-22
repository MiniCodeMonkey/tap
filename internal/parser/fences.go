package parser

import "strings"

// fenceTracker follows fenced code blocks through markdown one line at a
// time, with CommonMark's rules. A fence opens with a run of at least
// three backticks or three tildes, indented by at most three spaces. A
// backtick fence's info string cannot hold a backtick. The block closes
// at a line that is a run of the same character, at least as long as the
// opening run, indented by at most three spaces, with only spaces after
// it. The zero value is outside any fence.
type fenceTracker struct {
	character byte
	length    int
}

// advance reads the next line and reports whether it belongs to a fenced
// code block, counting the opening and the closing fence lines.
func (tracker *fenceTracker) advance(line string) bool {
	character, run, rest, isRun := fenceRun(line)
	if tracker.length == 0 {
		if !isRun || (character == '`' && strings.Contains(rest, "`")) {
			return false
		}
		tracker.character, tracker.length = character, run
		return true
	}
	if isRun && character == tracker.character && run >= tracker.length && strings.TrimSpace(rest) == "" {
		tracker.character, tracker.length = 0, 0
	}
	return true
}

// fenceRun reports whether line starts, after at most three spaces, with
// a run of at least three backticks or three tildes. It returns the run's
// character and length, and the rest of the line after the run.
func fenceRun(line string) (character byte, run int, rest string, isRun bool) {
	indent := 0
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}
	if indent > 3 || indent == len(line) {
		return 0, 0, "", false
	}
	character = line[indent]
	if character != '`' && character != '~' {
		return 0, 0, "", false
	}
	run = 1
	for indent+run < len(line) && line[indent+run] == character {
		run++
	}
	if run < 3 {
		return 0, 0, "", false
	}
	return character, run, line[indent+run:], true
}
