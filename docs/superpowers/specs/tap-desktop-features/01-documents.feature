Feature: Documents
  A deck is a plain .md file. The app opens it as an NSDocument, and the file
  on disk stays the only source of truth.

  Background:
    Given Tap Desktop is running

  Scenario: Welcome window
    Given no deck is open
    Then the app shows a welcome window with New Deck, Open, and recent decks with thumbnails
    When a deck opens
    Then the welcome window closes

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

