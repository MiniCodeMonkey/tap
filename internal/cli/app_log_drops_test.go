package cli

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"testing"
)

// wedgeableLog is a standard error that can be wedged and released more
// than once, which is what an app that stops and resumes draining its log
// panel looks like. It collects every line it is given.
type wedgeableLog struct {
	gateMu sync.Mutex
	gate   chan struct{}
	mu     sync.Mutex
	buffer bytes.Buffer
}

func newWedgeableLog() *wedgeableLog {
	open := make(chan struct{})
	close(open)
	return &wedgeableLog{gate: open}
}

func (log *wedgeableLog) Write(data []byte) (int, error) {
	log.gateMu.Lock()
	gate := log.gate
	log.gateMu.Unlock()
	<-gate
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.buffer.Write(data)
}

// wedge stops the log being read, so the writer goroutine blocks on its
// next line and the queue behind it fills.
func (log *wedgeableLog) wedge() {
	log.gateMu.Lock()
	defer log.gateMu.Unlock()
	log.gate = make(chan struct{})
}

// release lets the log be read again.
func (log *wedgeableLog) release() {
	log.gateMu.Lock()
	defer log.gateMu.Unlock()
	close(log.gate)
}

func (log *wedgeableLog) String() string {
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.buffer.String()
}

// dropNotices is every drop notice in text, as the pair of numbers each
// one carries: the length of the run that just ended, and the total for
// the run of tap.
var dropNotices = regexp.MustCompile(`(\d+) log lines were dropped, (\d+) in total`)

func noticesIn(t *testing.T, text string) [][2]int {
	t.Helper()
	var notices [][2]int
	for _, match := range dropNotices.FindAllStringSubmatch(text, -1) {
		run, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatal(err)
		}
		total, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatal(err)
		}
		notices = append(notices, [2]int{run, total})
	}
	return notices
}

// wedgeAndDrop wedges the log, writes enough lines to fill the queue and
// lose the rest, and returns with the log still wedged.
func wedgeAndDrop(t *testing.T, writer *appLogWriter, log *wedgeableLog, lost int) {
	t.Helper()
	log.wedge()
	fmt.Fprintln(writer, "a line that reaches the writer goroutine and stops there")
	waitUntil(t, "the queue to fill", func() bool {
		for range appLogQueueSize {
			fmt.Fprintln(writer, "a line filling the queue")
		}
		return len(writer.lines) == appLogQueueSize
	})
	for range lost {
		fmt.Fprintln(writer, "a line with nowhere to go")
	}
}

// TestAppLogWriterCountsEveryDropOfTheRun checks what an app can learn
// about what it missed. A count that is only ever spent on the next
// successful write tells the app about one run of losses and says nothing
// about the runs before it, so a log panel cannot say how much of the run
// it is missing. The notice carries a running total as well as the length
// of the run that just ended.
func TestAppLogWriterCountsEveryDropOfTheRun(t *testing.T) {
	log := newWedgeableLog()
	writer := newAppLogWriter(log)

	wedgeAndDrop(t, writer, log, 500)
	log.release()
	waitUntil(t, "the queue to drain", func() bool { return len(writer.lines) == 0 })
	fmt.Fprintln(writer, "a line that fits again")

	wedgeAndDrop(t, writer, log, 700)
	log.release()
	waitUntil(t, "the queue to drain again", func() bool { return len(writer.lines) == 0 })
	fmt.Fprintln(writer, "another line that fits again")
	writer.close()

	notices := noticesIn(t, log.String())
	if len(notices) < 2 {
		t.Fatalf("the log carries %d drop notices with a running total, want one per run of drops:\n%s", len(notices), log.String())
	}
	first, second := notices[0], notices[1]
	if first[1] != first[0] {
		t.Errorf("the first notice reports %d dropped and %d in total, want the two to agree on the first run", first[0], first[1])
	}
	if second[1] != first[1]+second[0] {
		t.Errorf("the second notice reports %d dropped and %d in total, want %d: the total must carry the earlier run", second[0], second[1], first[1]+second[0])
	}
}

// TestAppLogWriterReportsTheLastDropsFromClose checks the losses a burst
// that ends while the log is still wedged would otherwise take with it.
// The count is spent on the next successful write, and a run that has no
// next write is a run the app is never told about: it cannot tell it
// missed anything at all. Close is the one moment a count is certain not
// to be spent later, so close spends it.
func TestAppLogWriterReportsTheLastDropsFromClose(t *testing.T) {
	log := newWedgeableLog()
	writer := newAppLogWriter(log)

	wedgeAndDrop(t, writer, log, 900)
	log.release()
	waitUntil(t, "the queue to drain", func() bool { return len(writer.lines) == 0 })
	writer.close()

	notices := noticesIn(t, log.String())
	if len(notices) != 1 {
		t.Fatalf("the log carries %d drop notices, want the one close flushes for a burst nothing followed:\n%s", len(notices), log.String())
	}
	if notices[0][0] < 900 {
		t.Errorf("close reported %d dropped lines, want at least the 900 the burst lost", notices[0][0])
	}
}
