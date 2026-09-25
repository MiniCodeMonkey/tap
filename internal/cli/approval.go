package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/MiniCodeMonkey/tap/internal/tui"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// approvalRequest is the question tap asks before a deck may run live
// code. It encodes to JSON, so a caller other than the terminal can show
// it.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type approvalRequest struct {
	Deck string `json:"deck"`
	// Drivers are the drivers a yes approves: every declared driver, or
	// only the new ones when the deck was approved before.
	Drivers []approvalDriver `json:"drivers"`
	// ApprovedBefore are the drivers an earlier answer approved.
	ApprovedBefore []string `json:"approvedBefore,omitempty"`
	// Blocks are the live code blocks that use Drivers, in slide order.
	Blocks []approvalBlock `json:"blocks"`
}

// approvalDriver is one driver in an approval request.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type approvalDriver struct {
	Name string `json:"name"`
	// Command is what a custom driver runs, with its arguments, as the
	// frontmatter writes it. Empty for a built-in driver.
	Command string `json:"command,omitempty"`
	// PreviousCommand is the command an earlier approval of this driver
	// covered, when the question asks because the command changed. Empty
	// for a driver never approved.
	PreviousCommand string `json:"previousCommand,omitempty"`
	// Slides are the numbers of the slides with a block that uses it.
	Slides []int `json:"slides"`
	Blocks int   `json:"blocks"`
}

// approvalBlock is one live code block in an approval request.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type approvalBlock struct {
	Driver string `json:"driver"`
	Code   string `json:"code"`
	Slide  int    `json:"slide"`
	Block  int    `json:"block"`
}

// approvalAsker puts an approval request to the person and returns the
// answer: terminalAsker on standard input before the TUI starts, the
// TUI's own terminal prompt while it runs, and appApprovalAsker as a
// question event in --app mode. Never the slide page, where page script
// could answer. An error means no answer came, which is not a no. ctx
// ends when tap no longer needs the answer; an asker that cannot take a
// question back, such as the terminal, may ignore it.
type approvalAsker interface {
	askApproval(ctx context.Context, request approvalRequest) (bool, error)
}

// approvalInput is everything the approval check reads.
type approvalInput struct {
	Now          func() time.Time
	Config       *config.Config
	Presentation *transformer.TransformedPresentation
	Asker        approvalAsker
	Out          io.Writer
	// Context ends every question the gate asks. Nil is
	// context.Background.
	Context      context.Context
	SettingsPath string
	// Deck is the deck file: an absolute or relative path, possibly
	// through a symlink. The gate resolves it before it compares or
	// stores anything, so it never has to already be canonical.
	Deck string
	// AllowCode is --allow-code: live code runs for this run, and nothing
	// is asked or stored.
	AllowCode bool
	// Interactive is true when tap may ask: a terminal on standard input
	// and no --headless, or --app mode.
	Interactive bool
}

// liveCodeApproval decides which drivers tap dev or tap present may run
// at startup, asking when it may. See liveCodeGate.
func liveCodeApproval(input approvalInput) (server.LiveCodePolicy, error) {
	return newLiveCodeGate(input).startup(input.Config, input.Presentation)
}

// liveCodeGate decides which drivers one run of tap dev or tap present may
// run, for the deck as it is now, and asks the person about any driver
// that is not approved. It decides at startup and again on every reload,
// so a deck that gains a driver, or whose custom driver's command
// changes, asks again instead of running it.
//
// A driver is approved as its name and its command line together (see
// usersettings.Driver): a yes stored in the settings, or a yes given in
// this run. A driver the person declined in this run is not asked about
// again until its command changes. The policy the gate sets lists only
// approved drivers, each with its approved command, and /api/execute
// checks the command the registry would run against it, so until the
// person answers, a new or changed driver is refused while every
// approved one keeps running.
//
// One question is open at a time. A reload that leaves the drivers in
// question unchanged asks nothing new; one that changes them withdraws
// the open question and asks about the new set instead. An asker that
// cannot withdraw its question, the terminal, is answered first, and the
// gate then asks about whatever is still not approved.
//
// Approval is matched by usersettings.DeckKey, which usersettings.
// ResolveDeck alone can produce: it resolves the deck to its absolute
// path, with symlinks resolved and, on a case-insensitive filesystem,
// its on-disk case restored. Without this, the same deck reached through
// a symlink, a relative path, a path with a ".." segment, or a different
// spelling of its name would not match its own approval, and worse, an
// unrelated file could be made to match one by spelling alone. A deck
// that cannot be resolved, most often because it no longer exists, fails
// closed: the gate never asks and never stores an approval for it.
type liveCodeGate struct {
	input  approvalInput
	asker  approvalAsker
	apply  func(server.LiveCodePolicy)
	change func()
	// config and presentation are the deck as the latest reload left it.
	config       *config.Config
	presentation *transformer.TransformedPresentation
	// open is the question being asked, nil when none is.
	open *openApprovalQuestion
	// approved and declined are the answers given in this run.
	approved []usersettings.Driver
	declined []usersettings.Driver
	// refusedNotice is the drivers the last notice that live code is off
	// named, so a run that cannot ask names each set once.
	refusedNotice string
	policy        server.LiveCodePolicy
	mu            sync.Mutex
	// asking is true while a goroutine runs askUntilSettled.
	asking bool
	// active is true once startup ran. Until then a reload only records
	// the deck, and startup decides for the latest one.
	active bool
	// reloaded is true once a reload recorded a deck.
	reloaded bool
	// unresolvedNoticed is true once the gate said the deck cannot be
	// resolved.
	unresolvedNoticed bool
}

// openApprovalQuestion is the question the gate is waiting on.
type openApprovalQuestion struct {
	withdraw context.CancelCauseFunc
	drivers  []usersettings.Driver
}

// approvalDecision is what the gate decides for one version of the deck.
type approvalDecision struct {
	policy server.LiveCodePolicy
	// wanted are the declared drivers that are neither approved nor
	// declined in this run: the ones a question asks about.
	wanted []usersettings.Driver
	// refused are the declared drivers that are not approved, declined
	// ones included.
	refused []usersettings.Driver
	// approvedBefore names the declared drivers already approved.
	approvedBefore []string
	// previousCommands holds, for each wanted driver approved before with
	// another command, that command.
	previousCommands map[string]string
	blocks           []approvalBlock
	deck             string
}

func newLiveCodeGate(input approvalInput) *liveCodeGate {
	if input.Context == nil {
		input.Context = context.Background()
	}
	return &liveCodeGate{input: input, asker: input.Asker, config: input.Config, presentation: input.Presentation}
}

// setOut changes where the gate writes its notices, for when the TUI
// takes over the terminal.
func (gate *liveCodeGate) setOut(out io.Writer) {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	gate.input.Out = out
}

// attach gives the gate the server: apply sets the policy /api/execute
// checks, and change runs after an answer changed it, so open pages
// fetch which drivers may run again.
func (gate *liveCodeGate) attach(apply func(server.LiveCodePolicy), change func()) {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	gate.apply = apply
	gate.change = change
}

// setAsker changes who the gate asks. A nil asker asks nothing, for the
// moments when nobody can be asked, such as between startup and the TUI
// taking over the terminal. Setting one asks about anything still
// waiting for a question.
func (gate *liveCodeGate) setAsker(asker approvalAsker) {
	gate.mu.Lock()
	gate.asker = asker
	start := gate.shouldStartAsking(gate.decide(gate.config, gate.presentation))
	gate.mu.Unlock()
	if start {
		go gate.askInBackground()
	}
}

// startup decides for the deck at startup, asks when it may, and returns
// the policy. It returns once there is nothing left to ask. A yes that
// cannot be saved is an error.
func (gate *liveCodeGate) startup(cfg *config.Config, presentation *transformer.TransformedPresentation) (server.LiveCodePolicy, error) {
	gate.mu.Lock()
	if !gate.reloaded {
		// A reload that came before startup is newer than cfg.
		gate.config, gate.presentation = cfg, presentation
	}
	gate.active = true
	decision := gate.decide(gate.config, gate.presentation)
	gate.setPolicy(decision.policy)
	if decision.policy.AllowAll {
		fmt.Fprintln(gate.input.Out, "Live code is on for this run (--allow-code). No approval is saved.")
	}
	gate.noticeRefused(decision)
	start := gate.shouldStartAsking(decision)
	gate.mu.Unlock()

	if start {
		if err := gate.askUntilSettled(); err != nil {
			return server.LiveCodePolicy{}, err
		}
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	return gate.policy, nil
}

// reload decides for the deck a reload loaded, sets the policy at once,
// and asks in the background about any driver still waiting for a
// question. Until the answer, the new driver is refused.
func (gate *liveCodeGate) reload(cfg *config.Config, presentation *transformer.TransformedPresentation) {
	gate.mu.Lock()
	gate.config, gate.presentation = cfg, presentation
	gate.reloaded = true
	if !gate.active {
		gate.mu.Unlock()
		return
	}
	decision := gate.decide(cfg, presentation)
	gate.setPolicy(decision.policy)
	gate.noticeRefused(decision)
	if gate.open != nil && !sameDrivers(gate.open.drivers, decision.wanted) {
		gate.open.withdraw(errQuestionWithdrawn)
	}
	start := gate.shouldStartAsking(decision)
	gate.mu.Unlock()
	if start {
		go gate.askInBackground()
	}
}

// askInBackground asks until settled on its own goroutine, and reports a
// yes that could not be saved: the yes still holds for this run.
func (gate *liveCodeGate) askInBackground() {
	if err := gate.askUntilSettled(); err != nil {
		fmt.Fprintln(gate.input.Out, err)
	}
}

// shouldStartAsking reports whether a new asking goroutine should start
// for decision, and marks one as started. The caller holds gate.mu.
func (gate *liveCodeGate) shouldStartAsking(decision approvalDecision) bool {
	if !gate.active || gate.asking || gate.asker == nil || !gate.input.Interactive || len(decision.wanted) == 0 {
		return false
	}
	gate.asking = true
	return true
}

// askUntilSettled asks about the latest deck until nothing is left to
// ask, the question goes unanswered, or the run ends. It returns an
// error when a yes cannot be saved. Only one runs at a time.
func (gate *liveCodeGate) askUntilSettled() error {
	gate.mu.Lock()
	defer func() {
		gate.asking = false
		gate.open = nil
		gate.mu.Unlock()
	}()
	for {
		decision := gate.decide(gate.config, gate.presentation)
		gate.setPolicy(decision.policy)
		asker := gate.asker
		if len(decision.wanted) == 0 || asker == nil || gate.input.Context.Err() != nil {
			return nil
		}
		ctx, withdraw := context.WithCancelCause(gate.input.Context)
		question := &openApprovalQuestion{withdraw: withdraw, drivers: decision.wanted}
		gate.open = question
		request := newApprovalRequest(decision.deck, gate.config, decision.blocks, driverNamesOf(decision.wanted), decision.approvedBefore)
		for index := range request.Drivers {
			request.Drivers[index].PreviousCommand = decision.previousCommands[request.Drivers[index].Name]
		}
		names := joinWithAnd(driverNamesOf(decision.wanted))

		gate.mu.Unlock()
		approved, askErr := asker.askApproval(ctx, request)
		withdrawn := errors.Is(context.Cause(ctx), errQuestionWithdrawn)
		withdraw(nil)
		gate.mu.Lock()
		gate.open = nil

		switch {
		case askErr != nil && withdrawn:
			// A reload changed what to ask. Ask about the latest deck.
			continue
		case askErr != nil:
			if gate.input.Context.Err() == nil {
				fmt.Fprintf(gate.input.Out, "Live code is off for %s in this run: %v\n", names, askErr)
			}
			return nil
		case !approved:
			fmt.Fprintf(gate.input.Out, "Live code is off for %s in this run. tap asks again next time.\n", names)
			gate.declined = append(gate.declined, question.drivers...)
			continue
		}

		// The yes holds for this run even when it cannot be saved.
		gate.approved = append(gate.approved, question.drivers...)
		saveErr := gate.save(question.drivers)
		gate.setPolicy(gate.decide(gate.config, gate.presentation).policy)
		if gate.change != nil {
			gate.change()
		}
		if saveErr != nil {
			return fmt.Errorf("saving the live code approval: %w", saveErr)
		}
	}
}

// save stores a yes for drivers in the user settings.
func (gate *liveCodeGate) save(drivers []usersettings.Driver) error {
	deckKey, err := usersettings.ResolveDeck(gate.input.Deck)
	if err != nil {
		return err
	}
	// Reload under the lock rather than reusing an earlier read: the
	// person may have taken a while to answer, and another tap process
	// could have saved its own approval for a different deck in the
	// meantime. Merging into a fresh read keeps that approval instead of
	// overwriting it.
	return usersettings.WithLock(gate.input.SettingsPath, func() error {
		fresh, err := usersettings.Load(gate.input.SettingsPath)
		if err != nil {
			fresh = usersettings.Settings{}
		}
		fresh.ApproveDrivers(deckKey, drivers, gate.input.Now())
		return usersettings.Save(gate.input.SettingsPath, fresh)
	})
}

// setPolicy records policy and hands it to the server. The caller holds
// gate.mu.
func (gate *liveCodeGate) setPolicy(policy server.LiveCodePolicy) {
	gate.policy = policy
	if gate.apply != nil {
		gate.apply(policy)
	}
}

// noticeRefused says live code is off for the refused drivers when the
// gate cannot ask, once for each set of them. The caller holds gate.mu.
func (gate *liveCodeGate) noticeRefused(decision approvalDecision) {
	if gate.input.Interactive || gate.input.AllowCode || len(decision.refused) == 0 {
		gate.refusedNotice = ""
		return
	}
	names := joinWithAnd(driverNamesOf(decision.refused))
	if names == gate.refusedNotice {
		return
	}
	gate.refusedNotice = names
	fmt.Fprintf(gate.input.Out, "Live code is off: this deck is not approved to run %s. Run tap dev or tap present in a terminal to approve it, or pass --allow-code for this run.\n", names)
}

// decide works out the policy and the drivers to ask about for one
// version of the deck. The caller holds gate.mu.
func (gate *liveCodeGate) decide(cfg *config.Config, presentation *transformer.TransformedPresentation) approvalDecision {
	if cfg == nil || presentation == nil {
		return approvalDecision{}
	}
	blocks := runnableBlocks(presentation)
	if len(blocks) == 0 {
		return approvalDecision{}
	}
	if gate.input.AllowCode {
		return approvalDecision{policy: server.LiveCodePolicy{AllowAll: true}}
	}

	deckKey, err := usersettings.ResolveDeck(gate.input.Deck)
	if err != nil {
		if !gate.unresolvedNoticed {
			gate.unresolvedNoticed = true
			fmt.Fprintf(gate.input.Out, "Live code is off: %s could not be resolved: %v\n", gate.input.Deck, err)
		}
		return approvalDecision{}
	}
	settings, err := usersettings.Load(gate.input.SettingsPath)
	if err != nil {
		// A malformed settings file must not stop the talk. A yes
		// overwrites it with a well-formed one.
		fmt.Fprintf(gate.input.Out, "Ignoring %s, it could not be read: %v\n", gate.input.SettingsPath, err)
		settings = usersettings.Settings{}
	}

	decision := approvalDecision{blocks: blocks, deck: deckKey.String(), policy: server.LiveCodePolicy{Drivers: []string{}}}
	for _, name := range cfg.DeclaredDrivers() {
		driver := usersettings.Driver{Name: name, Command: approvalCommand(name, cfg.Drivers[name])}
		switch {
		case settings.Covers(deckKey, driver) || containsDriver(gate.approved, driver):
			decision.approvedBefore = append(decision.approvedBefore, name)
			decision.policy.Drivers = append(decision.policy.Drivers, name)
			if driver.Command != nil {
				if decision.policy.Commands == nil {
					decision.policy.Commands = map[string][]string{}
				}
				decision.policy.Commands[name] = driver.Command
			}
		case containsDriver(gate.declined, driver):
			decision.refused = append(decision.refused, driver)
		default:
			decision.refused = append(decision.refused, driver)
			decision.wanted = append(decision.wanted, driver)
			if previous := gate.previousCommand(settings, deckKey, name); previous != nil && !slices.Equal(previous, driver.Command) {
				if decision.previousCommands == nil {
					decision.previousCommands = map[string]string{}
				}
				decision.previousCommands[name] = strings.Join(previous, " ")
			}
		}
	}
	return decision
}

// previousCommand returns the command an earlier approval of the driver
// name covered: the stored approval's, or else the one approved in this
// run. It is nil when neither approved it with a command. The caller
// holds gate.mu.
func (gate *liveCodeGate) previousCommand(settings usersettings.Settings, deckKey usersettings.DeckKey, name string) []string {
	if approval, found := settings.ApprovalFor(deckKey); found && slices.Contains(approval.Drivers, name) && approval.Commands[name] != nil {
		return approval.Commands[name]
	}
	for index := len(gate.approved) - 1; index >= 0; index-- {
		if gate.approved[index].Name == name && gate.approved[index].Command != nil {
			return gate.approved[index].Command
		}
	}
	return nil
}

// approvalCommand returns the command line an approval of a driver
// covers: what buildDriverRegistry has the driver run, with its ${NAME}
// variables expanded, so the approval matches the registry's CommandLine.
// It is nil for a driver that runs no command of its own: a built-in
// driver, a custom one with no command, and one whose command cannot be
// expanded, which runs nothing but an error.
func approvalCommand(name string, settings config.DriverConfig) []string {
	if settings.Command == "" || slices.Contains(builtInDriverNames, name) {
		return nil
	}
	command, args, err := settings.ExpandedCommand(name, os.LookupEnv)
	if err != nil {
		return nil
	}
	return append([]string{command}, args...)
}

func containsDriver(drivers []usersettings.Driver, driver usersettings.Driver) bool {
	return slices.ContainsFunc(drivers, func(candidate usersettings.Driver) bool {
		return candidate.Name == driver.Name && slices.Equal(candidate.Command, driver.Command)
	})
}

// sameDrivers reports whether a and b hold the same drivers, in order.
func sameDrivers(a, b []usersettings.Driver) bool {
	return slices.EqualFunc(a, b, func(x, y usersettings.Driver) bool {
		return x.Name == y.Name && slices.Equal(x.Command, y.Command)
	})
}

func driverNamesOf(drivers []usersettings.Driver) []string {
	names := make([]string, len(drivers))
	for index, driver := range drivers {
		names[index] = driver.Name
	}
	return names
}

// runnableBlocks returns the live code blocks that use a declared driver,
// in slide order.
func runnableBlocks(presentation *transformer.TransformedPresentation) []approvalBlock {
	var blocks []approvalBlock
	for _, slide := range presentation.Slides {
		for _, block := range slide.CodeBlocks {
			if block.Block == 0 || block.Problem != "" {
				continue
			}
			blocks = append(blocks, approvalBlock{Driver: block.Driver, Code: block.Code, Slide: slide.Index + 1, Block: block.Block})
		}
	}
	return blocks
}

// newApprovalRequest builds the question for the drivers in wanted.
func newApprovalRequest(deck string, cfg *config.Config, blocks []approvalBlock, wanted, approvedBefore []string) approvalRequest {
	request := approvalRequest{Deck: deck, ApprovedBefore: approvedBefore, Blocks: []approvalBlock{}}
	for _, name := range wanted {
		entry := approvalDriver{Name: name, Slides: []int{}}
		if settings := cfg.Drivers[name]; settings.Command != "" && !slices.Contains(builtInDriverNames, name) {
			entry.Command = displayedCommand(name, settings)
		}
		for _, block := range blocks {
			if block.Driver != name {
				continue
			}
			entry.Blocks++
			if !slices.Contains(entry.Slides, block.Slide) {
				entry.Slides = append(entry.Slides, block.Slide)
			}
			request.Blocks = append(request.Blocks, block)
		}
		request.Drivers = append(request.Drivers, entry)
	}
	sort.SliceStable(request.Blocks, func(i, j int) bool {
		if request.Blocks[i].Slide != request.Blocks[j].Slide {
			return request.Blocks[i].Slide < request.Blocks[j].Slide
		}
		return request.Blocks[i].Block < request.Blocks[j].Block
	})
	return request
}

// displayedCommand returns what a custom driver's command will actually
// run, with its ${NAME} variables expanded, so the prompt shows the same
// thing the driver runs rather than the literal frontmatter text. A
// variable that is not set falls back to the literal, unexpanded text:
// the prompt's job is to inform, not to fail the question over a problem
// the block itself will report when it runs.
func displayedCommand(name string, settings config.DriverConfig) string {
	command, args, err := settings.ExpandedCommand(name, os.LookupEnv)
	if err != nil {
		return strings.Join(append([]string{settings.Command}, settings.Args...), " ")
	}
	return strings.Join(append([]string{command}, args...), " ")
}

// terminalAsker asks on the terminal, before the TUI starts.
type terminalAsker struct {
	in  io.Reader
	out io.Writer
}

// askApproval prints the request and reads y, n, or s to show the code
// first. An empty answer, or the end of input, is no.
func (asker terminalAsker) askApproval(_ context.Context, request approvalRequest) (bool, error) {
	printApprovalRequest(asker.out, request)
	reader := bufio.NewReader(asker.in)
	for {
		fmt.Fprint(asker.out, "Allow this deck to run code? Type s to show the code. [y/N/s] ")
		line, readErr := reader.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true, nil
		case "", "n", "no":
			return false, nil
		case "s", "show":
			printApprovalBlocks(asker.out, request.Blocks)
		}
		if readErr != nil {
			return false, nil
		}
	}
}

// printApprovalRequest prints the deck and one line per driver.
func printApprovalRequest(out io.Writer, request approvalRequest) {
	names := make([]string, len(request.Drivers))
	for index, entry := range request.Drivers {
		names[index] = entry.Name
	}
	fmt.Fprintln(out)
	if len(request.ApprovedBefore) > 0 {
		fmt.Fprintf(out, "This deck now also wants to run %s:\n", joinWithAnd(names))
	} else {
		fmt.Fprintln(out, "This deck can run code on this computer:")
	}
	fmt.Fprintf(out, "  %s\n\n", request.Deck)
	for _, entry := range request.Drivers {
		line := fmt.Sprintf("  %-10s %s", entry.Name, describeDriverBlocks(entry))
		if entry.Command != "" {
			line += ", runs: " + entry.Command
		}
		if entry.PreviousCommand != "" {
			line += " (was: " + entry.PreviousCommand + ")"
		}
		fmt.Fprintln(out, line)
	}
	if len(request.ApprovedBefore) > 0 {
		fmt.Fprintf(out, "\n  Already approved: %s\n", strings.Join(request.ApprovedBefore, ", "))
	}
	fmt.Fprintf(out, "\nA yes is remembered for this deck, so every future run skips this question; undo it with tap approval revoke %s.\n", request.Deck)
	fmt.Fprintln(out)
}

// describeDriverBlocks says how many blocks use a driver, and on which
// slides.
func describeDriverBlocks(entry approvalDriver) string {
	if entry.Blocks == 0 {
		return "no blocks yet"
	}
	blockWord := "blocks"
	if entry.Blocks == 1 {
		blockWord = "block"
	}
	slideWord := "slides"
	if len(entry.Slides) == 1 {
		slideWord = "slide"
	}
	numbers := make([]string, len(entry.Slides))
	for index, slide := range entry.Slides {
		numbers[index] = strconv.Itoa(slide)
	}
	return fmt.Sprintf("%d %s on %s %s", entry.Blocks, blockWord, slideWord, strings.Join(numbers, ", "))
}

// printApprovalBlocks prints the code of every block in the request.
func printApprovalBlocks(out io.Writer, blocks []approvalBlock) {
	for _, block := range blocks {
		fmt.Fprintf(out, "\nSlide %d, block %d (%s):\n", block.Slide, block.Block, block.Driver)
		for _, line := range strings.Split(block.Code, "\n") {
			fmt.Fprintf(out, "    %s\n", line)
		}
	}
	fmt.Fprintln(out)
}

// joinWithAnd joins names as "a", "a and b", or "a, b and c".
func joinWithAnd(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// tuiApprovalAsker asks on the terminal while the TUI runs: it suspends
// the TUI and puts the same question startup asks.
type tuiApprovalAsker struct {
	model *tui.DevModel
}

func (asker tuiApprovalAsker) askApproval(ctx context.Context, request approvalRequest) (bool, error) {
	var approved bool
	var askErr error
	err := asker.model.AskOnTerminal(ctx, func(in io.Reader, out io.Writer) {
		approved, askErr = terminalAsker{in: in, out: out}.askApproval(ctx, request)
	})
	if err != nil {
		return false, err
	}
	return approved, askErr
}

// tuiEventWriter shows each line written to it as a TUI event, for output
// that would otherwise go to the terminal the TUI owns.
type tuiEventWriter struct {
	model *tui.DevModel
}

func (writer tuiEventWriter) Write(data []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			writer.model.SendEvent("action", line)
		}
	}
	return len(data), nil
}
