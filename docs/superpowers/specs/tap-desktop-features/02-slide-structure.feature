Feature: Slide structure in the editor
  The editor shows one text view. tap tells the app where each slide begins and
  ends, and the app draws a box around each slide.

  Scenario: Boxes come from tap
    Given a deck with frontmatter and 7 slides
    When the app sends the buffer to "PUT /api/app/source"
    Then tap returns, for each slide: its line range, layout, title, step count,
      and its code blocks with driver and live flag                          # NEW fields: code blocks
    And the app draws one box per slide with its number, layout, and title

  Scenario: Typing never makes boxes jump
    Given tap has not answered yet for my last keystroke
    Then the app shifts the previous box ranges by the size of my edit
    When tap answers
    Then the app replaces the shifted ranges with tap's ranges

  Scenario: Typing a separator splits a slide
    Given the cursor is on an empty line inside slide 3
    When I type "---" and pause
    Then the app shows slide 3 and a new slide 4 below it

  Scenario: The current slide
    When the cursor is inside slide 3
    Then slide 3 is the current slide
    And its box and its thumbnail in the sidebar are highlighted

  Scenario: Sidebar and cursor are linked
    When I click thumbnail 5
    Then the cursor moves to the start of slide 5 and the preview shows slide 5
    When I Shift-click thumbnail 7
    Then slides 5 to 7 are selected and the cursor is in slide 7
    When I move the cursor into slide 2
    Then only thumbnail 2 is selected

  Scenario: Parse error in one slide
    Given slide 2 has "layout: sectoin"
    Then tap reports an error for slide 2
    And the app marks the box of slide 2 in red and shows the message on the line

  Scenario: Speaker notes
    Given slide 3 has a notes directive
    Then the notes lines stay in the box as text, dimmed and in italics

  Scenario: Code blocks with a live driver
    Given slide 4 has a sql block with "driver: sqlite"
    Then the header of box 4 shows "sql, live"

  Scenario: Box badges
    Then a box header shows the step count, for example "2 steps"
    And a live-code badge with the driver, for example "sqlite"

  Scenario: The editor never folds
    Then every line of every slide is always visible in the editor

  Scenario: Deck settings live in the inspector
    Then the frontmatter text is hidden from the editor, and slide 1 is the first box
    When I choose the "Deck" tab in the inspector (Preview | Deck)
    Then I see a form with one field per frontmatter key tap knows
    And the fields, types, and allowed values come from "tap deck schema --json"   # NEW
    And changing a field rewrites that key in the frontmatter as one undo step
    And keys the form does not know stay untouched, and are listed as "Other keys" to edit as text

