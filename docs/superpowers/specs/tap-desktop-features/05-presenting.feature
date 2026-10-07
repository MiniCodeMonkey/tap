Feature: Presenting
  Play runs tap present. The app starts "tap present --app <file>" as a second
  child process next to the editor's "tap dev --app", picks displays, and puts
  the audience and presenter pages in native windows. Stopping the talk ends
  that process, exactly as quitting tap present does.

  Scenario: Start presenting with two displays
    Given a projector is connected
    And this deck has never been played
    When I click Play
    Then the app shows Present Settings with the display arrangement
    When I click "Play" in Present Settings
    Then the app starts "tap present --app talk.md" as a second process
    And the preview keeps running from the tap dev process
    And the app opens the audience page full screen on the projector
    And the app opens the presenter page on the built-in display
    And both pages start at the slide under the cursor

  Scenario: Play starts at once
    Given this deck was played before, on the displays that are connected now
    And the cursor is in slide 4
    When I click the Play button
    Then the talk starts at slide 4 with the settings I last used, and no popover opens
    And holding the Play button or clicking its chevron opens a menu
    And the menu offers "Play from Here", "Play from Beginning" (with a "Shift click" hint), "Rehearse" and "Present Settings…"
    And with the cursor in slide 1 the menu says "Play from Here" and has no "Play from Beginning"

  Scenario: Play is dimmed while tap gets ready
    When a deck opens and tap has not started yet
    Then the Play glyph, and only the glyph, is dimmed and breathes slowly, and the tooltip says "Getting the slides ready…"
    When tap is ready
    Then the glyph is solid and the tooltip is the usual one
    And with Reduce Motion on the glyph is dimmed and still

  Scenario: Present Settings open on the first run
    Given this deck has never been played, or the connected displays are not the ones it was last played on
    When I click Play
    Then Present Settings open instead of the talk starting, so nobody starts on the wrong screen

  Scenario: Save before presenting
    Given "talk.md" has unsaved edits
    When I click Play or Rehearse
    Then the app saves the buffer first, because tap present reads the file

  Scenario: Play on an untitled deck
    Given a deck that has no file yet, such as the theme tour
    When I click Play, choose Present > Play, or press Cmd+Option+P
    Then the app writes the editor's text to the deck's private untitled file at once, and shows no save prompt
    And the talk shows that text, and the deck stays unsaved
    And an edit I make while presenting reaches the audience the way it does for a saved deck: Reload Slides writes the file again

  Scenario: Nothing interrupts the talk
    While I am presenting or rehearsing
    Then Sparkle shows no update prompt and never restarts the app
    And the first time I present, the app suggests a Focus mode with a button that opens its setting

  Scenario: Start from the first slide
    When I Shift-click Play
    Then presenting starts at slide 1

  Scenario: One display
    Given no second display is connected
    When I start presenting
    Then the audience page fills the screen
    And Option-Tab switches to the presenter window
    And Present Settings show no displays section, no explanation and no menus for displays

  Scenario: Start from the cursor or the beginning
    Given the cursor is in slide 3
    Then the "Start from" menu next to Rehearse and Play offers "Slide 3" and "Beginning"
    And "Slide 3" starts at slide 3, and "Beginning" starts at slide 1
    And with the cursor in slide 1 the menu is not shown

  Scenario: Three displays
    Given three displays are connected
    Then Present Settings draw every display to scale, labelled Presenter, Audience or Not used
    And each display has a menu, named for it, that chooses Audience, Presenter or Not used
    And clicking a display makes it the audience screen
    And there is exactly one audience and at most one presenter, and a role another display holds is swapped with it
    And the diagram follows a display being plugged in or unplugged while Present Settings are open

  Scenario: Rehearse
    When I choose Present > Rehearse
    Then the app runs "tap present --no-record --app talk.md"
    And shows only the presenter view, full screen, with the timer

  Scenario: Swap displays
    When I choose Present > Swap Displays, or make the presenter display the audience in Present Settings
    Then the presenter and audience displays swap before I start

  Scenario: Every tap dev presenter feature works
    Given I am presenting
    Then every audience and presenter key from tap dev works unchanged
      (arrows, Space, Home, End, O, T, F, ?, R, V, 1 to 5 in the layout menu, - and =)
    And the S key opens the presenter view in a native window, not a browser popup
    And the presenter layout and notes size persist between launches

  Scenario: Stop presenting
    When I press Escape in the audience window, or choose Present > Stop
    Then the app closes both presentation windows and returns to the editor
    And the editor cursor moves to the last slide I presented

  Scenario: Presenter controls
    Given I am presenting with the presenter view full screen
    Then the app's toolbar (REC, Reload Slides, Swap Displays, Stop) slides in when the pointer reaches the top edge
    And a small REC dot stays visible in a corner at all times while recording

  Scenario: The Mac stays awake
    While I am presenting
    Then the app holds a display sleep assertion and the cursor hides when idle

  Scenario: Phone remote
    When I turn on "Phone remote" in Present Settings
    Then tap starts the tunnel with a generated presenter password           # NEW endpoint: tunnel start and stop
    And the app shows the QR code that tap generates
    And a (?) button explains the remote, and that it works away from this Wi-Fi through cloudflared

  Scenario: Phone remote password
    Given "Phone remote" is on
    When I turn on "Require a password"
    Then a "Password" field appears, and its text is never saved
    And the app passes it as --presenter-password to tap present

  Scenario: Phone remote needs cloudflared
    Given cloudflared is not installed
    When I turn on "Phone remote"
    Then Present Settings say "Needs cloudflared, which is not installed." in red
    And "Copy install command" copies "brew install cloudflared"
    And Play stays enabled and starts the talk without the remote

  Scenario: Displays share one Space
    Given two displays share one Space
    Then Present Settings show a notice with an "Open Settings" button
    And the button opens Desktop & Dock in System Settings

  Scenario: First talk asks about recording
    Given I have never answered the recording question
    When I start presenting
    Then tap reports its consent question as an event                        # NEW --app event
    And the app shows it as a sheet: "Record automatically every time you present?"
    And tap saves my answer in ~/.config/tap/settings.yaml, shared with the CLI

  Scenario: Recording follows tap present
    Given I answered yes
    Then tap records from the start of the talk, follows the projector, and guards disk space
    And the app shows "REC 12:04" or "NOT RECORDING" as tap reports it

  Scenario: Keep the recording
    When I stop presenting and the run has a recording
    Then tap asks whether to keep it, and the app shows that as a sheet
    And a kept run is revealed in Finder

  Scenario: Edit while presenting
    Given I am presenting
    When I change slide 5 in the editor
    Then the audience does not see the change, because tap present does not watch files
    And the app shows that there are edits the audience has not seen
    When I choose Present > Reload Slides
    Then tap reloads the deck, as r does in tap present

  Scenario: Remember the display assignment
    Given I swapped displays last time with this projector
    When I present again with the same pair of displays
    Then the app uses the same assignment, for every deck

