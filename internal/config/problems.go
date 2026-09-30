package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// Problem severities. An error stops tap from rendering the deck; a
// warning renders it with a fallback.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Problem is one thing wrong with a deck's settings, described for a
// person and for a tool that offers a fix.
type Problem struct {
	// Key is the frontmatter key at fault, with dots for nested keys.
	Key string `json:"key"`
	// Value is the value as the deck writes it.
	Value string `json:"value"`
	// Message is a plain sentence naming the problem and what tap does.
	Message string `json:"message"`
	// Severity is SeverityError or SeverityWarning.
	Severity string `json:"severity"`
	// Suggestions are values that would fix the problem, best first. They
	// come from the key's allowed values, never from a list of guesses. A
	// problem with the name of a key, not its value, has none.
	Suggestions []string `json:"suggestions"`
	// Allowed lists every value the key accepts, in schema order. It is
	// empty for a key that takes free text.
	Allowed []string `json:"allowed"`
}

// Problems lists what is wrong with c, without changing it: Validate
// normalizes an unknown theme and Problems must see the name as written,
// so call it first. An error problem is exactly what makes Validate fail.
func (c *Config) Problems() []Problem {
	var problems []Problem
	if c.Theme != "" && !themes.IsValid(c.Theme) {
		problems = append(problems, Problem{
			Key:         "theme",
			Value:       c.Theme,
			Message:     fmt.Sprintf("%q is not a tap theme. The preview uses Base for now.", c.Theme),
			Severity:    SeverityWarning,
			Suggestions: NearestValues(c.Theme, themes.Slugs()),
			Allowed:     themes.Slugs(),
		})
	}
	for _, check := range []struct {
		key     string
		label   string
		value   string
		allowed []string
	}{
		{"aspectRatio", "aspect ratio", c.AspectRatio, aspectRatioValues},
		{"presenterLayout", "presenter layout", c.PresenterLayout, presenterLayoutValues},
		{"transition", "transition", c.Transition, transitionValues},
	} {
		if check.value == "" || valueSet(check.allowed)[check.value] {
			continue
		}
		problems = append(problems, Problem{
			Key:         check.key,
			Value:       check.value,
			Message:     fmt.Sprintf("The %s %q is not one tap supports. Use %s.", check.label, check.value, joinAlternatives(check.allowed)),
			Severity:    SeverityError,
			Suggestions: NearestValues(check.value, check.allowed),
			Allowed:     check.allowed,
		})
	}
	colorKeys := make([]string, 0, len(themeColorKeys))
	for _, key := range themeColorKeys {
		colorKeys = append(colorKeys, key.name)
	}
	names := make([]string, 0, len(c.ThemeColors))
	for name := range c.ThemeColors {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if validThemeColorKeys[name] {
			continue
		}
		problems = append(problems, Problem{
			Key:      "themeColors." + name,
			Value:    c.ThemeColors[name],
			Message:  fmt.Sprintf("%q is not a themeColors key. Use %s.", name, joinAlternatives(colorKeys)),
			Severity: SeverityError,
		})
	}
	for _, field := range []struct {
		key   string
		value string
	}{
		{"recording.warnAfter", c.Recording.WarnAfter},
		{"recording.stopAfter", c.Recording.StopAfter},
	} {
		value := strings.TrimSpace(field.value)
		if value == "" || (field.key == "recording.stopAfter" && strings.EqualFold(value, "off")) {
			continue
		}
		if parsed, err := time.ParseDuration(value); err != nil || parsed <= 0 {
			problems = append(problems, Problem{
				Key:      field.key,
				Value:    field.value,
				Message:  fmt.Sprintf("%q is not a duration. Use a positive one such as 90m or 3h.", field.value),
				Severity: SeverityError,
			})
		}
	}
	if c.Recording.Display < 0 {
		problems = append(problems, Problem{
			Key:      "recording.display",
			Value:    fmt.Sprint(c.Recording.Display),
			Message:  fmt.Sprintf("The display number %d is below 0. Use 0 or greater.", c.Recording.Display),
			Severity: SeverityError,
		})
	}
	for index := range problems {
		if problems[index].Suggestions == nil {
			problems[index].Suggestions = []string{}
		}
		if problems[index].Allowed == nil {
			problems[index].Allowed = []string{}
		}
	}
	return problems
}

// HasErrors reports whether any problem stops the deck from rendering.
func HasErrors(problems []Problem) bool {
	for _, problem := range problems {
		if problem.Severity == SeverityError {
			return true
		}
	}
	return false
}

// joinAlternatives writes values as "a, b or c".
func joinAlternatives(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	}
	return strings.Join(values[:len(values)-1], ", ") + " or " + values[len(values)-1]
}

// maximumSuggestions is how many values a problem offers.
const maximumSuggestions = 3

// NearestValues returns the allowed values closest to value, best first,
// at most maximumSuggestions. A value that matches one after case and
// separator differences ("16/9" for "16:9", "Keynote" for "keynote")
// suggests only that one. Otherwise a value is close when its edit
// distance is at most a third of the longer word, and never more than 3.
func NearestValues(value string, allowed []string) []string {
	normalized := normalizeForMatch(value)
	for _, candidate := range allowed {
		if normalizeForMatch(candidate) == normalized && candidate != value {
			return []string{candidate}
		}
	}
	type scored struct {
		value    string
		distance int
	}
	var close []scored
	for _, candidate := range allowed {
		longest := max(len(normalized), len(candidate))
		limit := min(3, max(1, longest/3))
		if distance := editDistance(normalized, normalizeForMatch(candidate)); distance <= limit {
			close = append(close, scored{candidate, distance})
		}
	}
	sort.SliceStable(close, func(a, b int) bool { return close[a].distance < close[b].distance })
	suggestions := []string{}
	for _, entry := range close {
		if len(suggestions) == maximumSuggestions {
			break
		}
		suggestions = append(suggestions, entry.value)
	}
	return suggestions
}

// normalizeForMatch lowercases value and turns every separator a person
// might type between the parts of a value into a colon.
func normalizeForMatch(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if ratioWithX.MatchString(value) {
		value = strings.Replace(value, "x", ":", 1)
	}
	return strings.NewReplacer("/", ":", ";", ":", "_", "-", " ", "-").Replace(value)
}

// ratioWithX matches a ratio written with an x, such as 16x9.
var ratioWithX = regexp.MustCompile(`^\d+x\d+$`)

// editDistance is the Levenshtein distance between two strings, counted in bytes.
func editDistance(first, second string) int {
	previous := make([]int, len(second)+1)
	for column := range previous {
		previous[column] = column
	}
	for row := 1; row <= len(first); row++ {
		current := make([]int, len(second)+1)
		current[0] = row
		for column := 1; column <= len(second); column++ {
			cost := 1
			if first[row-1] == second[column-1] {
				cost = 0
			}
			current[column] = min(previous[column]+1, current[column-1]+1, previous[column-1]+cost)
		}
		previous = current
	}
	return previous[len(second)]
}
