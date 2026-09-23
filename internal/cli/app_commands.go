package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Command types on standard input in --app mode.
const (
	appCommandAnswer    = "answer"
	appCommandSaved     = "saved"
	appCommandReload    = "reload"
	appCommandTunnel    = "tunnel"
	appCommandRecording = "recording"
	appCommandQuit      = "quit"
)

// appCommandQueueSize is how many commands can wait for the control loop.
const appCommandQueueSize = 64

// appCommandLineLimit is the longest command line tap reads.
const appCommandLineLimit = 1 << 20

// appCommand is one line on standard input.
type appCommand struct {
	Start  *bool           `json:"start,omitempty"`
	Value  json.RawMessage `json:"value,omitempty"`
	Type   string          `json:"type"`
	ID     string          `json:"id,omitempty"`
	Action string          `json:"action,omitempty"`
}

// readAppCommands reads one JSON command per line from input until it
// ends. An answer goes straight to questions, so a question can be
// answered while the control loop waits on it. Every other command goes
// to commands. At the end of input, every question ends unanswered and
// commands closes, which tells the control loop to quit.
func readAppCommands(input io.Reader, questions *appQuestions, events *appEventWriter, commands chan<- appCommand) {
	defer close(commands)
	defer questions.close()

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), appCommandLineLimit)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var command appCommand
		if err := json.Unmarshal(line, &command); err != nil || command.Type == "" {
			events.emit(appErrorEvent{Type: appEventError, Code: appErrorInvalidCommand, Message: fmt.Sprintf("not a command: %.200s", line)})
			continue
		}
		if command.Type == appCommandAnswer {
			if err := questions.answer(command.ID, command.Value); err != nil {
				code := appErrorInvalidAnswer
				if errors.Is(err, errUnknownQuestion) {
					code = appErrorUnknownQuestion
				}
				events.emit(appErrorEvent{Type: appEventError, Code: code, Message: err.Error()})
			}
			continue
		}
		select {
		case commands <- command:
		default:
			events.emit(appErrorEvent{Type: appEventError, Code: appErrorBusy, Message: fmt.Sprintf("too many commands at once, so %s was dropped", command.Type)})
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Reading commands from standard input: %v\n", err)
	}
}
