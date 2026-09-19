package server

import (
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
