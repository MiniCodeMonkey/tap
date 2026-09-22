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
	hasBuffer  bool
	seq        uint64
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

// isCurrent reports whether seq is still the sequence of the buffer tap was
// most recently given. A render started for an older sequence uses this to
// tell it has been superseded, so it can discard its result instead of
// publishing a stale deck over a newer one.
func (source *appDeckSource) isCurrent(seq uint64) bool {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.seq == seq
}

// dropBuffer goes back to the deck file. The app sends "saved" after it
// wrote the buffer to disk.
func (source *appDeckSource) dropBuffer() {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.buffer = nil
	source.hasBuffer = false
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
// render runs unserialized: two PUTs in flight render concurrently, and a
// slower render for an older buffer is not allowed to block a faster
// render for a newer one. Before render publishes its result (for example
// by making the parsed presentation visible to other handlers), it must
// call current and skip publishing when current reports false, since that
// means a newer buffer has already arrived and its own render is the one
// that should be visible.
func handleAppSource(source *appDeckSource, render func(buffer []byte, current func() bool) error, baseDir string, log io.Writer) http.HandlerFunc {
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
		seq := source.setBuffer(buffer)
		if err := render(buffer, func() bool { return source.isCurrent(seq) }); err != nil {
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
