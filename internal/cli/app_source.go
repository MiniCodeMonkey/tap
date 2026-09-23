package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/slidelist"
)

// appDeckSource is the text tap dev --app renders: the app's unsaved
// buffer after a PUT /api/app/source, and the deck file otherwise. It also
// remembers the last text tap was given, so the watcher can tell the app's
// own save from a change made somewhere else.
type appDeckSource struct {
	file       string
	buffer     []byte
	remembered []byte
	mu         sync.Mutex
	// publishMu is held across the sequence check and the publish step of
	// a render, so a superseded render cannot slip its result in between
	// the two (see render).
	publishMu sync.Mutex
	hasBuffer bool
	seq       uint64
}

func newAppDeckSource(file string) *appDeckSource {
	return &appDeckSource{file: file}
}

// current returns the buffer when there is one, and the deck file
// otherwise.
func (source *appDeckSource) current() ([]byte, error) {
	source.mu.Lock()
	if source.hasBuffer {
		buffer := source.buffer
		source.mu.Unlock()
		return buffer, nil
	}
	source.mu.Unlock()
	return os.ReadFile(source.file)
}

// setBuffer makes buffer the text tap renders, until dropBuffer. It returns
// the sequence number assigned to buffer, which a render started for an
// older buffer can compare against isCurrent to tell whether a newer buffer
// has since arrived.
func (source *appDeckSource) setBuffer(buffer []byte) uint64 {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.buffer = buffer
	source.hasBuffer = true
	source.seq++
	return source.seq
}

// isCurrent reports whether seq is still the newest sequence tap has
// handed out. A render started for an older sequence has been superseded.
func (source *appDeckSource) isCurrent(seq uint64) bool {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.seq == seq
}

// currentSequence returns the text tap renders now, the buffer while there
// is one and the deck file otherwise, together with the sequence number of
// this render. Every render takes a sequence, not only a buffer's, so a
// render of the deck file supersedes a buffer render still in flight
// exactly as a newer buffer does. The sequence is taken before the file is
// read, so a buffer that arrives during the read is the newer one.
func (source *appDeckSource) currentSequence() ([]byte, uint64, error) {
	source.mu.Lock()
	source.seq++
	seq := source.seq
	if source.hasBuffer {
		buffer := source.buffer
		source.mu.Unlock()
		return buffer, seq, nil
	}
	source.mu.Unlock()

	text, err := os.ReadFile(source.file)
	if err != nil {
		return nil, 0, err
	}
	return text, seq, nil
}

// dropBuffer goes back to the deck file. The app sends "saved" after it
// wrote the buffer to disk. It takes a new sequence, so a render of the
// buffer still in flight is superseded by the save rather than publishing
// the buffer over the deck file afterwards.
func (source *appDeckSource) dropBuffer() {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.buffer = nil
	source.hasBuffer = false
	source.seq++
}

// appRenderBuilder renders text and returns the step that makes the result
// visible: serving the new deck and telling every open page. All the work
// happens before that step, and none of it is visible until the step runs,
// so a render that has been superseded is thrown away whole.
type appRenderBuilder func(text []byte) (publish func(), err error)

// renderBuffer makes buffer the text tap renders, until dropBuffer, and
// publishes a render of it.
func (source *appDeckSource) renderBuffer(buffer []byte, build appRenderBuilder) error {
	return source.render(buffer, source.setBuffer(buffer), build)
}

// renderCurrent renders the text tap shows now: the buffer while there is
// one, and the deck file otherwise.
func (source *appDeckSource) renderCurrent(build appRenderBuilder) error {
	text, seq, err := source.currentSequence()
	if err != nil {
		return err
	}
	return source.render(text, seq, build)
}

// render builds text and publishes the result only while seq is still the
// newest sequence. The check and the publish step happen together under
// publishMu, which every publish takes, so a superseded render cannot
// publish: it either finds a newer sequence and throws its result away, or
// it publishes first and the newer render waits and publishes over it.
// There is no order of events that leaves the older deck on screen, and
// the caller has nothing to remember, since publishing is the deck
// source's to do and not the caller's.
func (source *appDeckSource) render(text []byte, seq uint64, build appRenderBuilder) error {
	publish, err := build(text)
	if err != nil {
		return err
	}
	source.publishMu.Lock()
	defer source.publishMu.Unlock()
	if !source.isCurrent(seq) {
		return nil
	}
	if publish != nil {
		publish()
	}
	source.remember(text)
	return nil
}

// buffering reports whether tap renders the app's buffer.
func (source *appDeckSource) buffering() bool {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.hasBuffer
}

// remember records text as the last text tap was given to render.
func (source *appDeckSource) remember(text []byte) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.remembered = text
}

// diskChanged reports whether the deck file differs from what tap renders:
// the buffer while there is one, and otherwise the last text tap was
// given.
func (source *appDeckSource) diskChanged() (bool, error) {
	disk, err := os.ReadFile(source.file)
	if err != nil {
		return false, err
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.hasBuffer {
		return !bytes.Equal(disk, source.buffer), nil
	}
	return !bytes.Equal(disk, source.remembered), nil
}

// appSourceRequest is the body of PUT /api/app/source. The buffer travels
// as a JSON string, because the cross-site guard accepts only JSON bodies.
type appSourceRequest struct {
	Source *string `json:"source"`
}

// handleAppSource serves PUT /api/app/source. It renders the app's unsaved
// buffer, which tap keeps rendering until the app sends "saved", and
// answers with the buffer's slide list, the structure tap slide list
// --json prints. A buffer that does not render still gets its slide list,
// whose errors say what is wrong, and the pages keep the last good render.
//
// build runs unserialized: two PUTs in flight render concurrently, and a
// slower render for an older buffer is not allowed to block a faster
// render for a newer one. Which of them ends up on screen is not this
// handler's to decide: it hands the buffer to the deck source, which
// publishes a render only while it is still the newest one (see
// appDeckSource.render).
func handleAppSource(source *appDeckSource, build appRenderBuilder, baseDir string, log io.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			if server.IsBodyTooLarge(err) {
				server.WriteBodyTooLarge(w)
				return
			}
			writeAppSourceError(w, http.StatusBadRequest, codeInvalidRequest, fmt.Sprintf("reading the body: %v", err))
			return
		}
		var request appSourceRequest
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil || request.Source == nil {
			writeAppSourceError(w, http.StatusBadRequest, codeInvalidRequest, `the body must be {"source": "<the deck's markdown>"}`)
			return
		}

		buffer := []byte(*request.Source)
		if err := source.renderBuffer(buffer, build); err != nil {
			fmt.Fprintf(log, "Not showing the buffer: %v\n", err)
		}
		result, err := slidelist.Build(buffer, baseDir)
		if err != nil {
			writeAppSourceError(w, http.StatusInternalServerError, codeInternal, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := printJSONOK(w, result); err != nil {
			fmt.Fprintf(log, "Writing the slide list: %v\n", err)
		}
	}
}

// writeAppSourceError answers with the --json error object.
func writeAppSourceError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = printJSONError(w, code, message)
}
