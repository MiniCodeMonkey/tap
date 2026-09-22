Feature: Presenting
  Play runs tap present. The app starts "tap present --app <file>" as a second
  child process next to the editor's "tap dev --app", picks displays, and puts
  the audience and presenter pages in native windows. Stopping the talk ends
  that process, exactly as quitting tap present does.

  Scenario: Start presenting with two displays
    Given a projector is connected
    When I click Play
    Then the app shows the Present popover with the display arrangement
    When I click "Start Presenting"
    Then the app starts "tap present --app talk.md" as a second process
    And the preview keeps running from the tap dev process
    And the app opens the audience page full screen on the projector
    And the app opens the presenter page on the built-in display
    And both pages start at the slide under the cursor

  Scenario: Save before presenting
    Given "talk.md" has unsaved edits
    When I click Play or Rehearse
    Then the app saves the buffer first, because tap present reads the file

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

  Scenario: Rehearse
    When I choose Present > Rehearse
    Then the app runs "tap present --no-record --app talk.md"
    And shows only the presenter view, full screen, with the timer

  Scenario: Swap displays
    When I click "Swap Displays" in the popover
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
    When I turn on "Phone remote"
    Then tap starts the tunnel with a generated presenter password           # NEW endpoint: tunnel start and stop
    And the app shows the QR code that tap generates

  Scenario: Advanced remote options
    When I open "Advanced" in the Present popover
    Then I can set a presenter password and turn the tunnel on or off
    And the app passes them as --presenter-password and --tunnel to tap present

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

