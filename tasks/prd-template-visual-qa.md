# PRD: Template Visual QA & Design Enhancement

## Introduction

This project establishes a comprehensive visual quality assurance system for tap's presentation templates. We will use Playwright to render every slide in every example template, capture screenshots, detect rendering errors, and create a visual gallery report. Beyond testing, this initiative includes elevating the visual design quality of all templates to a stunning, production-ready standard through improvements to the core theme system and individual template polish.

## Goals

- Ensure zero rendering errors across all example templates
- Capture high-quality screenshots of every slide in every template
- Generate a visual gallery/report showcasing all templates
- Elevate visual design quality to showcase/production level
- Improve core CSS theme system for better defaults
- Polish typography, colors, spacing, and layouts across all themes
- Create a repeatable visual regression testing workflow

## User Stories

### US-001: Add Playwright testing infrastructure
**Description:** As a developer, I need Playwright set up in the project so we can automate browser-based visual testing.

**Acceptance Criteria:**
- [ ] Playwright installed as dev dependency
- [ ] Playwright config file created with appropriate settings
- [ ] Test directory structure established (`tests/visual/`)
- [ ] Scripts added to package.json for running visual tests
- [ ] Typecheck/lint passes

---

### US-002: Create template test runner script
**Description:** As a developer, I need a test script that iterates through all example templates and renders them with tap dev in headless mode.

**Acceptance Criteria:**
- [ ] Script discovers all `.md` files in `examples/` folder
- [ ] Script starts `tap dev --headless` for each template
- [ ] Script waits for server to be ready before proceeding
- [ ] Script gracefully handles server startup/shutdown
- [ ] Script can be run via npm/pnpm command
- [ ] Typecheck/lint passes

---

### US-003: Implement slide-by-slide screenshot capture
**Description:** As a developer, I need Playwright to navigate through each slide and capture a screenshot so we have visual records of every slide.

**Acceptance Criteria:**
- [ ] Playwright opens presentation at `http://localhost:<port>`
- [ ] Test navigates through all slides using keyboard (arrow keys) or URL params
- [ ] Screenshot captured for each slide at consistent viewport (1920x1080)
- [ ] Screenshots saved with naming convention: `{template}-{theme}-slide-{number}.png`
- [ ] Fragment states captured (each pause point gets a screenshot)
- [ ] Typecheck/lint passes

---

### US-004: Implement rendering error detection
**Description:** As a developer, I need the test to detect and report any rendering errors so we catch issues automatically.

**Acceptance Criteria:**
- [ ] Console errors captured during rendering
- [ ] JavaScript exceptions caught and reported
- [ ] Missing image/asset errors detected
- [ ] Mermaid rendering failures detected
- [ ] Code highlighting failures detected
- [ ] Test fails if any errors are found
- [ ] Error report includes template name, slide number, and error details
- [ ] Typecheck/lint passes

---

### US-005: Generate visual gallery report
**Description:** As a developer, I need a visual gallery HTML report that showcases all templates and their slides so stakeholders can review the visual quality.

**Acceptance Criteria:**
- [ ] HTML report generated after test run
- [ ] Report organized by template, then by slide
- [ ] Each screenshot displayed with template name, theme, and slide number
- [ ] Report includes summary stats (total templates, slides, any errors)
- [ ] Report is self-contained (images embedded or relative paths)
- [ ] Report saved to `tests/visual/gallery/index.html`
- [ ] Report viewable in any browser
- [ ] Typecheck/lint passes

---

### US-006: Audit and improve Paper theme
**Description:** As a designer, I want the Paper theme to look stunning with refined typography, spacing, and visual hierarchy so presentations look professional.

**Acceptance Criteria:**
- [ ] Typography audit: font sizes, weights, line heights optimized for readability
- [ ] Spacing audit: margins, padding, gaps create visual breathing room
- [ ] Color contrast meets WCAG AA standards
- [ ] Code blocks have polished appearance with proper syntax highlighting contrast
- [ ] Headings have appropriate visual weight and hierarchy
- [ ] Lists and blockquotes have refined styling
- [ ] Typecheck/lint passes
- [ ] Verify in browser - all slides in `theme-paper.md` look polished

---

### US-007: Audit and improve Noir theme
**Description:** As a designer, I want the Noir theme to look stunning with cinematic dark aesthetics so presentations feel premium.

**Acceptance Criteria:**
- [ ] Dark background has appropriate depth (not flat black)
- [ ] Gold accents are refined and not overwhelming
- [ ] Typography has elegant contrast against dark background
- [ ] Code blocks integrate seamlessly with dark aesthetic
- [ ] Subtle gradients or shadows add depth where appropriate
- [ ] Typecheck/lint passes
- [ ] Verify in browser - all slides in `theme-noir.md` look polished

---

### US-008: Audit and improve Aurora theme
**Description:** As a designer, I want the Aurora theme to look stunning with beautiful gradients and glassmorphism so presentations feel modern.

**Acceptance Criteria:**
- [ ] Gradient backgrounds are smooth and visually appealing
- [ ] Glassmorphism effects are subtle and enhance readability
- [ ] Text remains highly readable against gradient backgrounds
- [ ] Code blocks work well with the colorful aesthetic
- [ ] Transitions and effects feel cohesive
- [ ] Typecheck/lint passes
- [ ] Verify in browser - all slides in `theme-aurora.md` look polished

---

### US-009: Audit and improve Phosphor theme
**Description:** As a designer, I want the Phosphor theme to look stunning with authentic CRT/terminal aesthetics so presentations feel retro-futuristic.

**Acceptance Criteria:**
- [ ] Green phosphor color is authentic but not harsh on eyes
- [ ] CRT effects (if any) are subtle and not distracting
- [ ] Monospace typography is well-balanced
- [ ] Code blocks feel native to the terminal aesthetic
- [ ] Scanlines or glow effects enhance rather than detract
- [ ] Typecheck/lint passes
- [ ] Verify in browser - all slides in `theme-phosphor.md` look polished

---

### US-010: Audit and improve Poster theme
**Description:** As a designer, I want the Poster theme to look stunning with bold, artistic styling so presentations make a strong visual impact.

**Acceptance Criteria:**
- [ ] Bold typography makes strong visual statements
- [ ] Color palette is vibrant but harmonious
- [ ] Layout spacing supports the bold aesthetic
- [ ] Visual hierarchy is clear despite bold styling
- [ ] Code blocks maintain readability within artistic style
- [ ] Typecheck/lint passes
- [ ] Verify in browser - all slides in `theme-poster.md` look polished

---

### US-011: Improve core layout system typography
**Description:** As a developer, I want the core `layouts.css` to have refined typography defaults so all themes benefit from better type hierarchy.

**Acceptance Criteria:**
- [ ] Title slide typography is impactful (size, weight, letter-spacing)
- [ ] Section headers have appropriate prominence
- [ ] Body text is comfortably readable at presentation scale
- [ ] Code blocks have consistent, readable sizing
- [ ] List items have proper spacing and bullet styling
- [ ] Blockquotes have refined, elegant styling
- [ ] Two-column and three-column layouts have balanced spacing
- [ ] Typecheck/lint passes
- [ ] Verify improvements across multiple themes

---

### US-012: Improve core layout system spacing
**Description:** As a developer, I want the core `layouts.css` to have refined spacing so slides have proper visual breathing room.

**Acceptance Criteria:**
- [ ] Slide padding provides comfortable margins from edges
- [ ] Vertical rhythm between elements is consistent
- [ ] Gaps in column layouts are visually balanced
- [ ] Code blocks have appropriate internal padding
- [ ] Lists have proper indentation and item spacing
- [ ] Big-stat layout has dramatic, centered presence
- [ ] Quote layout has elegant whitespace framing
- [ ] Typecheck/lint passes
- [ ] Verify improvements across multiple themes

---

### US-013: Improve Mermaid diagram rendering
**Description:** As a developer, I want Mermaid diagrams to render beautifully within each theme so diagrams feel native to the presentation.

**Acceptance Criteria:**
- [ ] Mermaid theme mappings refined for each tap theme
- [ ] Diagram colors complement slide theme
- [ ] Diagram text is readable at presentation scale
- [ ] Flowcharts, sequence diagrams, etc. all render well
- [ ] Error states are handled gracefully with styled fallback
- [ ] Typecheck/lint passes
- [ ] Verify diagrams in relevant example templates

---

### US-014: Improve code block syntax highlighting
**Description:** As a developer, I want code blocks to have refined syntax highlighting that looks stunning in each theme.

**Acceptance Criteria:**
- [ ] Highlighting theme matches or complements slide theme
- [ ] Line numbers (if shown) are subtle but readable
- [ ] Code font size is readable at presentation distance
- [ ] Line highlighting stands out appropriately
- [ ] Diff highlighting (additions/deletions) is clear
- [ ] Code block backgrounds integrate with slide design
- [ ] Typecheck/lint passes
- [ ] Verify code blocks in `code-demo.md` look polished

---

### US-015: Review and polish example template content
**Description:** As a content creator, I want the example templates to showcase tap's capabilities with well-crafted content so they serve as excellent examples.

**Acceptance Criteria:**
- [ ] `basic.md` demonstrates fundamentals clearly
- [ ] `code-demo.md` showcases code features effectively
- [ ] `sql-demo.md` demonstrates SQL capabilities
- [ ] Theme templates (`theme-*.md`) showcase their respective themes well
- [ ] Content is free of typos and formatting issues
- [ ] Examples demonstrate best practices for slide design
- [ ] Typecheck/lint passes

---

### US-016: Final visual QA pass
**Description:** As a QA engineer, I want to run the complete visual test suite and verify all templates pass with stunning results.

**Acceptance Criteria:**
- [ ] All templates render without errors
- [ ] Visual gallery report generated successfully
- [ ] All 5 themes look polished and professional
- [ ] All layouts render correctly across themes
- [ ] Mermaid diagrams render correctly
- [ ] Code blocks highlight correctly
- [ ] No visual regressions from baseline
- [ ] Gallery report reviewed and approved

---

## Functional Requirements

- FR-1: Playwright must be installed and configured for visual testing
- FR-2: Test runner must discover and iterate all `.md` files in `examples/`
- FR-3: Test runner must start `tap dev --headless` and wait for server ready
- FR-4: Playwright must capture screenshots at 1920x1080 viewport
- FR-5: Screenshots must be named `{template}-slide-{number}.png`
- FR-6: Fragment states must each get a separate screenshot
- FR-7: Console errors must be captured and cause test failure
- FR-8: HTML gallery report must be generated with all screenshots
- FR-9: Gallery must be organized by template with summary statistics
- FR-10: Theme CSS files must be updated to achieve polished designs
- FR-11: `layouts.css` must be updated for improved typography and spacing
- FR-12: Mermaid theme mappings must be refined in `mermaid.ts`
- FR-13: Code highlighting must be reviewed in `highlighting.ts`

## Non-Goals

- No cross-browser testing (Chromium only is sufficient)
- No mobile viewport testing (desktop presentation focus)
- No automated visual diff/regression comparison (manual review via gallery)
- No CI/CD integration (local testing workflow only)
- No performance testing or benchmarking
- No accessibility automation beyond color contrast checks

## Design Considerations

### Screenshot Specifications
- Viewport: 1920x1080 (standard presentation resolution)
- Format: PNG for lossless quality
- Full page: No (viewport only, as slides are single-screen)

### Gallery Report Design
- Clean, minimal HTML layout
- Grid display of screenshots
- Filter/navigation by template
- Dark mode friendly (meta theme-color)

### Visual Design Principles
- **Typography:** Clear hierarchy, comfortable reading at distance
- **Spacing:** Generous whitespace, balanced layouts
- **Color:** High contrast, theme-appropriate palettes
- **Polish:** Subtle shadows, refined borders, smooth transitions

## Technical Considerations

### Dependencies
- Playwright (dev dependency)
- Possibly a simple HTML template generator for gallery

### File Structure
```
tests/
  visual/
    templates.spec.ts    # Main test file
    gallery/
      index.html         # Generated gallery report
      screenshots/       # Screenshot output directory
```

### Test Execution Flow
1. Discover templates in `examples/`
2. For each template:
   a. Start `tap dev --headless --port <dynamic>`
   b. Wait for server ready
   c. Open Playwright browser
   d. Navigate through all slides
   e. Capture screenshot at each slide/fragment
   f. Collect any console errors
   g. Stop server
3. Generate gallery report
4. Report results (pass/fail, error summary)

### Theme Files to Modify
- `frontend/src/lib/themes/paper.css`
- `frontend/src/lib/themes/noir.css`
- `frontend/src/lib/themes/aurora.css`
- `frontend/src/lib/themes/phosphor.css`
- `frontend/src/lib/themes/poster.css`
- `frontend/src/lib/styles/layouts.css`
- `frontend/src/lib/utils/mermaid.ts`
- `frontend/src/lib/utils/highlighting.ts`

## Success Metrics

- 100% of example templates render without console errors
- Visual gallery showcases all templates in polished state
- All 5 themes reviewed and approved as "stunning"
- Typography, spacing, and color improvements measurable via before/after screenshots
- Test suite runs successfully and generates complete gallery

## Open Questions

- Should we establish baseline screenshots for future regression testing?
- What is the threshold for "stunning" - do we need external design review?
- Should gallery report be committed to repo or generated on-demand?
- Do we want to add new example templates beyond the existing ones?
