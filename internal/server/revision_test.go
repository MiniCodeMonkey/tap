package server

import (
	"fmt"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// TestComputeRevisionChangesWithContent verifies ComputeRevision is stable
// for identical input, and changes when the presentation content or the set
// of component bundle names changes.
func TestComputeRevisionChangesWithContent(t *testing.T) {
	pres := &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{{HTML: "<p>Slide 1</p>", Index: 0}},
	}
	bundles := map[string]ComponentBundleFile{
		"Chart.js": {Content: []byte("chart"), ContentType: "application/javascript"},
	}

	first := ComputeRevision(pres, bundles)
	second := ComputeRevision(pres, bundles)
	if first != second {
		t.Errorf("ComputeRevision() is not stable for identical input: %q != %q", first, second)
	}
	if first == "" {
		t.Error("ComputeRevision() = empty string, want a non-empty hash")
	}

	changedPres := &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{{HTML: "<p>Slide 1, edited</p>", Index: 0}},
	}
	if changed := ComputeRevision(changedPres, bundles); changed == first {
		t.Error("ComputeRevision() did not change when the presentation content changed")
	}

	changedBundles := map[string]ComponentBundleFile{
		"Chart.js":  {Content: []byte("chart"), ContentType: "application/javascript"},
		"Table.jsx": {Content: []byte("table"), ContentType: "application/javascript"},
	}
	if changed := ComputeRevision(pres, changedBundles); changed == first {
		t.Error("ComputeRevision() did not change when the component bundle names changed")
	}
}

func presentationWithHashes(hashes ...string) *transformer.TransformedPresentation {
	presentation := &transformer.TransformedPresentation{}
	for index, hash := range hashes {
		presentation.Slides = append(presentation.Slides, transformer.TransformedSlide{Index: index, Hash: hash})
	}
	return presentation
}

func TestChangedSlides(t *testing.T) {
	tests := []struct {
		name     string
		previous *transformer.TransformedPresentation
		next     *transformer.TransformedPresentation
		want     []int
	}{
		{"nothing changed", presentationWithHashes("a", "b", "c"), presentationWithHashes("a", "b", "c"), []int{}},
		{"the second slide changed", presentationWithHashes("a", "b", "c"), presentationWithHashes("a", "x", "c"), []int{2}},
		{"a slide was added at the end", presentationWithHashes("a", "b"), presentationWithHashes("a", "b", "c"), []int{3}},
		{"the last slide was removed", presentationWithHashes("a", "b", "c"), presentationWithHashes("a", "b"), []int{}},
		{"a slide was inserted first", presentationWithHashes("a", "b"), presentationWithHashes("z", "a", "b"), []int{1, 2, 3}},
		{"no previous deck", nil, presentationWithHashes("a", "b"), []int{1, 2}},
		{"no next deck", presentationWithHashes("a"), nil, []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChangedSlides(tt.previous, tt.next)
			if got == nil {
				t.Fatal("ChangedSlides() = nil, want an empty slice")
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("ChangedSlides() = %v, want %v", got, tt.want)
			}
		})
	}
}
