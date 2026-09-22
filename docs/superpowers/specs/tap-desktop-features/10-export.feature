Feature: Export

  Scenario: Export a PDF
    When I choose File > Export > PDF and pick slides, notes, or both
    Then the app runs "tap export pdf talk.md --output <path> --content <choice> --progress json"   # NEW --progress json
    And shows real progress, then reveals the file in Finder

  Scenario: First PDF export
    Given the export engine is not downloaded
    When I export a PDF
    Then the app shows a one-time "Downloading export engine" sheet with progress

  Scenario: Export a static site
    When I choose File > Export > Website
    Then the app runs "tap build talk.md --output <folder> --progress json"
    And offers "Preview", which runs "tap serve <folder>" and opens it

  Scenario: Export slide images
    When I choose File > Export > Images
    Then the app runs "tap export images talk.md --all --output <folder>"

  Scenario: A slide fails during export
    Given slide 2 renders an error card
    Then the export finishes and lists slide 2 as a warning, as tap pdf does

  Scenario: Cancel an export
    When I cancel during export
    Then the app sends SIGINT and tap exits cleanly

