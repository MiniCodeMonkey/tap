package tui

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// lockedBuffer is a bytes.Buffer the program writes from its own
// goroutine while the test reads it.
type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func TestAskOnTerminalHandsTheTerminalToThePrompt(t *testing.T) {
	model := NewDevModel(DevConfig{})
	input, typing, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer typing.Close()
	output := &lockedBuffer{}
	program := tea.NewProgram(model, tea.WithInput(input), tea.WithOutput(output), tea.WithoutSignalHandler())
	finished := make(chan struct{})
	go func() {
		_, _ = program.Run()
		model.Close()
		close(finished)
	}()
	defer func() {
		program.Quit()
		<-finished
	}()

	answered := make(chan string, 1)
	asked := make(chan error, 1)
	go func() {
		asked <- model.AskOnTerminal(context.Background(), func(in io.Reader, out io.Writer) {
			fmt.Fprint(out, "Allow this deck to run code? ")
			line, _ := bufio.NewReader(in).ReadString('\n')
			answered <- strings.TrimSpace(line)
		})
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(output.String(), "Allow this deck to run code? ") {
		if time.Now().After(deadline) {
			t.Fatalf("the prompt never reached the terminal: %q", output.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := typing.WriteString("y\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-answered:
		if line != "y" {
			t.Errorf("the prompt read %q, want y", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the prompt never read the answer")
	}
	select {
	case err := <-asked:
		if err != nil {
			t.Errorf("AskOnTerminal() = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("AskOnTerminal never returned")
	}
}

func TestAskOnTerminalFailsOnceTheTUIHasEnded(t *testing.T) {
	model := NewDevModel(DevConfig{})
	model.Close()
	err := model.AskOnTerminal(context.Background(), func(io.Reader, io.Writer) {
		t.Error("the prompt ran after the TUI ended")
	})
	if !errors.Is(err, ErrTUIClosed) {
		t.Errorf("AskOnTerminal() = %v, want ErrTUIClosed", err)
	}
}

func TestAskOnTerminalGivesUpWhenItsContextEndsBeforeTheQuestionShows(t *testing.T) {
	model := NewDevModel(DevConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := model.AskOnTerminal(ctx, func(io.Reader, io.Writer) {
		t.Error("the prompt ran after its context ended")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("AskOnTerminal() = %v, want context.Canceled", err)
	}
}
