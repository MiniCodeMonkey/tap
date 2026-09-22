package recorder

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/parser"
)

// SlideTitle names a slide for the chapter list: the text of its first
// heading (see parser.SlideTitle), or its number when it has none, which
// is the case for an image-only or component-only slide.
func SlideTitle(content string, index int) string {
	if title := parser.SlideTitle(content); title != "" {
		return title
	}
	return fmt.Sprintf("Slide %d", index+1)
}

// chapter is one entry in the list.
type chapter struct {
	offset     time.Duration
	title      string
	slideIndex int
}

// Chapters accumulates slide timings during a recording and renders them as
// a YouTube chapter list, which pastes into a video description unchanged
// and still reads as a plain outline in an editor.
type Chapters struct {
	mu      sync.Mutex
	start   time.Time
	lastAt  time.Time
	entries []chapter
	// mark is a single labelled moment, such as the start of the talk,
	// rendered in time order among the slide entries. A new mark replaces
	// the old one.
	mark *chapter
}

// NewChapters starts a list for a recording that began at start.
func NewChapters(start time.Time) *Chapters {
	return &Chapters{start: start}
}

// Add records that a slide came up at a point in time. A change that lands
// on the same slide as the previous entry is dropped, because it is not a
// new place in the video. A return to an earlier slide is kept, because it
// is.
func (c *Chapters) Add(at time.Time, slideIndex int, title string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) > 0 && c.entries[len(c.entries)-1].slideIndex == slideIndex {
		return
	}

	var offset time.Duration
	switch {
	case len(c.entries) == 0:
		// Whatever is on screen when recording starts is what a viewer
		// sees first, however long it took the hub to say so.
		offset = 0
	case at.Before(c.lastAt):
		// This arrival is out of order against the latest one actually
		// seen, even if that one's displayed offset was itself pinned or
		// clamped. Reusing the previous entry's displayed offset keeps
		// timestamps non-decreasing.
		offset = c.lastOffsetLocked()
	default:
		offset = at.Sub(c.start)
	}

	if at.After(c.lastAt) {
		c.lastAt = at
	}

	c.entries = append(c.entries, chapter{offset: offset, slideIndex: slideIndex, title: title})
}

// lastOffsetLocked is the most recent entry's offset. The caller holds the
// lock.
func (c *Chapters) lastOffsetLocked() time.Duration {
	if len(c.entries) == 0 {
		return 0
	}
	return c.entries[len(c.entries)-1].offset
}

// Empty reports whether anything was recorded.
func (c *Chapters) Empty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries) == 0
}

// SetMark places the chapter list's one mark at the given time.
func (c *Chapters) SetMark(at time.Time, title string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	offset := at.Sub(c.start)
	if offset < 0 {
		offset = 0
	}
	c.mark = &chapter{offset: offset, title: title, slideIndex: -1}
}

// ClearMark removes the mark.
func (c *Chapters) ClearMark() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mark = nil
}

// Render is the chapter list as text.
func (c *Chapters) Render() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var builder strings.Builder
	write := func(entry chapter) {
		builder.WriteString(formatOffset(entry.offset))
		builder.WriteString(" ")
		builder.WriteString(entry.title)
		builder.WriteString("\n")
	}

	markWritten := c.mark == nil
	for _, entry := range c.entries {
		if !markWritten && c.mark.offset <= entry.offset {
			write(*c.mark)
			markWritten = true
		}
		write(entry)
	}
	if !markWritten {
		write(*c.mark)
	}
	return builder.String()
}

// WriteTo writes the list to path.
func (c *Chapters) WriteTo(path string) error {
	return os.WriteFile(path, []byte(c.Render()), 0o600)
}

// formatOffset renders a duration the way a chapter list wants it: m:ss
// under an hour, h:mm:ss above it.
func formatOffset(offset time.Duration) string {
	if offset < 0 {
		offset = 0
	}

	totalSeconds := int(offset.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
