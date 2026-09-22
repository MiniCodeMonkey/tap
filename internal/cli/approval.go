package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
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
// answer. The terminal asks on standard input. tap dev --app sends the
// request as a question event and reads the answer from standard input.
// Never the slide page, where page script could answer.
type approvalAsker interface {
	askApproval(request approvalRequest) (bool, error)
}

// approvalInput is everything the approval check reads.
type approvalInput struct {
	Now          func() time.Time
	Config       *config.Config
	Presentation *transformer.TransformedPresentation
	Asker        approvalAsker
	Out          io.Writer
	SettingsPath string
	// Deck is the deck file: an absolute or relative path, possibly
	// through a symlink. liveCodeApproval resolves it before it compares
	// or stores anything, so it never has to already be canonical.
	Deck string
	// AllowCode is --allow-code: live code runs for this run, and nothing
	// is asked or stored.
	AllowCode bool
	// Interactive is true when tap may ask: a terminal on standard input
	// and no --headless.
	Interactive bool
}

// liveCodeApproval decides which drivers this run of tap dev or tap
// present may run. A deck needs approval when a live code block uses a
// declared driver. An approved deck runs. Otherwise tap asks, when it may,
// and a yes is stored in the user settings. A no, or a run that may not
// ask, stores nothing and keeps the drivers approved before.
//
// Approval is matched by usersettings.DeckKey, which usersettings.
// ResolveDeck alone can produce: it resolves Deck to its absolute path,
// with symlinks resolved and, on a case-insensitive filesystem, its
// on-disk case restored. Without this, the same deck reached through a
// symlink, a relative path, a path with a ".." segment, or a different
// spelling of its name would not match its own approval, and worse, an
// unrelated file could be made to match one by spelling alone. Because
// Approved, Approve and ApprovalFor take only a DeckKey, this function
// cannot compare or store an unresolved path even by accident. A deck
// that cannot be resolved, most often because it no longer exists, fails
// closed: liveCodeApproval never asks and never stores an approval for
// it.
func liveCodeApproval(input approvalInput) (server.LiveCodePolicy, error) {
	blocks := runnableBlocks(input.Presentation)
	if len(blocks) == 0 {
		return server.LiveCodePolicy{}, nil
	}
	if input.AllowCode {
		fmt.Fprintln(input.Out, "Live code is on for this run (--allow-code). No approval is saved.")
		return server.LiveCodePolicy{AllowAll: true}, nil
	}

	deckKey, err := usersettings.ResolveDeck(input.Deck)
	if err != nil {
		fmt.Fprintf(input.Out, "Live code is off: %s could not be resolved: %v\n", input.Deck, err)
		return server.LiveCodePolicy{}, nil
	}
	deck := deckKey.String()

	declared := input.Config.DeclaredDrivers()
	settings, err := usersettings.Load(input.SettingsPath)
	if err != nil {
		// A malformed settings file must not stop the talk. A yes below
		// overwrites it with a well-formed one.
		fmt.Fprintf(input.Out, "Ignoring %s, it could not be read: %v\n", input.SettingsPath, err)
		settings = usersettings.Settings{}
	}
	if settings.Approved(deckKey, declared) {
		return server.LiveCodePolicy{Drivers: declared}, nil
	}

	previous, _ := settings.ApprovalFor(deckKey)
	var approvedBefore, wanted []string
	for _, name := range declared {
		if slices.Contains(previous.Drivers, name) {
			approvedBefore = append(approvedBefore, name)
		} else {
			wanted = append(wanted, name)
		}
	}

	if !input.Interactive {
		fmt.Fprintf(input.Out, "Live code is off: this deck is not approved to run %s. Run tap dev or tap present in a terminal to approve it, or pass --allow-code for this run.\n", joinWithAnd(wanted))
		return server.LiveCodePolicy{Drivers: approvedBefore}, nil
	}

	approved, askErr := input.Asker.askApproval(newApprovalRequest(deck, input.Config, blocks, wanted, approvedBefore))
	if askErr != nil {
		fmt.Fprintf(input.Out, "Live code is off for %s in this run: %v\n", joinWithAnd(wanted), askErr)
		return server.LiveCodePolicy{Drivers: approvedBefore}, nil
	}
	if !approved {
		fmt.Fprintf(input.Out, "Live code is off for %s in this run. tap asks again next time.\n", joinWithAnd(wanted))
		return server.LiveCodePolicy{Drivers: approvedBefore}, nil
	}

	settings.Approve(deckKey, declared, input.Now())
	if err := usersettings.Save(input.SettingsPath, settings); err != nil {
		return server.LiveCodePolicy{}, fmt.Errorf("saving the live code approval: %w", err)
	}
	return server.LiveCodePolicy{Drivers: declared}, nil
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
func (asker terminalAsker) askApproval(request approvalRequest) (bool, error) {
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
