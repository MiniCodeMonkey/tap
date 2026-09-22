Feature: Settings and the tap command

  Scenario: Settings window
    When I open Settings
    Then I see tabs for General, Live Code, Image Generation, and Command Line
    And Live Code lists the approvals tap keeps in ~/.config/tap/settings.yaml

  Scenario: Gemini key
    When I enter a key in Settings > Image Generation
    Then the app stores it in the Keychain and passes it to tap as GEMINI_API_KEY
    And a GEMINI_API_KEY from the login shell takes precedence

  Scenario: Install the tap command
    Given no tap is on PATH
    When I choose Install Command Line Tool
    Then the app links the bundled tap into ~/.local/bin after I confirm

  Scenario: Another tap is already installed
    Given "/opt/homebrew/bin/tap" version 2.0.0 is on PATH
    Then Settings > Command Line shows its path and version next to the bundled version
    And the app never replaces or deletes it

  Scenario: Shared settings
    Then the recording consent answer and live code approvals live in ~/.config/tap/settings.yaml
    And the app reads and writes them through tap, so the CLI sees the same values

  Scenario: General settings
    Then Settings > General has the editor font size and line spacing
    And the default theme for new decks, passed to tap new --theme
    And the autosave delay, 1 second by default

