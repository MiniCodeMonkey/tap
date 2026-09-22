Feature: Creating decks and themes

  Scenario: New deck
    When I choose File > New
    Then the app shows a sheet with title, theme, and location, defaulting to the last used folder
    And the app runs "tap new --yes --title <t> --theme <slug> --output <path>"
    And the app creates a folder with the deck and an images/ folder
    And opens the result
    And tap records an approval for the new deck, because I made it          # NEW tap new approves its own deck

  Scenario: Theme picker lists tap's themes
    When I open the theme picker in the New Deck sheet, the toolbar, or the Deck tab
    Then I see all themes from "tap theme list --json" as one scrolling grid, grouped light and dark
    And each cell is a real render of a title slide in that theme            # NEW tap theme show <slug> --image
    And the renders are cached, so the grid opens instantly after the first time

  Scenario: Change the deck's theme
    When I pick "Midnight" in the theme pop-up
    Then the app saves the buffer first
    And tap sets "theme: midnight" in the frontmatter                        # NEW tap theme set
    And the app loads the result as one undo step, like an external change
    And the preview and thumbnails re-render

  Scenario: Try a theme without saving it
    When I press T in the preview, as in tap dev
    Then the preview cycles themes without changing the file

