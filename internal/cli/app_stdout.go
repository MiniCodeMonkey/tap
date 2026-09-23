package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// appLogInUse is the bounded standard error writer of the --app run in
// progress, and nil outside --app mode. It outlives runDevServer on
// purpose: execute prints a failed command's error after the command has
// returned, and that print is the process's last line. It arrives after
// the descriptors have been given back, so it is the one write of the run
// the redirect does not cover, and it goes through this writer instead.
// execute closes it.
var (
	appLogMu    sync.Mutex
	appLogInUse *appLogWriter
)

// appLogForFinalOutput is the bounded writer standard error belongs to,
// or nil when this is not an --app run.
func appLogForFinalOutput() io.Writer {
	appLogMu.Lock()
	defer appLogMu.Unlock()
	if appLogInUse == nil {
		return nil
	}
	return appLogInUse
}

// closeAppLog flushes and closes the --app run's log writer, bounded at
// appLogCloseBound. It is the last thing the process does, and it is a
// no-op outside --app mode and on a second call.
func closeAppLog() {
	appLogMu.Lock()
	writer := appLogInUse
	appLogInUse = nil
	appLogMu.Unlock()
	if writer != nil {
		writer.close()
	}
}

// claimStdoutForApp takes the process's standard output and standard
// error descriptors away for the life of an --app run, and hands back the
// real standard output for the protocol along with the bounded writer
// that owns the log.
//
// Recognising a raw write and routing it somewhere safe does not work.
// The set of ways to name a descriptor is open -- an alias, a method on
// the file, io.WriteString, a color writer, a helper taking the writer as
// an argument, the log package's own copy of standard error captured at
// process start, a package that knows nothing about --app at all -- and
// the cost of missing one is the process's exit, because a write to a
// pipe the app is not draining blocks for as long as the app lives.
//
// So the descriptor is taken away instead. Both of the process's own
// descriptors point at one pipe, and a goroutine drains that pipe into
// the bounded log writer, which takes a line without waiting and drops
// and counts when its queue is full. After this returns there is nowhere
// a raw write can go except that pipe: a write from any package, through
// any alias, in any syntax, on any goroutine, is bounded because there is
// no other destination left. A child process inherits the two redirected
// descriptors, so its output goes into the same pipe and is bounded by
// the same drain.
//
// The protocol is safe from all of this because it no longer travels on
// the descriptor. Standard output is duplicated first, and that
// duplicate, which no package-level write can name, is what the event
// writer gets. One JSON object per line goes there and nothing else does.
// The log's real destination is a duplicate of standard error taken the
// same way, so the two streams the app reads are exactly what they were.
//
// Both duplicates are taken close-on-exec, which is the only thing that
// keeps them out of the children tap starts, and a deck's live code is a
// child process. A deck is what --app mode treats as hostile: a duplicate
// it could name would let it write whatever it liked on the stream the
// app parses as the protocol, forged events included, and would hand it
// the raw standard error the redirect exists to make unreachable. The two
// redirected descriptors are left inheritable on purpose, because a
// child's output belongs in the log like anything else.
//
// restore gives both descriptors back and waits, bounded, for what is
// still in the pipe to reach the log. The bounded writer stays open and
// registered across it, because the process has one more line to print
// after the command returns and that line must still reach the app.
func claimStdoutForApp() (protocol *os.File, log *appLogWriter, restore func(), err error) {
	standardOutput, standardError := int(os.Stdout.Fd()), int(os.Stderr.Fd())
	protocolDescriptor, err := duplicateCloseOnExec(standardOutput)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("duplicating standard output for the app protocol: %w", err)
	}
	logDescriptor, err := duplicateCloseOnExec(standardError)
	if err != nil {
		_ = unix.Close(protocolDescriptor)
		return nil, nil, nil, fmt.Errorf("duplicating standard error for the app log: %w", err)
	}
	protocol = os.NewFile(uintptr(protocolDescriptor), "app protocol")
	log = newAppLogWriter(os.NewFile(uintptr(logDescriptor), "app log"))

	reader, writer, err := os.Pipe()
	if err != nil {
		_ = unix.Close(protocolDescriptor)
		_ = unix.Close(logDescriptor)
		return nil, nil, nil, fmt.Errorf("opening the app log pipe: %w", err)
	}
	writeEnd, err := descriptorOf(writer)
	if err != nil {
		_ = reader.Close()
		_ = writer.Close()
		_ = unix.Close(protocolDescriptor)
		_ = unix.Close(logDescriptor)
		return nil, nil, nil, fmt.Errorf("taking the write end of the app log pipe: %w", err)
	}
	release := func() {
		_ = reader.Close()
		_ = writer.Close()
		_ = unix.Close(protocolDescriptor)
		_ = unix.Close(logDescriptor)
	}
	if err := unix.Dup2(writeEnd, standardOutput); err != nil {
		release()
		return nil, nil, nil, fmt.Errorf("pointing standard output at the app log pipe: %w", err)
	}
	if err := unix.Dup2(writeEnd, standardError); err != nil {
		_ = unix.Dup2(protocolDescriptor, standardOutput)
		release()
		return nil, nil, nil, fmt.Errorf("pointing standard error at the app log pipe: %w", err)
	}
	// The two descriptors above are the only copies of the write end that
	// are wanted. Closing this one is what lets restore's handing them
	// back end the drain goroutine's read.
	_ = writer.Close()

	drained := make(chan struct{})
	go drainAppLogPipe(reader, log, drained)

	appLogMu.Lock()
	appLogInUse = log
	appLogMu.Unlock()

	// A write to the app's log pipe after the app has gone gets SIGPIPE,
	// whose default is to kill the process. Ignoring it turns that into an
	// error return, so a recording is still finished when the app has
	// gone. The redirect itself cannot raise it: tap holds the read end of
	// the pipe the descriptors point at for as long as they point at it.
	brokenPipes := make(chan os.Signal, 1)
	signal.Notify(brokenPipes, syscall.SIGPIPE)
	go func() {
		for range brokenPipes {
		}
	}()

	var once sync.Once
	restore = func() {
		once.Do(func() {
			signal.Stop(brokenPipes)
			close(brokenPipes)
			_ = unix.Dup2(protocolDescriptor, standardOutput)
			_ = unix.Dup2(logDescriptor, standardError)
			// No copy of the write end is left now, so the drain
			// goroutine reads what is still in the pipe and stops. The
			// wait is bounded like every other wait on the quit path, and
			// what it gives up on is some advisory lines on a process
			// that is about to exit.
			timer := time.NewTimer(appLogCloseBound)
			defer timer.Stop()
			select {
			case <-drained:
			case <-timer.C:
			}
			_ = reader.Close()
		})
	}
	return protocol, log, restore, nil
}

// duplicateCloseOnExec is a second descriptor on the same open file that
// a child process does not inherit. os/exec keeps a descriptor out of a
// child only by relying on close-on-exec, and plain dup clears the flag,
// so the one call that takes the duplicate and sets the flag together is
// what makes the duplicate private. Doing it in two calls would leave a
// window in which another goroutine's exec inherits it.
func duplicateCloseOnExec(descriptor int) (int, error) {
	return unix.FcntlInt(uintptr(descriptor), unix.F_DUPFD_CLOEXEC, 0)
}

// descriptorOf is file's descriptor number, left exactly as os.Pipe made
// it. File.Fd would do the same thing and also put the descriptor back in
// blocking mode, and blocking is the one mode the redirect target must
// not be in.
//
// A blocking write end means a write that finds the pipe full waits for
// the drain goroutine to make room. That reads as a bounded wait, and for
// every writer in Go it is. It is not bounded for the runtime: a panic is
// printed with the runtime's own write to descriptor 2, and the runtime
// prints it only after stopping every goroutine in the process. The drain
// is one of them, so during that write the pipe has no reader and cannot
// acquire one. A trace larger than the room left in the pipe would wait
// there for room nothing will ever make, and a tap that has already
// panicked and is still running is worse than any log line.
//
// Non-blocking makes that write fail instead, so the runtime finishes
// printing what fits and the process exits on its own. The cost is
// ordinary lines: a write that finds the pipe full is lost rather than
// waited out, and can be cut short. That only happens while the drain is
// behind, which is the same load under which the bounded writer is
// already dropping, and a log line is advisory. What it buys is that the
// process's death never waits for anything.
//
// There is one write the redirect cannot bound, and it is the reason
// nothing pollable may sit on descriptor 1 or 2. A write through an
// *os.File that Go has registered with its runtime poller waits inside
// the poller when the destination is full, and that registration follows
// the open file description the file was made from, not the descriptor
// number. Pointing the number at the drained pipe with dup2 therefore
// leaves the registration on the file the number used to name. On Linux
// epoll holds that description itself, so the write waits for the app's
// unread log pipe to become writable and never returns; macOS resolves
// the same wait by descriptor number, so the write is woken by the new
// destination and lands. tap is clear of this because nothing puts a
// poller-registered file on either descriptor: os.Stdout and os.Stderr
// are os.NewFile over the blocking pipes an app hands tap, which Go
// leaves unpollable, so a write to them that finds the pipe full gets
// EAGAIN back as an error and returns. Files that are pollable, os.Pipe's
// ends among them, belong anywhere but descriptor 1 and 2, and the write
// end above is reached only through the two descriptors dup2 points at
// it, never through the *os.File os.Pipe returned.
func descriptorOf(file *os.File) (int, error) {
	connection, err := file.SyscallConn()
	if err != nil {
		return 0, err
	}
	descriptor := -1
	if err := connection.Control(func(raw uintptr) { descriptor = int(raw) }); err != nil {
		return 0, err
	}
	return descriptor, nil
}

// drainAppLogPipe moves everything written to the redirected descriptors
// into the bounded log writer, a line at a time so a dropped line is a
// whole line. It never waits on anything but the pipe: the writer takes a
// line without waiting and drops it when its queue is full, which is what
// makes a raw write to a redirected descriptor impossible to block on.
func drainAppLogPipe(reader *os.File, log io.Writer, drained chan<- struct{}) {
	defer close(drained)
	lines := bufio.NewReader(reader)
	for {
		line, err := lines.ReadBytes('\n')
		if len(line) > 0 {
			_, _ = log.Write(line)
		}
		if err != nil {
			return
		}
	}
}
