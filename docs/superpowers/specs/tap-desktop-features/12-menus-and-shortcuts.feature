Feature: Menus and shortcuts
  The menu bar lists every command. Every toolbar and context menu action also
  has a menu item.

  Scenario: Menu bar
    Then the app has these menus: Tap, File, Edit, Slide, View, Present, Window, Help

  Scenario: Slide menu
    Then the Slide menu has New Slide (with layouts), Duplicate, Delete,
      Move Up, Move Down, Generate Image, New Component, and Go to Slide

  Scenario: Context menu on a slide
    When I right-click a thumbnail or a box header
    Then I see New Slide After, Duplicate, Delete, Move to Top, Move to Bottom, and Copy

  Scenario: Go to slide
    When I press Cmd+Shift+O and type "ro"
    Then the outline lists matching slide titles and Return jumps to the slide

  Scenario: Shortcuts do not clash with the presentation keys
    Given the preview has focus
    Then presentation keys (arrows, T, O) go to the preview
    Given the editor has focus
    Then those keys type text

  Scenario: Presenting shortcuts
    Then Cmd+Option+P starts presenting at once with the last settings from the Present popover
    And Cmd+Option+Shift+P starts rehearsing
    And clicking the Play button opens the Present popover, which Present > Play with Options also opens


  Scenario: VoiceOver
    Then every box, thumbnail, and drop indicator has a VoiceOver label,
      for example "Slide 3, default layout, What We Knew, 2 steps"
    And every slide operation works from the keyboard alone
