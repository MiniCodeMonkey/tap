package cli

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// askInBackground asks a question on its own goroutine and returns the
// channel its outcome arrives on.
func askInBackground(ctx context.Context, questions *appQuestions, kind string, payload any) <-chan [2]bool {
	outcome := make(chan [2]bool, 1)
	go func() {
		answer, answered := questions.ask(ctx, kind, payload)
		outcome <- [2]bool{answer, answered}
	}()
	return outcome
}

func receiveOutcome(t *testing.T, outcome <-chan [2]bool) [2]bool {
	t.Helper()
	select {
	case got := <-outcome:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("the question never returned")
		return [2]bool{}
	}
}

func TestAppQuestionsAskSendsAnEventAndWaitsForTheAnswer(t *testing.T) {
	events, log := newTestEvents(t)
	questions := newAppQuestions(events)
	outcome := askInBackground(context.Background(), questions, appQuestionKeepRecording, map[string]string{"directory": "/talks/recordings/run"})

	question := log.next(t, appEventQuestion)
	if question["id"] != "q1" || question["kind"] != "keep-recording" {
		t.Errorf("question = %v", question)
	}
	if payload, _ := question["payload"].(map[string]any); payload["directory"] != "/talks/recordings/run" {
		t.Errorf("payload = %v", question["payload"])
	}
	if err := questions.answer("q1", json.RawMessage("true")); err != nil {
		t.Fatal(err)
	}
	if got := receiveOutcome(t, outcome); got != [2]bool{true, true} {
		t.Errorf("outcome = %v, want answered true", got)
	}
}

func TestAppQuestionsRejectAnAnswerThatIsNotABoolean(t *testing.T) {
	events, log := newTestEvents(t)
	questions := newAppQuestions(events)
	outcome := askInBackground(context.Background(), questions, appQuestionApproval, map[string]string{})
	log.next(t, appEventQuestion)

	for _, value := range []string{"null", `"yes"`, "", "1"} {
		if err := questions.answer("q1", json.RawMessage(value)); !errors.Is(err, errInvalidAnswer) {
			t.Errorf("answer %q: error = %v, want errInvalidAnswer", value, err)
		}
	}
	if err := questions.answer("q1", json.RawMessage("false")); err != nil {
		t.Fatal(err)
	}
	if got := receiveOutcome(t, outcome); got != [2]bool{false, true} {
		t.Errorf("outcome = %v, want answered false", got)
	}
}

func TestAppQuestionsRejectAnUnknownID(t *testing.T) {
	events, _ := newTestEvents(t)
	if err := newAppQuestions(events).answer("q7", json.RawMessage("true")); !errors.Is(err, errUnknownQuestion) {
		t.Errorf("error = %v, want errUnknownQuestion", err)
	}
}

func TestAppQuestionsEndWhenTheContextEnds(t *testing.T) {
	events, log := newTestEvents(t)
	questions := newAppQuestions(events)
	ctx, cancel := context.WithCancel(context.Background())
	outcome := askInBackground(ctx, questions, appQuestionApproval, map[string]string{})
	log.next(t, appEventQuestion)

	cancel()
	if got := receiveOutcome(t, outcome); got[1] {
		t.Errorf("outcome = %v, want unanswered", got)
	}
	if err := questions.answer("q1", json.RawMessage("true")); !errors.Is(err, errUnknownQuestion) {
		t.Errorf("an answer after the question ended: error = %v, want errUnknownQuestion", err)
	}
}

func TestAppQuestionsCloseEndsEveryQuestion(t *testing.T) {
	events, log := newTestEvents(t)
	questions := newAppQuestions(events)
	outcome := askInBackground(context.Background(), questions, appQuestionApproval, map[string]string{})
	log.next(t, appEventQuestion)

	questions.close()
	if got := receiveOutcome(t, outcome); got[1] {
		t.Errorf("outcome = %v, want unanswered", got)
	}
	if _, answered := questions.ask(context.Background(), appQuestionApproval, map[string]string{}); answered {
		t.Error("a question after close was answered")
	}
	if log.drainHas(appEventQuestion) {
		t.Error("a question after close was sent")
	}
}

func TestAppApprovalAskerAsksAnApprovalQuestion(t *testing.T) {
	events, log := newTestEvents(t)
	questions := newAppQuestions(events)
	asker := appApprovalAsker{ctx: context.Background(), questions: questions}
	request := approvalRequest{
		Deck:    "/talks/talk.md",
		Drivers: []approvalDriver{{Name: "shell", Slides: []int{2}, Blocks: 1}},
		Blocks:  []approvalBlock{{Driver: "shell", Code: "ls", Slide: 2, Block: 1}},
	}
	approved := make(chan bool, 1)
	go func() {
		answer, _ := asker.askApproval(request)
		approved <- answer
	}()

	question := log.next(t, appEventQuestion)
	payload, _ := question["payload"].(map[string]any)
	if question["kind"] != "approval" || payload["deck"] != "/talks/talk.md" {
		t.Errorf("question = %v", question)
	}
	if err := questions.answer(question["id"].(string), json.RawMessage("true")); err != nil {
		t.Fatal(err)
	}
	if !<-approved {
		t.Error("a yes was not an approval")
	}
}

func TestAppConsentAskerAsksARecordConsentQuestion(t *testing.T) {
	events, log := newTestEvents(t)
	questions := newAppQuestions(events)
	asker := appConsentAsker{ctx: context.Background(), questions: questions, settingsPath: "/home/me/.config/tap/settings.yaml"}
	outcome := make(chan [2]bool, 1)
	go func() {
		record, answered := asker.askRecordConsent()
		outcome <- [2]bool{record, answered}
	}()

	question := log.next(t, appEventQuestion)
	payload, _ := question["payload"].(map[string]any)
	if question["kind"] != "record-consent" || payload["settingsPath"] != "/home/me/.config/tap/settings.yaml" {
		t.Errorf("question = %v", question)
	}
	if err := questions.answer(question["id"].(string), json.RawMessage("false")); err != nil {
		t.Fatal(err)
	}
	if got := receiveOutcome(t, outcome); got != [2]bool{false, true} {
		t.Errorf("outcome = %v, want answered false", got)
	}
}
