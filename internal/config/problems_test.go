package config

import (
	"reflect"
	"testing"
)

func TestProblems_NoneForADefaultDeck(t *testing.T) {
	if problems := DefaultConfig().Problems(); len(problems) != 0 {
		t.Fatalf("Problems() = %+v, want none", problems)
	}
}

func TestProblems_UnknownThemeIsAWarningWithASuggestion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Theme = "Keynot"
	problems := cfg.Problems()
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1: %+v", len(problems), problems)
	}
	problem := problems[0]
	if problem.Key != "theme" || problem.Severity != SeverityWarning || problem.Value != "Keynot" {
		t.Errorf("problem = %+v", problem)
	}
	if !reflect.DeepEqual(problem.Suggestions, []string{"keynote"}) {
		t.Errorf("suggestions = %v, want [keynote]", problem.Suggestions)
	}
	if len(problem.Allowed) == 0 {
		t.Error("allowed themes are missing")
	}
	if HasErrors(problems) {
		t.Error("an unknown theme must not count as an error")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, an unknown theme is not fatal", err)
	}
}

func TestProblems_ThemeWithNothingClose(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Theme = "apple-basic"
	problems := cfg.Problems()
	if len(problems) != 1 || len(problems[0].Suggestions) != 0 {
		t.Fatalf("problems = %+v, want one without suggestions", problems)
	}
}

func TestProblems_AspectRatioIsAnErrorThatNormalizes(t *testing.T) {
	for _, written := range []string{"16/9", "16x9", "16;9", " 16:9 "} {
		cfg := DefaultConfig()
		cfg.AspectRatio = written
		problems := cfg.Problems()
		if len(problems) != 1 {
			t.Fatalf("%q: got %d problems: %+v", written, len(problems), problems)
		}
		problem := problems[0]
		if problem.Key != "aspectRatio" || problem.Severity != SeverityError {
			t.Errorf("%q: problem = %+v", written, problem)
		}
		if len(problem.Suggestions) == 0 || problem.Suggestions[0] != "16:9" {
			t.Errorf("%q: suggestions = %v, want 16:9 first", written, problem.Suggestions)
		}
		if !reflect.DeepEqual(problem.Allowed, []string{"16:9", "4:3", "16:10"}) {
			t.Errorf("%q: allowed = %v", written, problem.Allowed)
		}
		if problem.Message != `The aspect ratio "`+written+`" is not one tap supports. Use 16:9, 4:3 or 16:10.` {
			t.Errorf("%q: message = %q", written, problem.Message)
		}
		if err := cfg.Validate(); err == nil {
			t.Errorf("%q: Validate accepted it", written)
		}
	}
}

func TestProblems_TransitionAndLayoutSuggestTheNearestValue(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transition = "fadee"
	cfg.PresenterLayout = "duoo"
	problems := cfg.Problems()
	if len(problems) != 2 {
		t.Fatalf("got %d problems: %+v", len(problems), problems)
	}
	if got := problems[0]; got.Key != "aspectRatio" && got.Key != "presenterLayout" {
		t.Errorf("first problem key = %q", got.Key)
	}
	byKey := map[string]Problem{}
	for _, problem := range problems {
		byKey[problem.Key] = problem
	}
	if got := byKey["transition"].Suggestions; len(got) == 0 || got[0] != "fade" {
		t.Errorf("transition suggestions = %v", got)
	}
	if got := byKey["presenterLayout"].Suggestions; len(got) == 0 || got[0] != "duo" {
		t.Errorf("presenterLayout suggestions = %v", got)
	}
}

func TestProblems_ErrorsMatchValidate(t *testing.T) {
	cases := map[string]func(*Config){
		"themeColors key":   func(c *Config) { c.ThemeColors = map[string]string{"bg": "#fff"} },
		"warnAfter":         func(c *Config) { c.Recording.WarnAfter = "soon" },
		"stopAfter off":     func(c *Config) { c.Recording.StopAfter = "off" },
		"negative display":  func(c *Config) { c.Recording.Display = -1 },
		"valid transition":  func(c *Config) { c.Transition = "zoom" },
		"empty everything":  func(c *Config) { c.AspectRatio, c.Transition, c.Theme = "", "", "" },
		"unknown and wrong": func(c *Config) { c.Theme, c.AspectRatio = "nope", "5:4" },
	}
	for name, change := range cases {
		cfg := DefaultConfig()
		change(cfg)
		wantError := HasErrors(cfg.Problems())
		if gotError := cfg.Validate() != nil; gotError != wantError {
			t.Errorf("%s: Validate failed = %v, HasErrors = %v", name, gotError, wantError)
		}
	}
}

func TestNearestValues(t *testing.T) {
	allowed := []string{"16:9", "4:3", "16:10"}
	cases := map[string][]string{
		"16/9":   {"16:9"},
		"4x3":    {"4:3"},
		"16:11":  {"16:10"},
		"banana": {},
	}
	for value, want := range cases {
		got := NearestValues(value, allowed)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("NearestValues(%q) = %v, want %v", value, got, want)
		}
	}
}
