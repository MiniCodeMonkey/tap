package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Question kinds in --app mode.
const (
	appQuestionApproval      = "approval"
	appQuestionRecordConsent = "record-consent"
	appQuestionKeepRecording = "keep-recording"
)

var (
	errUnknownQuestion = errors.New("no open question has this id")
	errInvalidAnswer   = errors.New("an answer must be true or false")
)

// appQuestions puts questions to the app as events and hands each answer
// from standard input to the question that waits for it. Only the tap
// process's parent can write standard input, so script on a slide page can
// never answer.
type appQuestions struct {
	events  *appEventWriter
	pending map[string]chan bool
	mu      sync.Mutex
	next    int
	closed  bool
}

func newAppQuestions(events *appEventWriter) *appQuestions {
	return &appQuestions{events: events, pending: make(map[string]chan bool)}
}

// ask sends a question event and waits for its answer. answered is false
// when ctx ends, or standard input closes, before an answer comes.
func (questions *appQuestions) ask(ctx context.Context, kind string, payload any) (answer, answered bool) {
	if ctx.Err() != nil {
		return false, false
	}
	questions.mu.Lock()
	if questions.closed {
		questions.mu.Unlock()
		return false, false
	}
	questions.next++
	id := fmt.Sprintf("q%d", questions.next)
	reply := make(chan bool, 1)
	questions.pending[id] = reply
	questions.mu.Unlock()

	questions.events.emit(appQuestionEvent{Type: appEventQuestion, ID: id, Kind: kind, Payload: payload})
	select {
	case answer, answered = <-reply:
		return answer, answered
	case <-ctx.Done():
		questions.mu.Lock()
		delete(questions.pending, id)
		questions.mu.Unlock()
		return false, false
	}
}

// answer hands value, a JSON true or false, to the open question id.
func (questions *appQuestions) answer(id string, value json.RawMessage) error {
	var answer bool
	switch strings.TrimSpace(string(value)) {
	case "true":
		answer = true
	case "false":
		answer = false
	default:
		return fmt.Errorf("%w: the answer to %q is %s", errInvalidAnswer, id, value)
	}

	questions.mu.Lock()
	reply, open := questions.pending[id]
	delete(questions.pending, id)
	questions.mu.Unlock()
	if !open {
		return fmt.Errorf("%w: %q", errUnknownQuestion, id)
	}
	reply <- answer
	close(reply)
	return nil
}

// close ends every open question unanswered, and every later question at
// once. Standard input has closed, so no answer can come.
func (questions *appQuestions) close() {
	questions.mu.Lock()
	defer questions.mu.Unlock()
	questions.closed = true
	for id, reply := range questions.pending {
		close(reply)
		delete(questions.pending, id)
	}
}

// appApprovalAsker asks the live code approval question as an event. An
// unanswered question is a no, so nothing is stored.
type appApprovalAsker struct {
	ctx       context.Context
	questions *appQuestions
}

func (asker appApprovalAsker) askApproval(request approvalRequest) (bool, error) {
	approved, _ := asker.questions.ask(asker.ctx, appQuestionApproval, request)
	return approved, nil
}

// recordConsentPayload is the payload of a record-consent question.
type recordConsentPayload struct {
	SettingsPath string `json:"settingsPath"`
}

// appConsentAsker asks whether to record every tap present run as an
// event.
type appConsentAsker struct {
	ctx          context.Context
	questions    *appQuestions
	settingsPath string
}

func (asker appConsentAsker) askRecordConsent() (record, answered bool) {
	return asker.questions.ask(asker.ctx, appQuestionRecordConsent, recordConsentPayload{SettingsPath: asker.settingsPath})
}
