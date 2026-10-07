Feature: Slide structure in the editor
  The editor shows one text view. tap tells the app where each slide begins and
  ends, and the app draws a box around each slide.

  Scenario: Boxes come from tap
    Given a deck with frontmatter and 7 slides
    When the app sends the buffer to "PUT /api/app/source"
    Then tap returns, for each slide: its line range, layout, title, step count,
      and its code blocks with driver and live flag                          # NEW fields: code blocks
    And the app draws one box per slide with its number and layout menu

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

  Scenario: Change a slide's layout from its header
    Given the cursor is in slide 3, which has layout "Big Stat"
    Then the header of box 3 shows "Big Stat" as a tinted chip with a chevron
    And other boxes show their layout name quietly, as a chip while the pointer is over it
    When I click the layout name
    Then the layout gallery opens for slide 3, with "Automatic" first and then tap's layouts, and "Big Stat" selected
    And "Automatic" names the layout tap would pick for the slide
    When I pick "Two Columns"
    Then only slide 3's directive comment changes to "layout: two-column"
    And it is one undo step named "Change Layout"
    When I pick "Default"
    Then slide 3's directive comment changes to "layout: default"
    When I pick "Automatic"
    Then the layout line is removed from slide 3's directive comment

  Scenario: Change a component slide's layout from its header
    Given slide 7 has layout "./components/sources/Groups.jsx"
    Then the header of box 7 shows "Groups.jsx" as a chip with a chevron
    When I click the layout name
    Then the layout gallery shows a "Groups.jsx" cell, selected, above a divider, then "Automatic" and tap's layouts
    When I pick "Groups.jsx"
    Then nothing changes
    When I pick "Default"
    Then slide 7's directive comment changes to "layout: default"

  Scenario: Box badges
    Then a box header shows the step count, for example "2 steps"
    And a live-code badge with the driver, for example "sqlite"

  Scenario: The Deck card is the one thing that folds
    Then every line of every slide is always visible in the editor
    And above slide 1 sits the Deck card, always present, the one element of the editor that folds
    And closed, it is one line: "Deck" and chips for the theme, the aspect ratio, and the author when set
    When I click the card, or focus it and press Space
    Then it opens, and it stays open or closed as I left it the next time this deck opens

  Scenario: Deck settings live in the Deck card
    Then the frontmatter text is hidden from the editor while the card is in Form, and slide 1 is the first box
    When I open the Deck card
    Then I see a form with one field per frontmatter key tap knows, in two columns when the editor is wide and one when it is narrow
    And the fields, types, and allowed values come from "tap deck schema --json"   # NEW
    And changing a field rewrites that key in the frontmatter as one undo step
    And keys the form does not know stay untouched, and are listed as "Other keys" to edit as text
    And the inspector shows the preview only

  Scenario: The Deck card shows the frontmatter as text
    Given the Deck card is open
    When I choose "Text" in its Form | Text switch
    Then the frontmatter's own lines are shown in the editor, under the card's header, and I edit them in place
    And an edit there is one undo step in the editor's own undo stack, the same as a change in the form

  Scenario: A deck with no frontmatter has a Deck card
    Given a deck with no frontmatter
    Then the Deck card is above slide 1, closed
    When I open it and change a field
    Then a frontmatter block with that one key is written above slide 1, as one undo step

  Scenario: Frontmatter that does not parse opens the Deck card as text
    Given the frontmatter has a line tap cannot read
    Then the Deck card opens by itself in Text, with the failing line marked


