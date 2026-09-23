package server

import (
	"fmt"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
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

// TestComputeRevisionIsSaltedPerProcess verifies the revision is no longer
// a pure function of the deck's content: a deck carrying a literal secret
// produces a different revision under a different process salt, even
// though the content hashed is byte-for-byte identical. This is what
// stops someone who knows the rest of a deck byte for byte from testing a
// guessed secret against the revision offline.
func TestComputeRevisionIsSaltedPerProcess(t *testing.T) {
	pres := &transformer.TransformedPresentation{
		Config: config.Config{Drivers: map[string]config.DriverConfig{
			"postgres": {Connections: map[string]config.ConnectionConfig{
				"prod": {Password: "hunter2literal"},
			}},
		}},
		Slides: []transformer.TransformedSlide{{HTML: "<p>Slide 1</p>", Index: 0}},
	}

	withSalt := ComputeRevision(pres, nil)

	original := revisionSalt
	t.Cleanup(func() { revisionSalt = original })
	revisionSalt = randomRevisionSalt()

	withDifferentSalt := ComputeRevision(pres, nil)
	if withSalt == withDifferentSalt {
		t.Error("ComputeRevision() produced the same hash under two different salts; the revision must depend on more than the deck's content")
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
		// SlideHash (internal/transformer) returns "" when json.Marshal
		// fails for a slide. Two empty hashes must never compare equal, or
		// a genuinely changed slide whose marshal keeps failing would be
		// reported unchanged and never re-rendered.
		{"both hashes are empty", presentationWithHashes(""), presentationWithHashes(""), []int{1}},
		{"previous hash is empty, next is not", presentationWithHashes(""), presentationWithHashes("a"), []int{1}},
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
