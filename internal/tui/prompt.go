package tui

import (
	"context"
	"errors"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// ErrTUIClosed is what AskOnTerminal returns once the TUI has ended, so
// nobody is left to answer.
var ErrTUIClosed = errors.New("the TUI has ended")

// terminalPromptMsg asks the TUI to hand the terminal to ask.
type terminalPromptMsg struct {
	ask  func(in io.Reader, out io.Writer)
	done chan error
}

// terminalPromptDoneMsg reports that a terminal prompt returned the
// terminal. It carries nothing: AskOnTerminal learns the outcome from the
// prompt's own done channel.
type terminalPromptDoneMsg struct{}

// terminalPrompt runs a question on the terminal while the TUI is
// suspended. It is a tea.ExecCommand, so Bubble Tea leaves the alternate
// screen and raw mode before it runs and restores both after.
type terminalPrompt struct {
	ask func(in io.Reader, out io.Writer)
	in  io.Reader
	out io.Writer
}

func (prompt *terminalPrompt) SetStdin(in io.Reader)   { prompt.in = in }
func (prompt *terminalPrompt) SetStdout(out io.Writer) { prompt.out = out }
func (prompt *terminalPrompt) SetStderr(io.Writer)     {}

func (prompt *terminalPrompt) Run() error {
	prompt.ask(prompt.in, prompt.out)
	return nil
}

// AskOnTerminal suspends the TUI, runs ask with the terminal's input and
// output, and resumes the TUI once ask returns. It is how tap asks a
// question on the terminal after the TUI has taken it over, such as a
// live code approval a reload needs. It returns ErrTUIClosed when the TUI
// has ended, and ctx's error when ctx ends before the TUI takes the
// question. Once the question is on screen it waits for ask to return.
func (m *DevModel) AskOnTerminal(ctx context.Context, ask func(in io.Reader, out io.Writer)) error {
	prompt := terminalPromptMsg{ask: ask, done: make(chan error, 1)}
	select {
	case <-m.closeCh:
		return ErrTUIClosed
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	select {
	case m.promptCh <- prompt:
	case <-m.closeCh:
		return ErrTUIClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-prompt.done:
		return err
	case <-m.closeCh:
		return ErrTUIClosed
	}
}

// runTerminalPrompt hands the terminal to prompt and listens for the next
// external event.
func (m *DevModel) runTerminalPrompt(prompt terminalPromptMsg) tea.Cmd {
	return tea.Batch(
		tea.Exec(&terminalPrompt{ask: prompt.ask}, func(err error) tea.Msg {
			prompt.done <- err
			return terminalPromptDoneMsg{}
		}),
		m.listenForEvents(),
	)
}
