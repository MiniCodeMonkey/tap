package recorder

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// chapterLinePattern matches "m:ss Title" and "h:mm:ss Title".
var chapterLinePattern = regexp.MustCompile(`^(?:(\d+):)?(\d+):(\d{2}) (.+)$`)

// segmentNamePattern matches a segment header line in chapters.txt, and
// the name of a segment file.
var segmentNamePattern = regexp.MustCompile(`^\d+\.mov$`)

// segmentChapters is one section of chapters.txt: a segment file and the
// chapters inside it, timed from the start of that segment.
type segmentChapters struct {
	name    string
	entries []chapter
}

// parseChapterFile reads the text Run writes: a segment file name, then
// its chapter lines, repeated for each segment. The mark is the line
// titled talkStartTitle; it is returned with slideIndex -1, as SetMark
// makes it.
func parseChapterFile(text string) []segmentChapters {
	var segments []segmentChapters
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if segmentNamePattern.MatchString(line) {
			segments = append(segments, segmentChapters{name: line})
			continue
		}
		match := chapterLinePattern.FindStringSubmatch(line)
		if match == nil || len(segments) == 0 {
			continue
		}
		hours, _ := strconv.Atoi(match[1])
		minutes, _ := strconv.Atoi(match[2])
		secondsPart, _ := strconv.Atoi(match[3])
		entry := chapter{
			offset: time.Duration(hours*3600+minutes*60+secondsPart) * time.Second,
			title:  match[4],
		}
		if entry.title == talkStartTitle {
			entry.slideIndex = -1
		}
		last := &segments[len(segments)-1]
		last.entries = append(last.entries, entry)
	}
	return segments
}

// joinChapters puts every segment's chapters on the timeline of the
// joined recording. starts maps a segment file name to where it begins in
// the joined recording: the sum of the durations of the segments before
// it. A section whose file is not in starts is dropped.
func joinChapters(segments []segmentChapters, starts map[string]time.Duration) []chapter {
	var joined []chapter
	for _, segment := range segments {
		start, known := starts[segment.name]
		if !known {
			continue
		}
		for _, entry := range segment.entries {
			entry.offset += start
			joined = append(joined, entry)
		}
	}
	return joined
}

// talkStart is where the talk starts on the joined timeline: the mark, or
// 0 when there is none.
func talkStart(joined []chapter) time.Duration {
	for _, entry := range joined {
		if entry.slideIndex == -1 {
			return entry.offset
		}
	}
	return 0
}

// talkChapters renders the chapter list for a video that begins at
// realStart on the joined timeline. realStart is at or before the mark,
// because a stream copy can only cut on a keyframe.
//
// Entries before the talk start are dropped. The mark itself is dropped
// when a slide chapter shares its time. The first remaining chapter is
// pinned to 0:00, since the video begins a little before it and a YouTube
// list must start there. Nothing is merged or dropped for being short.
func talkChapters(joined []chapter, realStart time.Duration) string {
	cut := talkStart(joined)

	var builder strings.Builder
	written := 0
	for _, entry := range joined {
		if entry.offset < cut {
			continue
		}
		if entry.slideIndex == -1 && hasSlideAt(joined, entry.offset) {
			continue
		}
		offset := entry.offset - realStart
		if written == 0 || offset < 0 {
			offset = 0
		}
		builder.WriteString(formatOffset(offset))
		builder.WriteString(" ")
		builder.WriteString(entry.title)
		builder.WriteString("\n")
		written++
	}
	return builder.String()
}

func hasSlideAt(joined []chapter, offset time.Duration) bool {
	for _, entry := range joined {
		if entry.slideIndex != -1 && entry.offset == offset {
			return true
		}
	}
	return false
}
