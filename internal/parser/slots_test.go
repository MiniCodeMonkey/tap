package parser

import (
	"strings"
	"testing"
)

func TestSplitSlots(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNames []string
		wantError string
	}{
		{"no markers", "# Title\n\nText", []string{"default"}, ""},
		{"default and named", "# Big\n\n::caption\nSmall\n\n::figure\n![](a.png)", []string{"default", "caption", "figure"}, ""},
		{"marker first", "::caption\nOnly caption", []string{"caption"}, ""},
		{"marker inside fence is content", "```\n::caption\n```", []string{"default"}, ""},
		{"empty slot dropped", "Text\n\n::caption\n\n::figure\nx", []string{"default", "figure"}, ""},
		{"duplicate", "::caption\na\n\n::caption\nb", nil, `duplicate slot "caption" on line 4`},
		{"uppercase is not a marker", "::Caption\ntext", []string{"default"}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sections, err := splitSlots(test.input)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to contain %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var names []string
			for _, section := range sections {
				names = append(names, section.Name)
			}
			if strings.Join(names, ",") != strings.Join(test.wantNames, ",") {
				t.Errorf("names = %v, want %v", names, test.wantNames)
			}
		})
	}
}

func TestParseSlotsAndFragments(t *testing.T) {
	input := "# Title\n\n<!-- pause -->\n\nSecond\n\n::caption\nAlways visible\n\n<!-- pause -->\n\nThird"
	presentation, err := New().Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	slide := presentation.Slides[0]
	if slide.FragmentCount != 2 {
		t.Errorf("FragmentCount = %d, want 2", slide.FragmentCount)
	}
	if !strings.Contains(slide.Slots["default"], `data-fragment-index="0"`) {
		t.Errorf("default slot lacks fragment 0: %s", slide.Slots["default"])
	}
	if !strings.Contains(slide.Slots["caption"], `data-fragment-index="1"`) {
		t.Errorf("caption slot lacks fragment 1: %s", slide.Slots["caption"])
	}
	if strings.Index(slide.Slots["caption"], "Always visible") > strings.Index(slide.Slots["caption"], "data-fragment-index") {
		t.Errorf("content before the first pause of a slot must sit outside the fragment wrapper")
	}
	if strings.Join(slide.SlotOrder, ",") != "default,caption" {
		t.Errorf("SlotOrder = %v", slide.SlotOrder)
	}
}

func TestParseDuplicateSlotNamesSlide(t *testing.T) {
	input := "# One\n\n---\n\n::a\nx\n\n::a\ny"
	_, err := New().Parse([]byte(input))
	if err == nil || !strings.Contains(err.Error(), "slide 2") {
		t.Fatalf("error = %v, want it to name slide 2", err)
	}
}
