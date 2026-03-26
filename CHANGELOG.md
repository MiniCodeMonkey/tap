# Changelog

All notable changes to Tap are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.0] - 2026-03-26

### Added

- **Asciinema terminal recording support** - Embed terminal recordings directly in slides with full playback support.
- **5 new themes** - Carbon, Flux, Mono, Signal, and Spectrum themes with distinct visual styles.

### Changed

- **Presenter view layout** - Slides now display on top with notes below for a more natural workflow.

### Fixed

- **Asciinema player** - Hide player control bar by default for cleaner slide appearance.
- **Theme list alignment** - Fix list padding and bullet design in Spectrum theme.
- **Centered layout alignment** - Force left-align on lists and blockquotes inside centered layouts.
- **Code block alignment** - Prevent text-align center on code blocks across all themes.
- **Static builds** - Use real Vite frontend in static builds for proper theme rendering.
- **Post theme** - Fix code block colors in Post theme.
- **PDF export** - Disable slide transitions in print mode to prevent ghosting.

## [0.1.0] - 2026-02-04

### Added

- **Animated map slides** - New `map` slide type with animated transitions between locations. Supports custom markers, zoom levels, and route animations.
- **Scroll reveal directive** - Add `<!-- scroll -->` to slides with long content for smooth scroll-based progressive reveal.
- **Fragment auto-fragmentation** - List items can now automatically animate in sequence using the `<!-- fragments -->` directive.
- **PDF export improvements** - Added print mode for fragments and PDF metadata support (title, author, subject).
- **Interactive file picker** - When no slide file is provided, tap now shows an interactive file picker.
- **Bidirectional presenter sync** - Complete two-way synchronization between presenter and audience views.
- **Image preloading** - Images are preloaded on page load to prevent transition flashes.
- **Theme gallery** - New documentation page showcasing all available themes with screenshots.
- **Local font embedding** - Font infrastructure for embedding fonts locally in themes.

### Changed

- **8 curated themes** - Aurora, Bauhaus, Editorial, Ink, Noir, Paper, Phosphor, and Poster themes with improved typography, spacing, and visual effects.
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
