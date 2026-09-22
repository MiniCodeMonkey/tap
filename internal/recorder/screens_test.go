package recorder

import "testing"

const builtInOnlyProfile = `{"SPDisplaysDataType":[{"spdisplays_ndrvs":[
 {"_name":"Color LCD","spdisplays_main":"spdisplays_yes","spdisplays_online":"spdisplays_yes","spdisplays_connection_type":"spdisplays_internal"}]}]}`

const extendedProfile = `{"SPDisplaysDataType":[{"spdisplays_ndrvs":[
 {"_name":"EPSON PJ","spdisplays_online":"spdisplays_yes"},
 {"_name":"Color LCD","spdisplays_main":"spdisplays_yes","spdisplays_online":"spdisplays_yes","spdisplays_connection_type":"spdisplays_internal"}]}]}`

// mirroredProfile is the shape a real mirrored setup reports: both screens
// say mirroring is on, and only the mirror status tells the source screen
// from its copy.
const mirroredProfile = `{"SPDisplaysDataType":[{"spdisplays_ndrvs":[
 {"_name":"Color LCD","spdisplays_main":"spdisplays_yes","spdisplays_online":"spdisplays_yes","spdisplays_connection_type":"spdisplays_internal","spdisplays_mirror":"spdisplays_on","spdisplays_mirror_status":"spdisplays_master_mirror"},
 {"_name":"EPSON PJ","spdisplays_online":"spdisplays_yes","spdisplays_mirror":"spdisplays_on","spdisplays_mirror_status":"spdisplays_hardware_mirror"}]}]}`

// projectorMainProfile has the menu bar on the projector, which makes it
// screencapture's -D1.
const projectorMainProfile = `{"SPDisplaysDataType":[{"spdisplays_ndrvs":[
 {"_name":"EPSON PJ","spdisplays_main":"spdisplays_yes","spdisplays_online":"spdisplays_yes","spdisplays_mirror":"spdisplays_off"},
 {"_name":"Color LCD","spdisplays_online":"spdisplays_yes","spdisplays_connection_type":"spdisplays_internal","spdisplays_mirror":"spdisplays_off"}]}]}`

func TestParseScreensNumbersTheMainScreenFirst(t *testing.T) {
	screens := parseScreens([]byte(extendedProfile))

	if len(screens) != 2 {
		t.Fatalf("got %d screens, want 2", len(screens))
	}
	if screens[0].Name != "Color LCD" || screens[0].Index != 1 || !screens[0].BuiltIn {
		t.Errorf("first screen = %+v, want the built-in main screen at index 1", screens[0])
	}
	if screens[1].Name != "EPSON PJ" || screens[1].Index != 2 || screens[1].BuiltIn {
		t.Errorf("second screen = %+v, want the projector at index 2", screens[1])
	}
}

func TestParseScreensGivesAMirroredScreenNoIndex(t *testing.T) {
	screens := parseScreens([]byte(mirroredProfile))

	if len(screens) != 2 {
		t.Fatalf("got %d screens, want 2", len(screens))
	}
	if screens[0].Mirrored || screens[0].Index != 1 {
		t.Errorf("laptop = %+v, want the mirror source at index 1", screens[0])
	}
	if !screens[1].Mirrored || screens[1].Index != 0 {
		t.Errorf("projector = %+v, want a mirror copy with index 0", screens[1])
	}
}

func TestChooseDisplayFollowsAProjectorThatIsTheMainDisplay(t *testing.T) {
	display, external := ChooseDisplay(parseScreens([]byte(projectorMainProfile)))
	if display != 1 || !external {
		t.Errorf("got (%d, %v), want (1, true)", display, external)
	}
}

func TestParseScreensWithBrokenJSON(t *testing.T) {
	if screens := parseScreens([]byte("{")); screens != nil {
		t.Errorf("got %+v, want nil", screens)
	}
}

func TestChooseDisplayRecordsTheBuiltInScreenAlone(t *testing.T) {
	display, external := ChooseDisplay(parseScreens([]byte(builtInOnlyProfile)))
	if display != 1 || external {
		t.Errorf("got (%d, %v), want (1, false)", display, external)
	}
}

func TestChooseDisplayPrefersTheProjector(t *testing.T) {
	display, external := ChooseDisplay(parseScreens([]byte(extendedProfile)))
	if display != 2 || !external {
		t.Errorf("got (%d, %v), want (2, true)", display, external)
	}
}

func TestChooseDisplayRecordsTheBuiltInScreenWhenMirrored(t *testing.T) {
	display, external := ChooseDisplay(parseScreens([]byte(mirroredProfile)))
	if display != 1 || !external {
		t.Errorf("got (%d, %v), want (1, true)", display, external)
	}
}

func TestChooseDisplayTakesTheFirstOfTwoProjectors(t *testing.T) {
	screens := []Screen{
		{Name: "Color LCD", Index: 1, BuiltIn: true},
		{Name: "Left", Index: 2},
		{Name: "Right", Index: 3},
	}
	if display, _ := ChooseDisplay(screens); display != 2 {
		t.Errorf("got %d, want 2", display)
	}
}

func TestChooseDisplayFallsBackToTheMainDisplay(t *testing.T) {
	if display, external := ChooseDisplay(nil); display != 1 || external {
		t.Errorf("got (%d, %v), want (1, false)", display, external)
	}
}
