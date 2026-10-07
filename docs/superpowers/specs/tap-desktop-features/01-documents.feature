Feature: Documents
  A deck is a plain .md file. The app opens it as an NSDocument, and the file
  on disk stays the only source of truth.

  Background:
    Given Tap Desktop is running

  Scenario: Welcome window
    Given no deck is open
    Then the app shows a welcome window, 880 by 560 points, with New Deck, Open, a search field and recent decks with thumbnails
    And the brand pane on the left shows the app icon, the wordmark and the tagline over the aurora
    And the recent decks list on the right is opaque, in the system's selection color, with no keyboard hint footer
    When a deck opens
    Then the welcome window closes

  Scenario: Welcome window without recent decks
    Given no deck is open and the recent decks list is empty
    Then the welcome window shows one centered hero instead of its two panes
    And the hero has the app icon, the wordmark "tap", the tagline "Markdown slides, without the markdown limits.", New Deck and Open buttons, a "take the theme tour" link and a drifting filmstrip of every theme
    And New Deck is the default button, drawn black in light mode and white in dark mode
    And the window shows no version number and no drop hint
    When I click a theme card
    Then the New Deck sheet opens with that theme chosen
    When I drop a .md file anywhere on the window
    Then the app opens it as a deck
    When a deck is added to the recent decks list while the window is open
    Then the window switches to its recent decks layout

  Scenario: Welcome window drag state
    Given the welcome window is showing
    When I drag a .md file over it
    Then the content steps back, a green ring lights inside the window edge and a pill says "Drop to open" and the file's name
    When I drag a file that is not Markdown over it
    Then nothing changes

  Scenario: Welcome window theme tour
    Given the welcome window has no recent decks
    When I click "take the theme tour"
    Then the app opens the bundled theme tour as a new untitled deck
    And its first save asks where to put it

  Scenario: The theme tour opens with its thumbnails
    Given the app bundle holds the tour's slide thumbnails, rendered at build time
    When I open the theme tour and change nothing
    Then every slide's thumbnail shows at once and no render starts
    When I edit a slide of the tour
    Then the edited slide renders as usual

  Scenario: Welcome window search
    Given the welcome window shows recent decks
    When I type while the window is key
    Then the search field takes the text and the list shows only the decks whose name or folder matches every word

  Scenario: Welcome window with Reduce Motion
    Given Reduce Motion is on
    When the welcome window opens
    Then the aurora draws one still frame at its resting height
    And the icon's caret does not blink, the filmstrip holds still and scrolls by hand, and nothing rises in

  Scenario: Welcome window aurora pauses while unseen
    Given the welcome window shows the aurora
    When the window is covered, minimized or hidden, or the app is hidden
    Then the aurora draws no frames until the window is visible again

  Scenario: App icon
    Then the app bundle carries an app icon, a dark squircle with a translucent slide and a green caret lit from below
    And the bundle carries the Instrument Sans typeface and its license

  Scenario: Open a deck
    When I open "talk.md" from Finder, File > Open, or Open Recent
    Then the app shows it in a new window, or in a new tab when a window is open
    And the app starts one "tap dev --app talk.md" process for that document

  Scenario: Open a file that is not a deck
    When I open a .md file that has no slide separators and no frontmatter
    Then the app opens it as a deck with one slide

  Scenario: Open the same deck twice
    Given "talk.md" is open
    When I open "talk.md" again
    Then the app brings the existing window to the front

  Scenario: Autosave in place
    Given I typed in "talk.md"
    When I stop typing
    Then the app saves the buffer to disk through NSDocument autosave
    And the title bar shows the document as saved

  Scenario: Browse versions
    When I choose File > Revert To > Browse All Versions
    Then the app shows the macOS version browser for "talk.md"

  Scenario: External change with no unsaved edits
    Given "talk.md" has no unsaved edits
    When another program writes "talk.md"
    Then tap sends a "file-changed" message                                  # NEW message type
    And the app loads the disk version as one undoable edit
    And the cursor and scroll position stay on the same slide when it still exists

  Scenario: External change with unsaved edits
    Given "talk.md" has unsaved edits
    When another program writes "talk.md"
    Then the app shows the "changed on disk" bar with "Load Disk Version" and "Keep Mine"

  Scenario: Keep mine
    Given the "changed on disk" bar is showing
    When I choose "Keep Mine"
    Then the app writes my buffer to disk and hides the bar

  Scenario: Deck deleted or moved while open
    Given "talk.md" is open
    When another program deletes or renames "talk.md"
    Then the app follows the rename like other NSDocument apps

  Scenario: Close a window
    When I close the window for "talk.md"
    Then the app stops its tap process

  Scenario: Open a folder
    When I open a folder
    Then the app opens the deck that tap's shared deck resolution rule picks in it

  Scenario: Deck deleted while open
    When another program deletes "talk.md"
    Then the app keeps the buffer as an unsaved document
    And shows a bar that says the file was deleted, with "Save As"

  Scenario: Last window closed
    When I close the last deck window
    Then the app keeps running and shows the welcome window

  Scenario: Set up recording before the first talk
    Given recording is not set up and I have not chosen Set Up Later
    When the welcome window would show at launch
    Then the window shows the recording setup screen in place of its layouts, over the same aurora, at the same size
    And the Microphone step has the only button, "Allow Microphone", and the Screen Recording step waits, dimmed
    When the microphone is off in System Settings
    Then the button reads "Open Settings" and opens the Microphone pane
    When I press "Open Settings" for Screen Recording
    Then the app asks macOS for Screen Recording, opens its pane, and shows "Waiting for System Settings" with a guide to the switch in place of the button
    And the screen updates by itself as macOS reports each permission
    When both permissions are on
    Then the screen says "You're ready to record" and "Continue to Tap" is the main button
    When I choose "Continue to Tap"
    Then the window shows its usual content, or closes if a deck is open
    When I choose "Set Up Later"
    Then the screen never shows by itself again, and Help > Set Up Recording… shows it at any time
    When I pressed "Open Settings" for Screen Recording and Tap relaunches with both permissions on
    Then the screen shows once more with both steps checked
    When a deck opens at launch
    Then the welcome window and this screen are skipped
