package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
// to commands. A line longer than appCommandLineLimit costs only that
// line: it is reported as one appErrorInvalidCommand event and the rest
// of it is discarded, so nothing further down the same line is ever
// parsed as a command, and the next line is read normally. At the end of
// input, every question ends unanswered and commands closes, which tells
// the control loop to quit. log is where the reader's own trouble goes,
// which in --app mode is the bounded writer that owns standard error.
func readAppCommands(input io.Reader, log io.Writer, questions *appQuestions, events *appEventWriter, commands chan<- appCommand) {
	defer close(commands)
	defer questions.close()

	reader := bufio.NewReaderSize(input, 64*1024)
	for {
		raw, oversized, err := readAppCommandLine(reader, appCommandLineLimit)
		if oversized {
			events.emit(appErrorEvent{Type: appEventError, Code: appErrorInvalidCommand, Message: fmt.Sprintf("line longer than %d bytes, dropped", appCommandLineLimit)})
		} else {
			handleAppCommandLine(raw, questions, events, commands)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Fprintf(log, "Reading commands from standard input: %v\n", err)
			}
			return
		}
	}
}

// readAppCommandLine reads one line from reader, whole. When the line is
// longer than limit, oversized is true, line is empty, and the rest of
// the oversized line, up to and including its newline, is still consumed
// from reader so it can never be mistaken for the start of the next
// line. err is io.EOF once there are no more lines.
func readAppCommandLine(reader *bufio.Reader, limit int) (line []byte, oversized bool, err error) {
	var buffer []byte
	for {
		chunk, isPrefix, readErr := reader.ReadLine()
		if readErr != nil {
			return nil, false, readErr
		}
		if !oversized {
			if len(buffer)+len(chunk) > limit {
				oversized = true
				buffer = nil
			} else {
				buffer = append(buffer, chunk...)
			}
		}
		if !isPrefix {
			return buffer, oversized, nil
		}
	}
}

// handleAppCommandLine parses one already-read line as a command and
// routes it: an answer straight to questions, every other command to the
// commands queue, and anything unparseable to an invalid_command event.
// A line that arrives while the queue is full is dropped as busy rather
// than made to wait, so the reader stays free for the next line.
func handleAppCommandLine(raw []byte, questions *appQuestions, events *appEventWriter, commands chan<- appCommand) {
	line := bytes.TrimSpace(raw)
	if len(line) == 0 {
		return
	}
	var command appCommand
	if err := json.Unmarshal(line, &command); err != nil || command.Type == "" {
		events.emit(appErrorEvent{Type: appEventError, Code: appErrorInvalidCommand, Message: fmt.Sprintf("not a command: %.200s", line)})
		return
	}
	if command.Type == appCommandAnswer {
		if err := questions.answer(command.ID, command.Value); err != nil {
			code := appErrorInvalidAnswer
			if errors.Is(err, errUnknownQuestion) {
				code = appErrorUnknownQuestion
			}
			events.emit(appErrorEvent{Type: appEventError, Code: code, Message: err.Error()})
		}
		return
	}
	select {
	case commands <- command:
	default:
		events.emit(appErrorEvent{Type: appEventError, Code: appErrorBusy, Message: fmt.Sprintf("too many commands at once, so %s was dropped", command.Type)})
	}
}
