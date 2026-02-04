# Changelog

All notable changes to Tap are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-02-04

### Added

- **Animated map slides** - New `map` slide type with animated transitions between locations. Supports custom markers, zoom levels, and route animations.
- **Scroll reveal directive** - Add `<!-- scroll -->` to slides with long content for smooth scroll-based progressive reveal.
- **Fragment auto-fragmentation** - List items can now automatically animate in sequence using the `<!-- fragments -->` directive.
- **PDF export improvements** - Added print mode for fragments and PDF metadata support (title, author, subject).
- **Interactive file picker** - When no slide file is provided, tap now shows an interactive file picker.
- **Bidirectional presenter sync** - Complete two-way synchronization between presenter and audience views.
- **Image preloading** - Images are preloaded on page load to prevent transition flashes.
- **15 new themes** - Ink, Manuscript, Deco, Stained Glass, Bauhaus, Watercolor, Comic, Blueprint, Editorial, Synthwave, Safari, Botanical, Cyber, Origami, and Chalkboard themes with example presentations.
- **Theme gallery** - New documentation page showcasing all available themes with screenshots.
- **Local font embedding** - Font infrastructure for embedding fonts locally in themes.

### Changed

- **Theme consolidation** - Reduced built-in themes from 20 to 8 core themes, with 15 additional artistic themes.
- **Improved core themes** - Enhanced Aurora, Noir, Paper, Phosphor, and Poster themes with better typography, spacing, and visual effects.
- **Better syntax highlighting** - Refined code block syntax highlighting per theme.
- **Mermaid integration** - Improved Mermaid diagram theme integration and foreignObject text clipping fix.

### Fixed

- **Slide transitions** - Enabled smooth slide transitions between slides.
- **PDF export** - Resolved PDF export failures and added image support.
- **Presenter view** - Speaker notes now display correctly in presenter view.
- **Image sizing** - Markdown image sizing attributes now apply correctly.
- **Code block parsing** - Slide delimiters inside code blocks are now ignored.
- **Scroll state** - Prevented scroll state from incorrectly triggering on forward navigation.
- **Dev server bugs** - Multiple dev server stability improvements.

## [0.0.1] - 2026-01-25

### Added

- **Mermaid diagram support** - Render flowcharts, sequence diagrams, ER diagrams, and more directly in slides using mermaid code blocks. Diagrams automatically match your presentation theme.
- **AI image generation** - Generate images from text prompts using Google Gemini. Press `i` in the dev server to open the image generator, describe what you want, and the image is created and inserted into your slide.
