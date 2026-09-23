package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/slidelist"
)

func writeAppTestDeck(t *testing.T, content string) string {
	t.Helper()
	deckPath := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(deckPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return deckPath
}

func TestAppDeckSourceReadsTheFileUntilABufferArrives(t *testing.T) {
	deckPath := writeAppTestDeck(t, "# On Disk\n")
	source := newAppDeckSource(deckPath)

	if current, err := source.current(); err != nil || string(current) != "# On Disk\n" || source.buffering() {
		t.Errorf("before a buffer: %q, %v, buffering %v", current, err, source.buffering())
	}
	source.setBuffer([]byte("# In The Buffer\n"))
	if current, _ := source.current(); string(current) != "# In The Buffer\n" || !source.buffering() {
		t.Errorf("with a buffer: %q, buffering %v", current, source.buffering())
	}
	source.dropBuffer()
	if current, _ := source.current(); string(current) != "# On Disk\n" || source.buffering() {
		t.Errorf("after saved: %q, buffering %v", current, source.buffering())
	}
}

func TestAppDeckSourceTellsTheAppsOwnSaveFromAnOutsideChange(t *testing.T) {
	deckPath := writeAppTestDeck(t, "# One\n")
	source := newAppDeckSource(deckPath)
	source.remember([]byte("# One\n"))
	if changed, err := source.diskChanged(); err != nil || changed {
		t.Errorf("the file tap rendered: changed %v, %v", changed, err)
	}

	source.setBuffer([]byte("# Two\n"))
	if err := os.WriteFile(deckPath, []byte("# Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, _ := source.diskChanged(); changed {
		t.Error("the app's own save of its buffer counts as an outside change")
	}

	if err := os.WriteFile(deckPath, []byte("# Three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, _ := source.diskChanged(); !changed {
		t.Error("a change that differs from the buffer does not count")
	}

	source.dropBuffer()
	source.remember([]byte("# Three\n"))
	if changed, _ := source.diskChanged(); changed {
		t.Error("the file tap last rendered counts as a change")
	}
}

func putAppSource(handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "http://127.0.0.1:3000/api/app/source", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	handler(recorder, request)
	return recorder
}

type slideListResponse struct {
	OK     bool              `json:"ok"`
	Slides []slidelist.Slide `json:"slides"`
	Errors []string          `json:"errors"`
}

func TestAppSourceRendersTheBufferAndAnswersWithTheSlideList(t *testing.T) {
	deckPath := writeAppTestDeck(t, "# On Disk\n")
	source := newAppDeckSource(deckPath)
	var rendered []byte
	handler := handleAppSource(source, func(buffer []byte) (func(), error) {
		return func() { rendered = buffer }, nil
	}, filepath.Dir(deckPath), &bytes.Buffer{})

	response := putAppSource(handler, `{"source": "# One\n\n---\n\n# Two\n"}`)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status %d, content type %q: %s", response.Code, response.Header().Get("Content-Type"), response.Body)
	}
	var list slideListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if !list.OK || len(list.Slides) != 2 || list.Slides[1].Title != "Two" || list.Slides[1].StartLine != 5 || list.Errors == nil {
		t.Errorf("slide list = %+v", list)
	}
	if string(rendered) != "# One\n\n---\n\n# Two\n" {
		t.Errorf("rendered %q, want the buffer", rendered)
	}
	if current, _ := source.current(); string(current) != string(rendered) {
		t.Error("the buffer is not the current source")
	}
	if onDisk, _ := os.ReadFile(deckPath); string(onDisk) != "# On Disk\n" {
		t.Error("PUT wrote the deck file")
	}
}

func TestAppSourceRejectsABodyThatIsNotASource(t *testing.T) {
	source := newAppDeckSource(writeAppTestDeck(t, "# One\n"))
	renders := 0
	handler := handleAppSource(source, func([]byte) (func(), error) { renders++; return nil, nil }, t.TempDir(), &bytes.Buffer{})
	for _, body := range []string{`{}`, `{"source": 3}`, `{"source": "# A", "extra": 1}`, `not json`} {
		response := putAppSource(handler, body)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code": "invalid_request"`) {
			t.Errorf("body %s: status %d, %s", body, response.Code, response.Body)
		}
	}
	if renders != 0 || source.buffering() {
		t.Errorf("a rejected body rendered %d times, buffering %v", renders, source.buffering())
	}
}

func TestAppSourceAnswers413ForABodyOverTheLimit(t *testing.T) {
	handler := handleAppSource(newAppDeckSource(writeAppTestDeck(t, "# One\n")), func([]byte) (func(), error) { return nil, nil }, t.TempDir(), &bytes.Buffer{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "http://127.0.0.1:3000/api/app/source", strings.NewReader(`{"source": "a long deck"}`))
	request.Body = http.MaxBytesReader(recorder, request.Body, 10)
	handler(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status %d, want 413", recorder.Code)
	}
}

func TestAppSourceStillListsSlidesWhenTheBufferDoesNotRender(t *testing.T) {
	var log bytes.Buffer
	handler := handleAppSource(newAppDeckSource(writeAppTestDeck(t, "# One\n")), func([]byte) (func(), error) { return nil, errors.New("frontmatter: bad") }, t.TempDir(), &log)
	response := putAppSource(handler, `{"source": "# Still A Slide\n"}`)
	var list slideListResponse
	_ = json.Unmarshal(response.Body.Bytes(), &list)
	if response.Code != http.StatusOK || len(list.Slides) != 1 {
		t.Errorf("status %d, slides %+v", response.Code, list.Slides)
	}
	if !strings.Contains(log.String(), "frontmatter: bad") {
		t.Errorf("log = %q, want the render error", log.String())
	}
}

func TestAppSourceDiscardsAStaleRenderThatFinishesAfterANewerOne(t *testing.T) {
	source := newAppDeckSource(writeAppTestDeck(t, "# One\n"))
	oldEnteredRender := make(chan struct{})
	releaseOldRender := make(chan struct{})

	var mu sync.Mutex
	var published []byte

	handler := handleAppSource(source, func(buffer []byte) (func(), error) {
		if string(buffer) == "# Old\n" {
			close(oldEnteredRender)
			<-releaseOldRender
		}
		return func() {
			mu.Lock()
			published = buffer
			mu.Unlock()
		}, nil
	}, t.TempDir(), &bytes.Buffer{})

	oldDone := make(chan struct{})
	go func() {
		putAppSource(handler, `{"source": "# Old\n"}`)
		close(oldDone)
	}()

	// Wait until the older PUT's render has started (its sequence has
	// already been taken) before sending the newer PUT, so the sequences
	// are taken in order while the older render is still deliberately
	// slow.
	<-oldEnteredRender
	putAppSource(handler, `{"source": "# New\n"}`)

	// Only now let the older, slower render finish, after the newer,
	// faster render has already published.
	close(releaseOldRender)
	<-oldDone

	mu.Lock()
	defer mu.Unlock()
	if string(published) != "# New\n" {
		t.Errorf("published %q, want the newer buffer even though its render finished first", published)
	}
}

// The superseded render does not merely choose not to publish: its publish
// step is never called at all, because the deck source is what calls it and
// only ever does so while the render is still the newest one.
func TestAppSourceNeverRunsASupersededRendersPublishStep(t *testing.T) {
	source := newAppDeckSource(writeAppTestDeck(t, "# One\n"))
	entered := make(chan struct{})
	release := make(chan struct{})
	var published atomic.Int64

	build := func(text []byte) (func(), error) {
		if string(text) == "# Old\n" {
			close(entered)
			<-release
		}
		return func() { published.Add(1) }, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := source.renderBuffer([]byte("# Old\n"), build); err != nil {
			t.Error(err)
		}
	}()
	<-entered
	if err := source.renderBuffer([]byte("# New\n"), build); err != nil {
		t.Fatal(err)
	}
	close(release)
	<-done

	if got := published.Load(); got != 1 {
		t.Errorf("%d publishes, want only the newer render's", got)
	}
	if current, _ := source.current(); string(current) != "# New\n" {
		t.Errorf("current = %q, want the newer buffer", current)
	}
}

// A save supersedes a render of the buffer still in flight, so the buffer
// cannot land on screen after tap has gone back to the deck file.
func TestAppSourceDropsABufferRenderThatOutlivesTheSave(t *testing.T) {
	source := newAppDeckSource(writeAppTestDeck(t, "# On Disk\n"))
	entered := make(chan struct{})
	release := make(chan struct{})
	var published atomic.Int64

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := source.renderBuffer([]byte("# Buffer\n"), func([]byte) (func(), error) {
			close(entered)
			<-release
			return func() { published.Add(1) }, nil
		}); err != nil {
			t.Error(err)
		}
	}()
	<-entered
	source.dropBuffer()
	close(release)
	<-done

	if got := published.Load(); got != 0 {
		t.Errorf("the buffer published %d times after the save, want 0", got)
	}
}

// TestAppDeckSourceDoesNotBroadcastASupersededRendersList proves the
// residual race the re-review flagged: dev.go broadcasts whatever list
// renderCurrentAndList hands back, so a render whose publish step is
// skipped because a newer write landed must hand back a nil list too, not
// one built from text that will never reach the screen. The ordering is
// forced with channels, as the other supersession tests in this file do,
// rather than hoped for.
func TestAppDeckSourceDoesNotBroadcastASupersededRendersList(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "talk.md")
	if err := os.WriteFile(file, []byte("# One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := newAppDeckSource(file)

	entered := make(chan struct{})
	release := make(chan struct{})
	var publishedOld atomic.Int64

	oldDone := make(chan struct{})
	var oldList *slidelist.Result
	var oldErr error
	go func() {
		defer close(oldDone)
		oldList, oldErr = source.renderCurrentAndList(directory, func(text []byte) (func(), error) {
			close(entered)
			<-release
			return func() { publishedOld.Add(1) }, nil
		})
	}()

	// Wait until the older render has taken its sequence and started its
	// build, then land a newer write before letting it finish.
	<-entered
	source.setBuffer([]byte("# Newer\n---\n# Second\n"))
	close(release)
	<-oldDone

	if oldErr != nil {
		t.Fatalf("rendering: %v", oldErr)
	}
	if publishedOld.Load() != 0 {
		t.Error("the superseded render published, want its publish step never called")
	}
	if oldList != nil {
		t.Errorf("list = %+v, want nil: the render never published, so its list must not reach a caller that broadcasts it", oldList)
	}

	newList, err := source.renderCurrentAndList(directory, func(text []byte) (func(), error) {
		return func() {}, nil
	})
	if err != nil {
		t.Fatalf("rendering the newer text: %v", err)
	}
	if newList == nil {
		t.Fatal("no slide list for the newer, unsuperseded render")
	}
	if len(newList.Slides) != 2 {
		t.Errorf("the newer slide list has %d slides, want 2", len(newList.Slides))
	}
}

func TestLoadPresentationSourceRendersTheGivenText(t *testing.T) {
	deckPath := writeAppTestDeck(t, "# On Disk\n")
	presentation, _, _, _, _, err := loadPresentationSource([]byte("# In The Buffer\n"), deckPath, config.DefaultConfig(), filepath.Dir(deckPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(presentation.Slides) != 1 || !strings.Contains(presentation.Slides[0].HTML, "In The Buffer") {
		t.Errorf("slides = %+v, want the buffer's text", presentation.Slides)
	}
}

// TestAppDeckSourceListsTheTextItRendered covers the slide list that
// rides along with a file-changed event. build is handed the exact text
// that was snapshotted for this render, not text re-read afterwards, so a
// PUT landing while the render is going cannot leave build looking at one
// deck while a screen shows another. That same PUT also supersedes this
// render, so its list is correctly withheld; see
// TestAppDeckSourceDoesNotBroadcastASupersededRendersList for that half.
func TestAppDeckSourceListsTheTextItRendered(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "talk.md")
	if err := os.WriteFile(file, []byte("# Rendered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := newAppDeckSource(file)

	var built []byte
	list, err := source.renderCurrentAndList(directory, func(text []byte) (func(), error) {
		built = text
		// A PUT landing while this render is still going, which
		// supersedes it.
		source.setBuffer([]byte("# Newer\n---\n# Second\n"))
		return func() {}, nil
	})
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}
	if string(built) != "# Rendered\n" {
		t.Fatalf("rendered %q, want the deck file", built)
	}
	if list != nil {
		t.Errorf("list = %+v, want nil: this render was superseded by the PUT, so it must not publish a list either", list)
	}
}
