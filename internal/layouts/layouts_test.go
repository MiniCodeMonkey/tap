package layouts

import (
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

func TestValidate_UnknownLayout(t *testing.T) {
	pres := &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{
			{Layout: "title", SlotOrder: []string{"default"}},
			{Layout: "default", SlotOrder: []string{"default"}},
			{Layout: "nope", SlotOrder: []string{"default"}},
		},
	}

	warnings := Validate(pres)

	if len(warnings) != 1 {
		t.Fatalf("Validate() returned %d warnings, want 1: %+v", len(warnings), warnings)
	}

	warning := warnings[0]
	if warning.SlideNumber != 3 {
		t.Errorf("SlideNumber = %d, want 3", warning.SlideNumber)
	}
	if !strings.Contains(warning.Message, "nope") {
		t.Errorf("Message %q should contain %q", warning.Message, "nope")
	}
	for _, name := range []string{"big-stat", "blank", "title", "default"} {
		if !strings.Contains(warning.Message, name) {
			t.Errorf("Message %q should list valid layout name %q", warning.Message, name)
		}
	}
}

func TestValidate_UnknownSlot(t *testing.T) {
	pres := &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{
			{Layout: "title", SlotOrder: []string{"default", "caption"}},
		},
	}

	warnings := Validate(pres)

	if len(warnings) != 1 {
		t.Fatalf("Validate() returned %d warnings, want 1: %+v", len(warnings), warnings)
	}

	warning := warnings[0]
	if warning.SlideNumber != 1 {
		t.Errorf("SlideNumber = %d, want 1", warning.SlideNumber)
	}
	for _, want := range []string{"caption", "title", "default"} {
		if !strings.Contains(warning.Message, want) {
			t.Errorf("Message %q should contain %q", warning.Message, want)
		}
	}
}

func TestValidate_ValidSlideHasNoWarnings(t *testing.T) {
	pres := &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{
			{Layout: "two-column", SlotOrder: []string{"default", "left", "right"}},
		},
	}

	warnings := Validate(pres)

	if len(warnings) != 0 {
		t.Errorf("Validate() returned %d warnings for a valid slide, want 0: %+v", len(warnings), warnings)
	}
}

func TestValidate_ComponentLayoutExemptFromSlotList(t *testing.T) {
	pres := &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{
			{
				Layout:    "component",
				SlotOrder: []string{"default", "anything-goes"},
				Component: &transformer.WholeSlideComponent{Source: "./slides/RollingDeploy.jsx", URL: "/components/RollingDeploy-abc.js"},
			},
		},
	}

	warnings := Validate(pres)

	if len(warnings) != 0 {
		t.Errorf("Validate() returned %d warnings for a built component with arbitrary slot names, want 0: %+v", len(warnings), warnings)
	}
}

func TestValidate_ComponentLayoutMissingFileWarns(t *testing.T) {
	pres := &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{
			{
				Layout:    "component",
				SlotOrder: []string{"default"},
				Component: &transformer.WholeSlideComponent{Source: "./slides/Missing.jsx", Error: "read component file: no such file or directory"},
			},
		},
	}

	warnings := Validate(pres)

	if len(warnings) != 1 {
		t.Fatalf("Validate() returned %d warnings, want 1: %+v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0].Message, "./slides/Missing.jsx") {
		t.Errorf("Message %q should contain the component path", warnings[0].Message)
	}
}

func TestValidate_LayoutComponentWithNoPathWarns(t *testing.T) {
	pres := &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{
			{Layout: "component", SlotOrder: []string{"default"}},
		},
	}

	warnings := Validate(pres)

	if len(warnings) != 1 {
		t.Fatalf("Validate() returned %d warnings, want 1: %+v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0].Message, "component") {
		t.Errorf("Message %q should name the unknown layout \"component\"", warnings[0].Message)
	}
}

func TestValidate_StepsInvalidWarns(t *testing.T) {
	pres := &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{
			{Layout: "default", SlotOrder: []string{"default"}, StepsInvalid: true},
		},
	}

	warnings := Validate(pres)

	if len(warnings) != 1 {
		t.Fatalf("Validate() returned %d warnings, want 1: %+v", len(warnings), warnings)
	}
	if warnings[0].SlideNumber != 1 {
		t.Errorf("SlideNumber = %d, want 1", warnings[0].SlideNumber)
	}
	if !strings.Contains(warnings[0].Message, "steps") {
		t.Errorf("Message %q should mention the steps directive", warnings[0].Message)
	}
}

func TestValidate_SkipInvalidWarns(t *testing.T) {
	pres := &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{
			{Layout: "default", SlotOrder: []string{"default"}, SkipInvalid: true},
		},
	}

	warnings := Validate(pres)

	if len(warnings) != 1 {
		t.Fatalf("Validate() returned %d warnings, want 1: %+v", len(warnings), warnings)
	}
	if warnings[0].SlideNumber != 1 {
		t.Errorf("SlideNumber = %d, want 1", warnings[0].SlideNumber)
	}
	if !strings.Contains(warnings[0].Message, "skip") {
		t.Errorf("Message %q should mention the skip directive", warnings[0].Message)
	}
}

func TestSlots(t *testing.T) {
	slots, ok := Slots("two-column")
	if !ok {
		t.Fatal("Slots(\"two-column\") ok = false, want true")
	}
	want := []string{"default", "left", "right"}
	if len(slots) != len(want) {
		t.Fatalf("Slots(\"two-column\") = %v, want %v", slots, want)
	}
	for i, name := range want {
		if slots[i] != name {
			t.Errorf("Slots(\"two-column\")[%d] = %q, want %q", i, slots[i], name)
		}
	}

	if _, ok := Slots("nope"); ok {
		t.Error("Slots(\"nope\") ok = true, want false")
	}
}
