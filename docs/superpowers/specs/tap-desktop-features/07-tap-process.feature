Feature: The tap process
  Each open deck has one child process, "tap dev --app <file>", from the tap
  binary inside the app bundle.

  Scenario: Start
    When the app opens "talk.md"
    Then the app starts the bundled tap with the login shell environment
    And tap binds 127.0.0.1 on a free port and prints {"port", "token"} on stdout   # NEW --app mode

  Scenario: Login shell environment
    When the app launches
    Then the app runs "$SHELL -l -i -c env" once with a timeout
    And uses that environment for every tap process
    When the command fails or times out
    Then the app uses the default environment and shows a notice

  Scenario: Crash and restart
    When the tap process exits unexpectedly
    Then the preview shows "Restarting preview" with the last good render
    And the app restarts tap with backoff
    And the editor keeps working

  Scenario: tap keeps failing
    When tap exits 3 times within 30 seconds
    Then the app stops retrying and says "The preview stopped" with a "Restart Preview" button
    And the last lines of tap's stderr are behind "Details", never on the notice

  Scenario: A deck problem is not a crash
    When tap exits because a setting of the deck is wrong, and said so in a deck-problems event
    Then the app does not start tap again, since the same deck would fail the same way
    And when the deck changes and no longer has an error, the app starts tap once

  Scenario: tap reports deck problems
    When tap dev --app starts, reloads, or renders the app's buffer
    Then it sends a deck-problems event with each problem's key, value, message, severity, allowed values and nearest values   # NEW
    And an unknown theme is a warning with the nearest theme, an unsupported aspect ratio an error whose first suggestion is "16:9" for "16/9"
    And the event follows the ready line, and a problem that stops tap is sent before it exits

  Scenario: Requests are protected
    Then every request from the app carries the token
    And tap rejects requests with a foreign Origin or a non-JSON body
    And tap rejects a WebSocket upgrade without the token

  Scenario: Version match
    Then the bundled tap has the same version as the app

  Scenario: Logs and version
    When I choose Window > Tap Log
    Then I see the output of each open deck's tap process
    And the About window shows the bundled tap version

