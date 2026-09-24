Feature: Preview
  The preview is the same audience page that tap dev serves, shown in a
  WKWebView next to the editor.

  Scenario: Split layout
    Then the editor and the preview each take half of the window, and the divider can be dragged
    And on first launch the slide panel is pinned, and each deck remembers its state after that

  Scenario: Peek at the slide panel
    Given the slide panel is not pinned
    When I hover the sidebar button in the toolbar
    Then the panel appears as a glass overlay
    And I can click a slide in it to jump there
    When the pointer leaves the button and the panel
    Then the panel hides

  Scenario: Pin the slide panel
    When I click the sidebar button
    Then the panel docks as a sidebar and pushes the editor and preview to the right, with no overlap
    When I click it again
    Then the panel unpins, and hovering the button peeks at it again

  Scenario: The preview follows the cursor
    Given the cursor is in slide 3
    Then the preview shows slide 3 with all steps revealed

  Scenario: The preview updates while I type
    When I change a bullet in slide 3 and pause
    Then the preview shows the change within 200 ms

  Scenario: Step through fragments
    Given slide 3 has 2 steps
    When I use the step controls
    Then the preview shows step 1, then step 2
    And the app drives the page through the existing WebSocket "slide" message

  Scenario: Step through a custom component
    Given slide 8 uses the whole-slide component "./slides/RollingDeploy.jsx" with "export const steps = 5"
    Then tap reports 5 steps for slide 8, and the box shows "5 steps"
    When I use the step controls
    Then the component receives step 1 to 5 through useStep(), exactly as in tap dev
    And the thumbnail shows the final step, because tap renders previews at the last step
    When I change the export to 6 and save the .jsx file
    Then tap rebuilds the component and reports 6 steps

  Scenario: Pin a slide
    Given the preview shows slide 3
    When I pin it and move the cursor to slide 6
    Then the preview keeps showing slide 3

  Scenario: The preview shows the audience-safe error form
    Given slide 2 has a component that throws
    Then the preview shows the full error card, the same as tap dev in a browser
    And the audience-safe error form is used only while presenting

  Scenario: Preview in its own window
    When I choose View > Preview in Window
    Then the preview moves into a resizable window that keeps following the cursor
    When I close that window
    Then the preview docks back next to the editor

  Scenario: Hide the preview
    When I choose View > Hide Preview
    Then the editor takes the full width

  Scenario: The preview is the audience view
    Then the preview always shows the audience view, never the presenter view

  Scenario: Try a theme in the preview
    When I press T with the preview focused
    Then the preview cycles themes and the file does not change

