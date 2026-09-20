package recorder

import "testing"

const twoDisplayProfile = `{
  "SPDisplaysDataType": [
    {
      "spdisplays_ndrvs": [
        {
          "_name": "Color LCD",
          "_spdisplays_resolution": "2294 x 1432 @ 120.00Hz",
          "spdisplays_main": "spdisplays_yes",
          "spdisplays_online": "spdisplays_yes"
        },
        {
          "_name": "DELL U2720Q",
          "_spdisplays_resolution": "3840 x 2160 @ 60.00Hz",
          "spdisplays_online": "spdisplays_yes"
        }
      ]
    }
  ]
}`

func TestParseDisplayCountReadsTheRefusal(t *testing.T) {
	got, ok := parseDisplayCount("screencapture: Invalid display specified. Only 1 display, the only valid value is 1.")
	if !ok {
		t.Fatal("parseDisplayCount() did not recognise the message")
	}
	if got != 1 {
		t.Errorf("parseDisplayCount() = %d, want 1", got)
	}
}

func TestParseDisplayCountReadsAPluralRefusal(t *testing.T) {
	got, ok := parseDisplayCount("screencapture: Invalid display specified. Only 2 displays, the valid values are 1 to 2.")
	if !ok {
		t.Fatal("parseDisplayCount() did not recognise the plural message")
	}
	if got != 2 {
		t.Errorf("parseDisplayCount() = %d, want 2", got)
	}
}

func TestParseDisplayCountRejectsSomethingElse(t *testing.T) {
	if _, ok := parseDisplayCount("screencapture: cannot write file"); ok {
		t.Error("parseDisplayCount() claimed to read an unrelated message")
	}
}

func TestParseDisplayDetailsPutsTheMainDisplayFirst(t *testing.T) {
	got := parseDisplayDetails([]byte(twoDisplayProfile))

	if len(got) != 2 {
		t.Fatalf("parseDisplayDetails() returned %d displays, want 2", len(got))
	}
	if got[0].Name != "Color LCD" || !got[0].Main {
		t.Errorf("first display = %+v, want the main Color LCD", got[0])
	}
	if got[0].Resolution != "2294 x 1432" {
		t.Errorf("Resolution = %q, want the refresh rate stripped", got[0].Resolution)
	}
	if got[1].Name != "DELL U2720Q" {
		t.Errorf("second display = %q, want DELL U2720Q", got[1].Name)
	}
}

func TestParseDisplayDetailsSkipsOfflineDisplays(t *testing.T) {
	raw := `{"SPDisplaysDataType":[{"spdisplays_ndrvs":[
		{"_name":"Color LCD","spdisplays_main":"spdisplays_yes","spdisplays_online":"spdisplays_yes"},
		{"_name":"Sleeping","spdisplays_online":"spdisplays_no"}
	]}]}`

	got := parseDisplayDetails([]byte(raw))
	if len(got) != 1 {
		t.Fatalf("parseDisplayDetails() returned %d displays, want 1", len(got))
	}
}

func TestMergeDisplaysNumbersFromOne(t *testing.T) {
	got := mergeDisplays(2, parseDisplayDetails([]byte(twoDisplayProfile)))

	if len(got) != 2 {
		t.Fatalf("mergeDisplays() returned %d displays, want 2", len(got))
	}
	if got[0].Index != 1 || got[1].Index != 2 {
		t.Errorf("indexes = %d, %d, want 1, 2", got[0].Index, got[1].Index)
	}
	if got[0].Name != "Color LCD" {
		t.Errorf("display 1 = %q, want the main display", got[0].Name)
	}
}

func TestMergeDisplaysDropsNamesWhenTheCountsDisagree(t *testing.T) {
	got := mergeDisplays(3, parseDisplayDetails([]byte(twoDisplayProfile)))

	if len(got) != 3 {
		t.Fatalf("mergeDisplays() returned %d displays, want 3", len(got))
	}
	for _, display := range got {
		if display.Name != "" || display.Resolution != "" {
			t.Errorf("display %d carries a name %q, want it dropped when the sources disagree", display.Index, display.Name)
		}
	}
	if !got[0].Main {
		t.Error("display 1 is not marked main, and screencapture documents -D 1 as the main display")
	}
}
