Feature: Slide operations
  Structural edits are editor features and belong to the app. The CLI does not
  offer them. The app edits its own buffer using the slide ranges tap reports,
  and each operation is one undo step.

  Background:
    Given a deck with 7 slides

  Scenario: Move one slide by dragging in the sidebar
    When I drag slide 5 above slide 3
    Then the app moves the text of slide 5, using tap's range for it, above slide 3
    And the slide order becomes 1, 2, 5, 3, 4, 6, 7
    And the file has exactly one "---" line between each pair of slides
    And Cmd+Z restores the old order

  Scenario: Move with the keyboard
    Given the cursor is in slide 3
    When I press Cmd+Option+Up
    Then slide 3 moves above slide 2 and the cursor moves with it

  Scenario: Move several slides
    Given slides 5 and 6 are selected in the sidebar
    When I drag them above slide 3
    Then the order becomes 1, 2, 5, 6, 3, 4, 7 in one undo step

  Scenario: Move by dragging a box header in the editor
    When I drag the header of box 5 above box 3
    Then the result is the same as dragging in the sidebar

  Scenario: Insert a slide with the last layout
    Given the cursor is in slide 3 and I last inserted a two-column slide
    When I click New Slide in the toolbar
    Then the app inserts the two-column template after slide 3
    And the cursor moves into the new slide's first slot

  Scenario: Pick a layout from the gallery
    When I hold New Slide, or choose Slide > New Slide
    Then the app shows a gallery with a preview of each layout
    And the layouts and their templates come from tap                        # NEW tap slide add --layout <x> --print
    When I pick "Big Stat"
    Then the app inserts that template after the current slide

  Scenario: Duplicate and delete
    When I duplicate slide 3
    Then the app inserts a copy after slide 3
    When I delete slides 5 and 6
    Then the app removes them in one undo step

  Scenario: Skip a slide
    When I choose Skip Slide on slide 4
    Then the app adds "skip: true" to slide 4's directive comment
    And tap leaves slide 4 out when presenting and exporting                 # NEW tap directive: skip
    And the app shows slide 4 dimmed in the sidebar and the editor

  Scenario: Frontmatter never moves
    When I drop a slide above slide 1
    Then the app inserts it after the frontmatter, never above it

  Scenario: Drag slides to another deck
    When I drag slides 5 and 6 from one deck's sidebar into another deck's sidebar
    Then the app copies them into the other deck
    When I hold Cmd while dropping
    Then the app moves them instead

# Note: a "---" inside a fenced block is not a separator, because the ranges come from tap's parser.

