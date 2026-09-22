Feature: Performance

  Background:
    Given a generated deck with 200 slides and a component on every slide

  Scenario: Typing
    Then typing latency stays under 16 ms

  Scenario: Preview update
    Then the preview shows an edit within 200 ms after I stop typing

  Scenario: The preview updates in place
    When I type in slide 3 and pause
    Then tap sends an "update" message, not "reload"                         # NEW in-place update
    And the preview re-renders slide 3 without a page reload, keeping the current step
    And no browser process is started

  Scenario: The current thumbnail follows the preview
    When the preview has re-rendered slide 3
    Then thumbnail 3 becomes a snapshot of the preview

  Scenario: Thumbnails
    Then only the preview is a live render
    And thumbnails are static images from one hidden renderer, cached on disk
    And the renderer waits for tap's ready signal before each snapshot       # NEW single ready signal, shared with tap pdf

  Scenario: Reopen
    When I reopen the deck
    Then every thumbnail loads from the cache without rendering
