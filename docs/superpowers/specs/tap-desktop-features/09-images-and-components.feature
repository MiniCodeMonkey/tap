Feature: Images and components

  Scenario: Paste an image
    Given the cursor is in slide 3
    When I paste or drop "diagram.png"
    Then tap copies it into images/ next to the deck, keeping its name (diagram-2.png on a clash)  # NEW tap image add
    And returns the markdown
    And the app inserts the markdown at the cursor as one undo step

  Scenario: Generate an image with AI
    Given a Gemini key is set
    When I choose Slide > Generate Image and enter a prompt
    Then tap generates it exactly as the TUI i key does                       # NEW tap image generate
    And the image and its ai-prompt comment are added to the current slide

  Scenario: Regenerate an AI image
    Given slide 3 has an AI image
    When I choose "Regenerate" on it
    Then tap replaces it in place and deletes the old file                    # NEW tap image regenerate

  Scenario: Create a component
    When I choose Slide > New Component and name it "Counter"
    Then the app runs "tap component new Counter talk.md"
    And inserts the snippet that tap prints into the current slide
    And opens the new .jsx file in my default code editor

  Scenario: Open a component
    When I Cmd-click a component path in a slide
    Then the app opens the .jsx file in my default code editor

  Scenario: Component errors
    Given a component fails to build
    Then tap reports the error on its slide
    And the app marks the slide's box

