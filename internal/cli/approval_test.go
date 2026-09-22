package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// fakeAsker answers every approval request with answer, and keeps them, or
// with err when it is set, to stand in for the terminal or standard input
// failing outright rather than the person answering no.
type fakeAsker struct {
	requests []approvalRequest
	answer   bool
	err      error
}

func (asker *fakeAsker) askApproval(request approvalRequest) (bool, error) {
	asker.requests = append(asker.requests, request)
	return asker.answer, asker.err
}

// resolveKey resolves path with usersettings.ResolveDeck and fails the
// test if it cannot, the same way liveCodeApproval itself would refuse an
// unresolvable deck.
func resolveKey(t *testing.T, path string) usersettings.DeckKey {
	t.Helper()
	key, err := usersettings.ResolveDeck(path)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// approvalFixture returns a deck with shell blocks on slides 1 and 2 and a
// python block on slide 3, declaring the drivers in declared. python is a
// custom driver that runs "python3 -c".
func approvalFixture(declared ...string) (*config.Config, *transformer.TransformedPresentation) {
	cfg := config.DefaultConfig()
	for _, name := range declared {
		settings := config.DriverConfig{}
		if name == "python" {
			settings = config.DriverConfig{Command: "python3", Args: []string{"-c"}}
		}
		cfg.Drivers[name] = settings
	}
	parsed := &parser.Presentation{Slides: []parser.Slide{
		{Index: 0, CodeBlocks: []parser.CodeBlock{{Language: "bash", Code: "echo one", Meta: parser.CodeBlockMeta{Driver: "shell"}}}},
		{Index: 1, CodeBlocks: []parser.CodeBlock{{Language: "bash", Code: "echo two\necho three", Meta: parser.CodeBlockMeta{Driver: "shell"}}}},
		{Index: 2, CodeBlocks: []parser.CodeBlock{{Language: "python", Code: "print(3)", Meta: parser.CodeBlockMeta{Driver: "python"}}}},
	}}
	return cfg, transformer.New(cfg).Transform(parsed)
}

var approvalNow = time.Date(2026, 9, 22, 19, 32, 0, 0, time.UTC)

// makeDeckFile creates a real file named talk.md in a fresh temporary
// directory and returns the resolved, canonical path liveCodeApproval
// keys its approval by. Approval matching resolves the deck to its
// absolute path with symlinks resolved, so a fake path such as
// "/talks/talk.md" that does not exist on disk can never be resolved;
// every test that exercises the approval flow (rather than only the
// terminal prompt, which never touches the filesystem) needs a deck that
// is actually there.
func makeDeckFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "talk.md")
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	return resolveKey(t, path).String()
}

func approvalInputFor(t *testing.T, cfg *config.Config, pres *transformer.TransformedPresentation, asker approvalAsker) (approvalInput, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	return approvalInput{
		Now:          func() time.Time { return approvalNow },
		Config:       cfg,
		Presentation: pres,
		Asker:        asker,
		Out:          out,
		SettingsPath: filepath.Join(t.TempDir(), "settings.yaml"),
		Deck:         makeDeckFile(t),
		Interactive:  true,
	}, out
}

func TestApprovalAsksOnceAndRemembersTheAnswer(t *testing.T) {
	cfg, pres := approvalFixture("python", "shell")
	asker := &fakeAsker{answer: true}
	input, _ := approvalInputFor(t, cfg, pres, asker)

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(policy.Drivers, ",") != "python,shell" || policy.AllowAll {
		t.Errorf("policy = %+v, want python and shell", policy)
	}
	settings, _ := usersettings.Load(input.SettingsPath)
	if !settings.Approved(resolveKey(t, input.Deck), []string{"python", "shell"}) {
		t.Errorf("the approval was not saved: %+v", settings)
	}

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	if len(asker.requests) != 1 {
		t.Errorf("asked %d times, want once", len(asker.requests))
	}
}

func TestApprovalRequestListsDriversBlocksAndCommands(t *testing.T) {
	cfg, pres := approvalFixture("python", "shell")
	asker := &fakeAsker{}
	input, _ := approvalInputFor(t, cfg, pres, asker)
	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}

	request := asker.requests[0]
	if request.Deck != input.Deck || len(request.ApprovedBefore) != 0 {
		t.Errorf("request = %+v", request)
	}
	if len(request.Drivers) != 2 {
		t.Fatalf("drivers = %+v, want python and shell", request.Drivers)
	}
	python, shell := request.Drivers[0], request.Drivers[1]
	if python.Name != "python" || python.Command != "python3 -c" || python.Blocks != 1 {
		t.Errorf("python = %+v", python)
	}
	if shell.Name != "shell" || shell.Command != "" || shell.Blocks != 2 || len(shell.Slides) != 2 {
		t.Errorf("shell = %+v, want 2 blocks on 2 slides and no command", shell)
	}
	if len(request.Blocks) != 3 || request.Blocks[0].Slide != 1 || request.Blocks[2].Slide != 3 {
		t.Errorf("blocks = %+v, want three in slide order", request.Blocks)
	}
}

func TestApprovalDeclineStoresNothingAndAsksAgain(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: false}
	input, out := approvalInputFor(t, cfg, pres, asker)

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Drivers) != 0 || policy.AllowAll {
		t.Errorf("policy = %+v, want nothing allowed", policy)
	}
	if !strings.Contains(out.String(), "Live code is off for shell in this run. tap asks again next time.") {
		t.Errorf("output = %q", out.String())
	}
	if _, err := os.Stat(input.SettingsPath); !os.IsNotExist(err) {
		t.Errorf("a no saved the settings file (stat error %v)", err)
	}

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	if len(asker.requests) != 2 {
		t.Errorf("asked %d times, want again on the next start", len(asker.requests))
	}
}

func TestApprovalAsksOnlyAboutANewDriver(t *testing.T) {
	cfg, pres := approvalFixture("python", "shell")
	asker := &fakeAsker{answer: false}
	input, _ := approvalInputFor(t, cfg, pres, asker)
	var settings usersettings.Settings
	settings.Approve(resolveKey(t, input.Deck), []string{"shell"}, approvalNow)
	if err := usersettings.Save(input.SettingsPath, settings); err != nil {
		t.Fatal(err)
	}

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	request := asker.requests[0]
	if len(request.Drivers) != 1 || request.Drivers[0].Name != "python" {
		t.Errorf("drivers = %+v, want only the new python driver", request.Drivers)
	}
	if strings.Join(request.ApprovedBefore, ",") != "shell" {
		t.Errorf("approvedBefore = %v, want shell", request.ApprovedBefore)
	}
	if len(request.Blocks) != 1 || request.Blocks[0].Driver != "python" {
		t.Errorf("blocks = %+v, want only the python block", request.Blocks)
	}
	if strings.Join(policy.Drivers, ",") != "shell" {
		t.Errorf("policy = %+v, want shell still allowed after a no to python", policy)
	}

	asker.answer = true
	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	saved, _ := usersettings.Load(input.SettingsPath)
	approval, _ := saved.ApprovalFor(resolveKey(t, input.Deck))
	if strings.Join(approval.Drivers, ",") != "python,shell" {
		t.Errorf("approved drivers = %v, want python and shell", approval.Drivers)
	}
}

// TestApprovalAsksAgainAfterARealMove approves a deck, moves the actual
// file on disk to a new path (rather than stubbing a stored path that
// could never itself resolve), and confirms the deck at its new location
// is asked about again rather than inheriting the old approval.
func TestApprovalAsksAgainAfterARealMove(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.md")
	if err := os.WriteFile(oldPath, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, _ := approvalInputFor(t, cfg, pres, asker)
	input.Deck = oldPath

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	if len(asker.requests) != 1 {
		t.Fatalf("asked %d times approving the deck at its old path, want once", len(asker.requests))
	}

	newPath := filepath.Join(dir, "new.md")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	input.Deck = newPath

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	if len(asker.requests) != 2 {
		t.Errorf("asked %d times after the deck moved, want a second ask for the deck at its new path", len(asker.requests))
	}
}

func TestApprovalAllowCodeAsksNothingAndStoresNothing(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{}
	input, out := approvalInputFor(t, cfg, pres, asker)
	input.AllowCode = true
	input.Interactive = false

	policy, err := liveCodeApproval(input)
	if err != nil || !policy.AllowAll {
		t.Errorf("policy = %+v, %v; want AllowAll", policy, err)
	}
	if len(asker.requests) != 0 {
		t.Error("--allow-code asked")
	}
	if _, err := os.Stat(input.SettingsPath); !os.IsNotExist(err) {
		t.Error("--allow-code saved the settings file")
	}
	if !strings.Contains(out.String(), "--allow-code") {
		t.Errorf("output = %q, want a note that --allow-code is on", out.String())
	}
}

func TestApprovalNeverAsksWithoutATerminal(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, out := approvalInputFor(t, cfg, pres, asker)
	input.Interactive = false

	policy, err := liveCodeApproval(input)
	if err != nil || len(policy.Drivers) != 0 {
		t.Errorf("policy = %+v, %v; want nothing allowed", policy, err)
	}
	if len(asker.requests) != 0 {
		t.Error("asked without a terminal")
	}
	if !strings.Contains(out.String(), "Live code is off: this deck is not approved to run shell.") {
		t.Errorf("output = %q", out.String())
	}

	var settings usersettings.Settings
	settings.Approve(resolveKey(t, input.Deck), []string{"shell"}, approvalNow)
	if err := usersettings.Save(input.SettingsPath, settings); err != nil {
		t.Fatal(err)
	}
	policy, err = liveCodeApproval(input)
	if err != nil || strings.Join(policy.Drivers, ",") != "shell" {
		t.Errorf("an approved deck without a terminal: policy = %+v, %v; want shell", policy, err)
	}
}

func TestApprovalIsNotNeededWithoutRunnableCode(t *testing.T) {
	asker := &fakeAsker{answer: true}

	cfg := config.DefaultConfig()
	cfg.Drivers["shell"] = config.DriverConfig{}
	noLiveCode := transformer.New(cfg).Transform(&parser.Presentation{Slides: []parser.Slide{
		{Index: 0, CodeBlocks: []parser.CodeBlock{{Language: "go", Code: "package main"}}},
	}})
	input, out := approvalInputFor(t, cfg, noLiveCode, asker)
	if policy, err := liveCodeApproval(input); err != nil || len(policy.Drivers) != 0 {
		t.Errorf("policy = %+v, %v", policy, err)
	}

	undeclaredCfg, undeclaredOnly := approvalFixture()
	input, _ = approvalInputFor(t, undeclaredCfg, undeclaredOnly, asker)
	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}

	if len(asker.requests) != 0 || out.Len() != 0 {
		t.Errorf("asked %d times, printed %q; want nothing", len(asker.requests), out.String())
	}
}

func TestApprovalIgnoresAnUnreadableSettingsFile(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, out := approvalInputFor(t, cfg, pres, asker)
	if err := os.WriteFile(input.SettingsPath, []byte("approvals: [not: valid"), 0o600); err != nil {
		t.Fatal(err)
	}

	policy, err := liveCodeApproval(input)
	if err != nil || strings.Join(policy.Drivers, ",") != "shell" {
		t.Errorf("policy = %+v, %v", policy, err)
	}
	if !strings.Contains(out.String(), "Ignoring") {
		t.Errorf("output = %q, want the unreadable file named", out.String())
	}
}

// The tests below exercise usersettings.ResolveDeck and liveCodeApproval's
// use of it: this is the gate the whole plan exists to build, and a gate
// that can be fooled by spelling is not a gate. Approval must be keyed by
// the resolved deck so the same deck reached through a symlink, a
// relative path, a path with "..", or a different spelling of an existing
// name always matches its own approval, and a deck that cannot be
// resolved at all is never approved. usersettings itself carries the
// exhaustive resolution tests (symlinked directory component, the
// fail-open case-sensitive direction, and so on); the tests here confirm
// liveCodeApproval actually uses it end to end.

func TestApprovalResolvesASymlinkToTheRealDeck(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.md")
	if err := os.WriteFile(real, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	resolvedReal := resolveKey(t, real)

	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, _ := approvalInputFor(t, cfg, pres, asker)
	input.Deck = link

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	settings, _ := usersettings.Load(input.SettingsPath)
	if !settings.Approved(resolvedReal, []string{"shell"}) {
		t.Errorf("the approval was not stored under the symlink's real path: %+v", settings)
	}

	// Asking again through the same symlink must not ask twice.
	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	if len(asker.requests) != 1 {
		t.Errorf("asked %d times through the symlink, want once", len(asker.requests))
	}

	// The deck's real path, reached directly, matches the same approval.
	input.Deck = real
	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	if len(asker.requests) != 1 {
		t.Errorf("asked %d times for the deck's real path after approving it through a symlink, want once", len(asker.requests))
	}
}

func TestApprovalResolvesARelativePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "talk.md")
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved := resolveKey(t, path)

	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, _ := approvalInputFor(t, cfg, pres, asker)
	t.Chdir(dir)
	input.Deck = "talk.md"

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	settings, _ := usersettings.Load(input.SettingsPath)
	if !settings.Approved(resolved, []string{"shell"}) {
		t.Errorf("the approval was not stored under the resolved absolute path: %+v", settings)
	}
}

func TestApprovalResolvesAPathWithDotDot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "talk.md")
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved := resolveKey(t, path)

	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, _ := approvalInputFor(t, cfg, pres, asker)
	input.Deck = filepath.Join(dir, "sub", "..", "talk.md")

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	settings, _ := usersettings.Load(input.SettingsPath)
	if !settings.Approved(resolved, []string{"shell"}) {
		t.Errorf("the approval was not stored under the cleaned path: %+v", settings)
	}
}

func TestApprovalResolvesADifferentlyCasedSpelling(t *testing.T) {
	dir := t.TempDir()
	upper := filepath.Join(dir, "Talk.md")
	if err := os.WriteFile(upper, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	lower := filepath.Join(dir, "talk.md")
	if _, err := os.Stat(lower); err != nil {
		t.Skip("this filesystem is case sensitive, so a different spelling names a different file")
	}

	resolvedUpper := resolveKey(t, upper)
	resolvedLower := resolveKey(t, lower)
	if resolvedUpper != resolvedLower {
		t.Fatalf("ResolveDeck(%q) = %v, ResolveDeck(%q) = %v, want the same key", upper, resolvedUpper, lower, resolvedLower)
	}

	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, _ := approvalInputFor(t, cfg, pres, asker)
	input.Deck = upper

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	input.Deck = lower
	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	if len(asker.requests) != 1 {
		t.Errorf("asked %d times for two spellings of the same deck, want once", len(asker.requests))
	}
}

func TestApprovalFailsClosedWhenTheDeckCannotBeResolved(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, out := approvalInputFor(t, cfg, pres, asker)
	input.Deck = filepath.Join(t.TempDir(), "gone.md")

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Drivers) != 0 || policy.AllowAll {
		t.Errorf("policy = %+v, want nothing allowed for a deck that cannot be resolved", policy)
	}
	if len(asker.requests) != 0 {
		t.Error("asked about a deck that could not be resolved")
	}
	if !strings.Contains(out.String(), "could not be resolved") {
		t.Errorf("output = %q, want the resolution failure named", out.String())
	}
	if _, statErr := os.Stat(input.SettingsPath); !os.IsNotExist(statErr) {
		t.Error("a deck that could not be resolved saved the settings file")
	}
}

func terminalRequest() approvalRequest {
	return approvalRequest{
		Deck: "/talks/talk.md",
		Drivers: []approvalDriver{
			{Name: "python", Command: "python3 -c", Slides: []int{3}, Blocks: 1},
			{Name: "shell", Slides: []int{1, 2}, Blocks: 2},
		},
		Blocks: []approvalBlock{
			{Driver: "shell", Code: "echo one", Slide: 1, Block: 1},
			{Driver: "shell", Code: "echo two\necho three", Slide: 2, Block: 1},
			{Driver: "python", Code: "print(3)", Slide: 3, Block: 1},
		},
	}
}

func TestTerminalAskerListsTheDriversAndShowsTheCode(t *testing.T) {
	out := &bytes.Buffer{}
	approved, err := terminalAsker{in: strings.NewReader("s\ny\n"), out: out}.askApproval(terminalRequest())
	if err != nil || !approved {
		t.Fatalf("approved = %v, %v; want true", approved, err)
	}
	text := out.String()
	for _, want := range []string{
		"This deck can run code on this computer:\n  /talks/talk.md\n",
		"  python     1 block on slide 3, runs: python3 -c\n",
		"  shell      2 blocks on slides 1, 2\n",
		"Slide 2, block 1 (shell):\n    echo two\n    echo three\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "[y/N/s]") != 2 {
		t.Errorf("want the question again after showing the code:\n%s", text)
	}
}

func TestTerminalAskerDefaultsToNo(t *testing.T) {
	for _, typed := range []string{"\n", "", "n\n", "no\n", "maybe\n"} {
		approved, err := terminalAsker{in: strings.NewReader(typed), out: &bytes.Buffer{}}.askApproval(terminalRequest())
		if err != nil || approved {
			t.Errorf("typed %q: approved = %v, %v; want false", typed, approved, err)
		}
	}
}

func TestTerminalAskerAsksAgainAfterAnUnclearAnswer(t *testing.T) {
	out := &bytes.Buffer{}
	approved, _ := terminalAsker{in: strings.NewReader("maybe\ny\n"), out: out}.askApproval(terminalRequest())
	if !approved || strings.Count(out.String(), "[y/N/s]") != 2 {
		t.Errorf("approved = %v, output:\n%s", approved, out.String())
	}
}

func TestTerminalAskerNamesOnlyTheNewDriver(t *testing.T) {
	request := approvalRequest{
		Deck:           "/talks/talk.md",
		Drivers:        []approvalDriver{{Name: "shell", Slides: []int{6}, Blocks: 1}},
		ApprovedBefore: []string{"sqlite"},
		Blocks:         []approvalBlock{{Driver: "shell", Code: "ls", Slide: 6, Block: 1}},
	}
	out := &bytes.Buffer{}
	_, _ = terminalAsker{in: strings.NewReader("n\n"), out: out}.askApproval(request)
	if !strings.Contains(out.String(), "This deck now also wants to run shell:\n") {
		t.Errorf("output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Already approved: sqlite") {
		t.Errorf("output does not name the drivers approved before:\n%s", out.String())
	}
}
