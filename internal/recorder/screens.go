package recorder

import "encoding/json"

// Screen is one online display as system_profiler reports it. Index is the
// screencapture -D number: the main display, the one with the menu bar, is
// 1, and the other screens follow in system_profiler order. A mirror copy
// shows another screen's picture and has no -D number of its own, so its
// Index is 0.
type Screen struct {
	Name     string
	Index    int
	BuiltIn  bool
	Mirrored bool
}

// screenProfile is the part of the system_profiler display report the
// screen list reads.
type screenProfile struct {
	Displays []struct {
		Screens []struct {
			Name           string `json:"_name"`
			Main           string `json:"spdisplays_main"`
			Online         string `json:"spdisplays_online"`
			ConnectionType string `json:"spdisplays_connection_type"`
			Mirror         string `json:"spdisplays_mirror"`
			MirrorStatus   string `json:"spdisplays_mirror_status"`
		} `json:"spdisplays_ndrvs"`
	} `json:"SPDisplaysDataType"`
}

// parseScreens reads the online screens, main screen first, and numbers
// the ones screencapture can address.
func parseScreens(raw []byte) []Screen {
	var profile screenProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return nil
	}

	var main, others []Screen
	for _, group := range profile.Displays {
		for _, entry := range group.Screens {
			if entry.Online == "spdisplays_no" {
				continue
			}
			screen := Screen{
				Name:    entry.Name,
				BuiltIn: entry.ConnectionType == "spdisplays_internal",
				// Both screens report mirroring on; only the source
				// screen is the master, and only it has a -D number.
				Mirrored: entry.Mirror == "spdisplays_on" && entry.MirrorStatus != "spdisplays_master_mirror",
			}
			if entry.Main == "spdisplays_yes" {
				main = append(main, screen)
			} else {
				others = append(others, screen)
			}
		}
	}

	screens := append(main, others...)
	next := 1
	for position := range screens {
		if screens[position].Mirrored {
			continue
		}
		screens[position].Index = next
		next++
	}
	return screens
}

// ChooseDisplay is the display a recording should capture: the first
// external screen screencapture can address, or the built-in screen when
// there is none. externalConnected reports whether any external screen is
// connected at all, mirrored or not, because a mirrored projector still
// means the talk is on a projector.
func ChooseDisplay(screens []Screen) (display int, externalConnected bool) {
	for _, screen := range screens {
		if screen.BuiltIn {
			continue
		}
		externalConnected = true
		if display == 0 && screen.Index > 0 {
			display = screen.Index
		}
	}
	if display == 0 {
		display = 1
	}
	return display, externalConnected
}
