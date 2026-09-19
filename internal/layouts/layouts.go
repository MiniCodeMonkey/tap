// Package layouts holds the built-in list of layout names and their declared
// slots, and validates a transformed presentation against that list.
package layouts

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

//go:embed layouts.json
var layoutsJSON []byte

var (
	registry     map[string][]string
	layoutNames  []string
	registryOnce sync.Once
)

// loadRegistry parses the embedded layouts.json once and caches the result.
func loadRegistry() {
	registryOnce.Do(func() {
		if err := json.Unmarshal(layoutsJSON, &registry); err != nil {
			panic(fmt.Sprintf("layouts: failed to parse layouts.json: %v", err))
		}
		layoutNames = make([]string, 0, len(registry))
		for name := range registry {
			layoutNames = append(layoutNames, name)
		}
		sort.Strings(layoutNames)
	})
}

// Warning describes one layout problem on one slide.
type Warning struct {
	SlideNumber int // one-based
	Message     string
}

// Slots returns the declared slot names for a layout, and false for an unknown layout.
func Slots(layout string) ([]string, bool) {
	loadRegistry()
	slots, ok := registry[layout]
	return slots, ok
}

// Validate checks each slide's layout name and slot names against the built-in list.
func Validate(presentation *transformer.TransformedPresentation) []Warning {
	loadRegistry()

	var warnings []Warning
	for i, slide := range presentation.Slides {
		slideNumber := i + 1

		if slide.StepsInvalid {
			warnings = append(warnings, Warning{
				SlideNumber: slideNumber,
				Message:     "steps: directive ignored (must be a non-negative integer)",
			})
		}

		// A component path is exempt from the built-in layout list and may
		// use any slot name; the only thing checked here is whether it
		// resolved to a bundle at all (a missing file is reported the same
		// way an unknown layout name is). "layout: component" written
		// literally, with no path, never resolves to a component at all
		// (Component stays nil), so it falls through to the general
		// unknown-layout check below instead of being treated as one.
		if slide.Layout == "component" && slide.Component != nil {
			if slide.Component.Error != "" {
				warnings = append(warnings, Warning{
					SlideNumber: slideNumber,
					Message:     fmt.Sprintf("component %q failed to build: %s", slide.Component.Source, slide.Component.Error),
				})
			}
			continue
		}

		declaredSlots, ok := Slots(slide.Layout)
		if !ok {
			warnings = append(warnings, Warning{
				SlideNumber: slideNumber,
				Message:     fmt.Sprintf("unknown layout %q (valid layouts: %s)", slide.Layout, strings.Join(layoutNames, ", ")),
			})
			continue
		}

		validSlots := make(map[string]bool, len(declaredSlots))
		for _, name := range declaredSlots {
			validSlots[name] = true
		}

		sortedValidSlots := append([]string(nil), declaredSlots...)
		sort.Strings(sortedValidSlots)

		for _, name := range slotNames(slide) {
			if !validSlots[name] {
				warnings = append(warnings, Warning{
					SlideNumber: slideNumber,
					Message:     fmt.Sprintf("layout %q has no slot %q (valid slots: %s)", slide.Layout, name, strings.Join(sortedValidSlots, ", ")),
				})
			}
		}
	}

	return warnings
}

// slotNames returns the slide's slot names, in a stable order, preferring
// SlotOrder and falling back to the sorted keys of Slots.
func slotNames(slide transformer.TransformedSlide) []string {
	if len(slide.SlotOrder) > 0 {
		return slide.SlotOrder
	}

	names := make([]string, 0, len(slide.Slots))
	for name := range slide.Slots {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
