package server

import (
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
