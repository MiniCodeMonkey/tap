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
	handler := handleAppSource(source, func(buffer []byte) error { rendered = buffer; return nil }, filepath.Dir(deckPath), &bytes.Buffer{})

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
	handler := handleAppSource(source, func([]byte) error { renders++; return nil }, t.TempDir(), &bytes.Buffer{})
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
	handler := handleAppSource(newAppDeckSource(writeAppTestDeck(t, "# One\n")), func([]byte) error { return nil }, t.TempDir(), &bytes.Buffer{})
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
	handler := handleAppSource(newAppDeckSource(writeAppTestDeck(t, "# One\n")), func([]byte) error { return errors.New("frontmatter: bad") }, t.TempDir(), &log)
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
