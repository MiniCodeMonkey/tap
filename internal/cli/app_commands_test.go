package cli

import (
	"context"
	"strings"
	"testing"
)

func TestReadAppCommandsRoutesAnswersAndQueuesCommands(t *testing.T) {
	events, log := newTestEvents(t)
	questions := newAppQuestions(events)
	outcome := askInBackground(context.Background(), questions, appQuestionApproval, map[string]string{})
	log.next(t, appEventQuestion)

	input := strings.NewReader(`{"type":"answer","id":"q1","value":true}` + "\n\n" +
		`{"type":"reload"}` + "\n" +
		`{"type":"tunnel","start":false}` + "\n" +
		`{"type":"recording","action":"stop"}` + "\n")
	commands := make(chan appCommand, appCommandQueueSize)
	readAppCommands(input, questions, events, commands)

	if got := receiveOutcome(t, outcome); got != [2]bool{true, true} {
		t.Errorf("the answer did not reach the question: %v", got)
	}
	var got []appCommand
	for command := range commands {
		got = append(got, command)
	}
	if len(got) != 3 || got[0].Type != appCommandReload || got[1].Type != appCommandTunnel || got[2].Action != "stop" {
		t.Fatalf("commands = %+v", got)
	}
	if got[1].Start == nil || *got[1].Start {
		t.Errorf("tunnel start = %v, want false", got[1].Start)
	}
}

func TestReadAppCommandsReportsBadLines(t *testing.T) {
	events, log := newTestEvents(t)
	questions := newAppQuestions(events)
	input := strings.NewReader("not json\n" +
		`{"type":"answer","id":"q9","value":true}` + "\n" +
		`{"id":"no type"}` + "\n")
	commands := make(chan appCommand, appCommandQueueSize)
	readAppCommands(input, questions, events, commands)

	for _, want := range []string{appErrorInvalidCommand, appErrorUnknownQuestion, appErrorInvalidCommand} {
		if event := log.next(t, appEventError); event["code"] != want {
			t.Errorf("error code = %v, want %s", event["code"], want)
		}
	}
}

func TestReadAppCommandsClosesQuestionsAtTheEndOfInput(t *testing.T) {
	events, log := newTestEvents(t)
	questions := newAppQuestions(events)
	outcome := askInBackground(context.Background(), questions, appQuestionKeepRecording, map[string]string{})
	log.next(t, appEventQuestion)

	commands := make(chan appCommand, appCommandQueueSize)
	readAppCommands(strings.NewReader(""), questions, events, commands)

	if got := receiveOutcome(t, outcome); got[1] {
		t.Errorf("outcome = %v, want unanswered once standard input closed", got)
	}
	if _, open := <-commands; open {
		t.Error("the command channel is still open")
	}
}

func TestReadAppCommandsDropsACommandWhenTheQueueIsFull(t *testing.T) {
	events, log := newTestEvents(t)
	commands := make(chan appCommand, 1)
	readAppCommands(strings.NewReader(`{"type":"reload"}`+"\n"+`{"type":"reload"}`+"\n"), newAppQuestions(events), events, commands)
	if event := log.next(t, appEventError); event["code"] != appErrorBusy {
		t.Errorf("error = %v, want busy", event)
	}
}
