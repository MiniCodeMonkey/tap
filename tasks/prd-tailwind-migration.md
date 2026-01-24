# PRD: TailwindCSS Migration & Theme Redesign

## Introduction

Tap's current CSS architecture uses hand-written custom CSS files (~1500+ lines across 5 themes). This PRD covers a complete migration to TailwindCSS and a ground-up redesign of all presentation themes to be **bold, distinctive, and visually stunning** out of the box.

The goal is not just a technical migration—it's a visual transformation. Tap presentations should make audiences say "wow, what tool is that?" within the first 3 slides.

## Goals

- **Stunning by default**: Every theme should be design-forward, opinionated, and memorable
- **100% Tailwind**: Zero custom CSS files; all styling via Tailwind utilities
- **Faster iteration**: Utility-first approach for rapid theme development
- **Design consistency**: Shared design tokens across all themes via Tailwind config
- **Flexible theming**: Simple color swaps for built-in themes, full customization for power users
- **Animation escape hatch**: Svelte transitions + minimal custom CSS only where Tailwind can't reach

## User Stories

---

### US-101: Install and configure TailwindCSS in frontend build

**Description:** As a developer, I need TailwindCSS integrated into the Vite/Svelte build process so I can use utility classes throughout the application.

**Acceptance Criteria:**
- [ ] Install `tailwindcss`, `postcss`, `autoprefixer` as dev dependencies
- [ ] Create `tailwind.config.js` with content paths for all Svelte/TS files
- [ ] Create `postcss.config.js` with Tailwind and Autoprefixer plugins
- [ ] Add Tailwind directives (`@tailwind base/components/utilities`) to main CSS entry
- [ ] Verify utility classes work in a test component (e.g., `class="text-red-500"`)
- [ ] Build completes successfully with no CSS errors
- [ ] Typecheck passes

---

### US-102: Configure Tailwind for presentation-specific design tokens

**Description:** As a developer, I need Tailwind configured with presentation-appropriate scales so themes can use consistent, large typography and spacing.

**Acceptance Criteria:**
- [ ] Extend `fontSize` with presentation scale: `title` (8rem), `h1` (6rem), `h2` (4rem), `h3` (2.5rem), `body` (2rem), `code` (1.5rem), `stat` (12rem)
- [ ] Extend `fontFamily` with `sans`, `mono`, and `display` stacks
- [ ] Extend `spacing` for slide-appropriate values (slide padding, content gaps)
- [ ] Add `aspectRatio` utilities for 16:9, 4:3, 16:10
- [ ] Configure animation/transition timing tokens
- [ ] Add CSS custom property bridge for theme switching (`--color-primary`, etc.)
- [ ] Typecheck passes

---

### US-103: Create theme configuration system with CSS custom properties

**Description:** As a developer, I need a theming system where themes define CSS custom properties that Tailwind utilities reference, enabling runtime theme switching.

**Acceptance Criteria:**
- [ ] Create `src/lib/themes/theme-tokens.css` defining CSS variable structure
- [ ] Define required variables: `--color-bg`, `--color-text`, `--color-muted`, `--color-accent`, `--color-code-bg`, `--font-sans`, `--font-mono`
- [ ] Configure Tailwind to use these variables (e.g., `bg-theme-bg`, `text-theme-accent`)
- [ ] Theme class on root element (`.theme-paper`) sets the variable values
- [ ] Switching theme class instantly updates all colors
- [ ] Typecheck passes
- [ ] Verify in browser: switching theme class changes colors immediately

---

### US-104: Remove all legacy custom CSS files

**Description:** As a developer, I need to remove the old CSS architecture so we have a clean slate for the Tailwind migration.

**Acceptance Criteria:**
- [ ] Archive existing theme CSS files to `frontend/src/lib/themes/_legacy/` (for reference during redesign)
- [ ] Remove imports of legacy CSS files from components
- [ ] Remove any component-level `<style>` blocks with custom CSS (except animations)
- [ ] Application still builds (may look broken—that's expected)
- [ ] Typecheck passes

---

### US-105: Migrate SlideContainer component to Tailwind

**Description:** As a developer, I need the core slide container using Tailwind so it properly scales and centers slides.

**Acceptance Criteria:**
- [ ] Replace all custom CSS in SlideContainer.svelte with Tailwind utilities
- [ ] Maintain aspect ratio scaling (16:9, 4:3, 16:10) using Tailwind
- [ ] Center slide in viewport with flex utilities
- [ ] Support fullscreen mode
- [ ] Handle window resize correctly
- [ ] Typecheck passes
- [ ] Verify in browser: slide scales correctly at different window sizes

---

### US-106: Migrate SlideRenderer component to Tailwind

**Description:** As a developer, I need the slide renderer using Tailwind for content display and fragment visibility.

**Acceptance Criteria:**
- [ ] Replace all custom CSS with Tailwind utilities
- [ ] Layout-specific classes applied via Tailwind
- [ ] Fragment visibility controlled via Tailwind opacity/transform utilities
- [ ] Background image support maintained
- [ ] Typecheck passes
- [ ] Verify in browser: slides render with correct layout structure

---

### US-107: Migrate all layout components to Tailwind

**Description:** As a developer, I need all layout components (Title, Section, Default, TwoColumn, CodeFocus, Quote, BigStat, etc.) using Tailwind.

**Acceptance Criteria:**
- [ ] LayoutTitle.svelte - centered title with subtitle using Tailwind flex/text utilities
- [ ] LayoutSection.svelte - large section header using Tailwind
- [ ] LayoutDefault.svelte - standard content layout using Tailwind
- [ ] LayoutTwoColumn.svelte - CSS grid via Tailwind
- [ ] LayoutCodeFocus.svelte - full-width code using Tailwind
- [ ] LayoutQuote.svelte - styled blockquote using Tailwind
- [ ] LayoutBigStat.svelte - large number emphasis using Tailwind
- [ ] LayoutThreeColumn.svelte - three-column grid using Tailwind
- [ ] LayoutCover.svelte - full-bleed background using Tailwind
- [ ] LayoutSidebar.svelte - sidebar layout using Tailwind
- [ ] LayoutSplitMedia.svelte - image + text using Tailwind
- [ ] LayoutBlank.svelte - empty canvas using Tailwind
- [ ] All layouts use theme CSS variables via Tailwind utilities
- [ ] Typecheck passes
- [ ] Verify in browser: each layout renders correctly

---

### US-108: Migrate typography and content styling to Tailwind

**Description:** As a developer, I need base typography (headings, paragraphs, lists, links, code, blockquotes, tables) styled via Tailwind.

**Acceptance Criteria:**
- [ ] Create `src/lib/styles/prose.css` with Tailwind `@apply` for markdown content
- [ ] Style h1-h6 with appropriate sizes from presentation scale
- [ ] Style paragraphs and list items with proper spacing
- [ ] Style inline code with background and padding
- [ ] Style code blocks (pre) with theme-aware backgrounds
- [ ] Style blockquotes with accent border
- [ ] Style tables with proper borders and padding
- [ ] Style links with accent color and hover states
- [ ] All styles use theme CSS variables
- [ ] Typecheck passes
- [ ] Verify in browser: markdown content renders beautifully

---

### US-109: Migrate UI components to Tailwind

**Description:** As a developer, I need all UI components (ProgressBar, ConnectionIndicator, LiveCodeBlock, etc.) using Tailwind.

**Acceptance Criteria:**
- [ ] ProgressBar.svelte - thin accent-colored bar using Tailwind
- [ ] ConnectionIndicator.svelte - subtle corner indicator using Tailwind
- [ ] LiveCodeBlock.svelte - code display with run button using Tailwind
- [ ] FragmentContainer.svelte - fragment wrapper using Tailwind
- [ ] SlideOverview.svelte - thumbnail grid using Tailwind
- [ ] AsciinemaPlayer.svelte - player wrapper using Tailwind
- [ ] All components use theme CSS variables
- [ ] Typecheck passes
- [ ] Verify in browser: all components render correctly

---

### US-110: Migrate presenter view to Tailwind

**Description:** As a developer, I need the presenter view page using Tailwind for its multi-panel layout.

**Acceptance Criteria:**
- [ ] Main layout with current slide, next preview, notes panel using Tailwind grid
- [ ] Timer display styled with Tailwind
- [ ] Slide counter styled with Tailwind
- [ ] Touch-friendly controls sized appropriately
- [ ] Responsive for tablet use
- [ ] Typecheck passes
- [ ] Verify in browser: presenter view is functional and well-styled

---

### US-111: Design and implement stunning "Paper" theme

**Description:** As a user, I want the Paper theme to feel ultra-clean and airy—like a fresh sheet of premium paper where content takes center stage with invisible design.

**Acceptance Criteria:**
- [ ] Create `src/lib/themes/paper.css` with CSS custom properties only (no utility classes)
- [ ] Color palette: Pure white background (#ffffff), near-black text (#0a0a0a), sophisticated gray for muted (#71717a), subtle warm accent
- [ ] Typography: Inter / SF Pro for both headings and body—confident letter-spacing, perfect line heights, generous whitespace
- [ ] Subtle refinements: Delicate shadows on code blocks, refined border radii (but minimal), lots of breathing room
- [ ] Code blocks: Dark with excellent contrast, clean edges
- [ ] Transitions: Smooth, confident fades (no flashy animations)
- [ ] Distinctive feature: The restraint IS the design—every pixel intentional, content-first
- [ ] Typecheck passes
- [ ] Verify in browser: theme looks premium and effortlessly clean

---

### US-112: Design and implement stunning "Aurora" theme

**Description:** As a user, I want the Aurora theme to feel like the northern lights—vibrant color flows, glassmorphism, dynamic and mesmerizing.

**Acceptance Criteria:**
- [ ] Create `src/lib/themes/aurora.css` with CSS custom properties only
- [ ] Color palette: Rich gradient backgrounds (deep purples → electric blues → vibrant teals → subtle pinks), white/light text
- [ ] Background: Animated gradient mesh or layered aurora-like color flows with subtle movement
- [ ] Glassmorphism: Frosted glass cards for content areas with backdrop-blur
- [ ] Typography: Modern geometric sans-serif (Space Grotesk, Outfit, or similar)
- [ ] Code blocks: Semi-transparent dark glass with colored syntax highlighting
- [ ] Accent elements: Glowing borders, subtle light leaks, color bleeds
- [ ] Distinctive feature: Feels alive—like presenting inside a design tool (Linear/Figma aesthetic)
- [ ] Typecheck passes
- [ ] Verify in browser: theme is vibrant, dynamic, and modern

---

### US-113: Design and implement stunning "Phosphor" theme

**Description:** As a user, I want the Phosphor theme to feel like authentic CRT monitors—glowing phosphor text, scanlines, retro-futuristic hacker aesthetic.

**Acceptance Criteria:**
- [ ] Create `src/lib/themes/phosphor.css` with CSS custom properties only
- [ ] Color palette: True black (#000) background, phosphor green (#00ff00) as primary, with amber (#ffb000) variant option, dim gray for muted
- [ ] CRT effects: Subtle scanlines overlay, slight screen curve vignette, phosphor glow/bloom on text
- [ ] Typography: JetBrains Mono or IBM Plex Mono throughout—authentic terminal feel, no sans-serif
- [ ] Text effects: Multi-layer text-shadow for authentic phosphor glow, optional typing cursor blink
- [ ] Code blocks: No background change needed—everything IS terminal
- [ ] Boot sequence: Optional slide transition that mimics terminal startup/text rendering
- [ ] Distinctive feature: Feels like presenting from inside a mainframe—the screen glows
- [ ] Typecheck passes
- [ ] Verify in browser: theme has authentic retro-tech CRT feel

---

### US-114: Design and implement stunning "Poster" theme

**Description:** As a user, I want the Poster theme to feel like bold graphic design—giant typography, high contrast, impossible to ignore, like a protest poster or album cover.

**Acceptance Criteria:**
- [ ] Create `src/lib/themes/poster.css` with CSS custom properties only
- [ ] Color palette: Stark black and white, with ONE bold accent color (electric red, safety yellow, or hot pink)
- [ ] Typography: Heavy, condensed sans-serif (Anton, Bebas Neue, or similar), ALL CAPS headings, massive scale
- [ ] Layout elements: Thick black borders, harsh drop shadows (offset, no blur), geometric blocks
- [ ] No rounded corners: Everything sharp and rectangular—brutalist influence
- [ ] Code blocks: Inverted colors, monospace with visible structure
- [ ] Intentional "roughness": Slight asymmetry, overlapping elements, broken grid moments for visual tension
- [ ] Distinctive feature: Confrontational and memorable—your slides demand attention
- [ ] Typecheck passes
- [ ] Verify in browser: theme is bold, graphic, and striking

---

### US-115: Design and implement stunning "Noir" theme

**Description:** As a user, I want the Noir theme to feel cinematic and sophisticated—deep blacks, dramatic lighting, film noir elegance for executive presentations.

**Acceptance Criteria:**
- [ ] Create `src/lib/themes/noir.css` with CSS custom properties only
- [ ] Color palette: Deep charcoal/near-black backgrounds (#0a0a0a), crisp white text, sophisticated accent (gold, champagne, or cool blue)
- [ ] Typography: Classic, authoritative serifs for titles (Playfair Display, Freight), clean sans-serif for body
- [ ] Cinematic depth: Multi-layer soft shadows, subtle vignette, sense of dimension
- [ ] Subtle gradients: Very subtle dark-to-darker gradients for richness and depth
- [ ] Code blocks: Elevated card style with refined shadows, slight glow
- [ ] Film grain: Optional subtle noise texture for authentic film aesthetic
- [ ] Distinctive feature: Cinematic confidence—your content feels important and dramatic
- [ ] Typecheck passes
- [ ] Verify in browser: theme looks sophisticated, dark, and executive

---

### US-116: Implement simple color customization for built-in themes

**Description:** As a user, I want to easily swap colors in built-in themes via frontmatter so I can match my brand without building a custom theme.

**Acceptance Criteria:**
- [ ] Support `themeColors` object in frontmatter: `{ accent: "#ff0000", background: "#000" }`
- [ ] Parse and apply color overrides as inline CSS variables on root element
- [ ] Document supported color keys: `background`, `text`, `muted`, `accent`, `codeBg`
- [ ] Partial overrides work (only specify what you want to change)
- [ ] Invalid colors ignored gracefully with console warning
- [ ] Typecheck passes
- [ ] Verify in browser: custom colors applied correctly

---

### US-117: Implement custom theme support via user CSS file

**Description:** As a power user, I want to define a completely custom theme by providing my own CSS file so I have full control over the visual design.

**Acceptance Criteria:**
- [ ] Support `customTheme: "./my-theme.css"` in frontmatter (path relative to markdown file)
- [ ] Load and inject custom CSS file at runtime (dev mode) or embed at build time
- [ ] Custom theme CSS can define all CSS variables to completely override defaults
- [ ] Custom theme can include additional utility classes or styles
- [ ] Document the CSS variable contract that custom themes should implement
- [ ] Error handling: graceful fallback if custom theme file not found
- [ ] Typecheck passes
- [ ] Verify in browser: custom theme fully applied

---

### US-118: Create animation utilities with Tailwind + Svelte transitions

**Description:** As a developer, I need a clean approach for animations that combines Tailwind utilities with Svelte's transition system for the effects Tailwind can't handle.

**Acceptance Criteria:**
- [ ] Create `src/lib/styles/animations.css` for complex animations only (CRT effects, glows, etc.)
- [ ] Keep this file minimal—only what Tailwind truly cannot do
- [ ] Document which animations are Tailwind (opacity, transform, scale) vs custom
- [ ] Svelte transitions used for slide transitions (fade, fly, scale)
- [ ] Fragment reveals use Tailwind opacity/transform with CSS transition
- [ ] Theme-specific animations (terminal glow, gradient mesh) isolated to theme files
- [ ] Typecheck passes
- [ ] Verify in browser: all animations work smoothly

---

### US-119: Update embedded asset build to include Tailwind output

**Description:** As a developer, I need the Go binary to embed the Tailwind-built CSS so the single binary distribution works correctly.

**Acceptance Criteria:**
- [ ] Vite build outputs optimized CSS with Tailwind utilities
- [ ] CSS is properly purged (only used utilities included)
- [ ] Output CSS copied to `../embedded/` directory
- [ ] Go embed directive picks up the new CSS
- [ ] Binary serves correct styles
- [ ] Build size is reasonable (target: <50KB CSS gzipped)
- [ ] Typecheck passes

---

### US-120: Update documentation and examples for new theming system

**Description:** As a user, I need documentation explaining how to use themes, customize colors, and create custom themes.

**Acceptance Criteria:**
- [ ] Update SPEC.md with new theming architecture
- [ ] Document all 5 themes with visual descriptions and recommended use cases
- [ ] Document `themeColors` frontmatter option with examples
- [ ] Document custom theme CSS file approach with template
- [ ] Update example presentations to showcase new themes
- [ ] Add theme showcase slide to testdata/sample.md
- [ ] Typecheck passes

---

### US-121: Rename theme references in Go backend

**Description:** As a developer, I need all Go code updated to use the new theme names (paper, noir, aurora, phosphor, poster) instead of the old names (minimal, gradient, terminal, brutalist, keynote).

**Acceptance Criteria:**
- [ ] Update `internal/config/config.go` theme validation to accept new names
- [ ] Update any theme constants or enums
- [ ] Update default theme from "minimal" to "paper"
- [ ] Update theme list in CLI help text (`tap new --help`)
- [ ] Update TUI theme selector options in `internal/tui/new.go`
- [ ] Add backwards compatibility: old names map to new names with deprecation warning
- [ ] All Go tests pass with new theme names
- [ ] Typecheck passes

---

### US-122: Rename theme files and references in frontend

**Description:** As a developer, I need all frontend code updated to use the new theme names.

**Acceptance Criteria:**
- [ ] Rename CSS files: `minimal.css` → `paper.css`, `keynote.css` → `noir.css`, `gradient.css` → `aurora.css`, `terminal.css` → `phosphor.css`, `brutalist.css` → `poster.css`
- [ ] Update all CSS class references (`.theme-minimal` → `.theme-paper`, etc.)
- [ ] Update theme imports in components
- [ ] Update TypeScript types/enums for theme names
- [ ] Update any hardcoded theme references in Svelte components
- [ ] Typecheck passes
- [ ] Verify in browser: all themes load correctly with new names

---

### US-123: Update sample presentations and test data

**Description:** As a developer, I need all sample presentations updated to use the new theme names.

**Acceptance Criteria:**
- [ ] Update `testdata/sample.md` frontmatter to use new theme name
- [ ] Update any other test fixtures referencing old theme names
- [ ] Update example presentations in `examples/` directory (if exists)
- [ ] Grep codebase for old theme names and update any remaining references
- [ ] Typecheck passes

---

### US-124: Update SPEC.md with new theme names and descriptions

**Description:** As a developer, I need the product specification updated to reflect the new theme names and their visual directions.

**Acceptance Criteria:**
- [ ] Replace all references to "minimal" with "paper"
- [ ] Replace all references to "keynote" with "noir"
- [ ] Replace all references to "gradient" with "aurora"
- [ ] Replace all references to "terminal" with "phosphor"
- [ ] Replace all references to "brutalist" with "poster"
- [ ] Update theme descriptions to match new visual directions
- [ ] Update any code examples showing theme configuration
- [ ] Typecheck passes

---

### US-125: Add theme switcher to dev server TUI

**Description:** As a presenter, I want to quickly switch themes from the dev server TUI so I can preview how my presentation looks in different themes without editing the markdown file.

**Acceptance Criteria:**
- [ ] Add `t` keyboard shortcut to open theme switcher in `tap dev` TUI
- [ ] Display theme picker with all 5 themes (Paper, Noir, Aurora, Phosphor, Poster)
- [ ] Show theme name and brief description for each option
- [ ] Arrow keys to navigate, Enter to select, Escape to cancel
- [ ] Selecting a theme instantly updates the browser preview via WebSocket
- [ ] Current theme highlighted in the picker
- [ ] Theme change is temporary (doesn't modify markdown file)
- [ ] Display current theme name in TUI status bar
- [ ] Typecheck passes
- [ ] Verify in browser: theme switches instantly when selected in TUI

---

### US-126: Embed custom fonts for offline presentation support

**Description:** As a presenter, I need all fonts embedded so presentations work fully offline without internet access.

**Acceptance Criteria:**
- [ ] Install font packages via npm: `@fontsource/inter`, `@fontsource/space-grotesk`, `@fontsource/jetbrains-mono`, `@fontsource/playfair-display`, `@fontsource/anton`
- [ ] Import required font weights in theme CSS files
- [ ] Paper theme: Inter (400, 500, 600, 700)
- [ ] Noir theme: Playfair Display (400, 700) + Inter (400, 500)
- [ ] Aurora theme: Space Grotesk (400, 500, 700)
- [ ] Phosphor theme: JetBrains Mono (400, 700)
- [ ] Poster theme: Anton (400) + system monospace for code
- [ ] Fonts included in Vite build output
- [ ] Verify presentations render correctly with network disabled
- [ ] Document font licensing in README (all fonts are OFL/open source)
- [ ] Typecheck passes
- [ ] Verify in browser: fonts load correctly offline

---

## Functional Requirements

- **FR-1:** TailwindCSS must be the sole styling solution (no separate CSS files except theme variables and complex animations)
- **FR-2:** All 5 themes must use CSS custom properties for colors, with Tailwind utilities referencing them
- **FR-3:** Theme switching must work at runtime by changing a class on the root element
- **FR-4:** Built-in themes support color customization via `themeColors` frontmatter
- **FR-5:** Power users can provide a custom CSS file for complete theme control
- **FR-6:** All animations must work with `prefers-reduced-motion` respected
- **FR-7:** Final CSS bundle must be optimized and purged of unused utilities
- **FR-8:** Themes must look stunning at both 1080p and 4K resolutions
- **FR-9:** Dev server TUI must include theme switcher (press `t`) for instant preview of all themes
- **FR-10:** All custom fonts must be embedded—presentations must work fully offline

## Non-Goals (Out of Scope)

- **Not building a theme editor UI** - Themes are configured via frontmatter/CSS files
- **Not supporting Tailwind JIT in browser** - All styles are build-time
- **Not creating a theme marketplace** - That's a future phase
- **Not adding new layouts** - Focus is on styling existing layouts beautifully
- **Not supporting CSS-in-JS solutions** - Tailwind + CSS custom properties only

## Design Considerations

### Visual Direction for Each Theme

| Theme | Aesthetic | Inspiration | Key Feature |
|-------|-----------|-------------|-------------|
| **Paper** | Ultra-clean, airy | Fresh sheet, Stripe | Perfect whitespace, invisible design |
| **Noir** | Cinematic, sophisticated | Film noir, executive | Deep blacks, dramatic lighting |
| **Aurora** | Vibrant, dynamic | Northern lights, Linear/Figma | Living gradients, glassmorphism |
| **Phosphor** | Retro-futuristic | CRT monitors, mainframes | Glowing text, scanlines |
| **Poster** | Bold, graphic | Protest posters, album art | Giant type, high contrast |

### Typography Pairings (Embedded via @fontsource)

| Theme | Headings | Body | Package |
|-------|----------|------|---------|
| **Paper** | Inter | Inter | `@fontsource/inter` |
| **Noir** | Playfair Display | Inter | `@fontsource/playfair-display`, `@fontsource/inter` |
| **Aurora** | Space Grotesk | Space Grotesk | `@fontsource/space-grotesk` |
| **Phosphor** | JetBrains Mono | JetBrains Mono | `@fontsource/jetbrains-mono` |
| **Poster** | Anton | System sans | `@fontsource/anton` |

All fonts are embedded for offline support. No external font loading.

### Color Philosophy

Each theme should have:
- A dominant background approach (light, dark, gradient, etc.)
- A clear text hierarchy (primary, secondary, muted)
- One strong accent color used sparingly for emphasis
- Code block treatment that fits the theme personality

## Technical Considerations

### Tailwind Configuration Strategy

```javascript
// tailwind.config.js approach
module.exports = {
  theme: {
    extend: {
      colors: {
        theme: {
          bg: 'var(--color-bg)',
          text: 'var(--color-text)',
          muted: 'var(--color-muted)',
          accent: 'var(--color-accent)',
          // ... etc
        }
      },
      fontSize: {
        'slide-title': ['8rem', { lineHeight: '1.1' }],
        'slide-h1': ['6rem', { lineHeight: '1.1' }],
        // ... etc
      }
    }
  }
}
```

### Theme CSS Structure

Each theme file defines only CSS custom properties:

```css
/* paper.css */
.theme-paper {
  --color-bg: #ffffff;
  --color-text: #0a0a0a;
  --color-muted: #71717a;
  --color-accent: #78716c;
  /* ... */
}
```

### Animation Escape Hatch

Complex animations that Tailwind can't handle:
- CRT scanline overlay (CSS pseudo-element with repeating gradient)
- Phosphor glow (CSS text-shadow with multiple layers)
- Gradient mesh animation (CSS @keyframes)
- Glassmorphism backdrop-blur (Tailwind supports this, but layering may need custom)

These live in `src/lib/styles/animations.css` and are kept minimal.

## Success Metrics

- **Visual impact:** 5 out of 5 developers shown the themes say "that looks professional"
- **Bundle size:** CSS output <50KB gzipped (Tailwind purge working correctly)
- **Migration completeness:** Zero remaining `.css` files except theme variables and animations.css
- **Theme switching:** <16ms to switch themes (instant, no visible delay)
- **Customization:** Users can change accent color in <30 seconds via frontmatter

## Decisions (Resolved Questions)

1. **Font loading strategy:** Embed all custom fonts via npm packages (`@fontsource/*`) so presentations work fully offline. No external font loading.

2. **Aurora animation performance:** Always enabled—the animated background is core to the theme's identity.

3. **Theme preview in CLI:** Name + description is sufficient. No ASCII previews needed.

4. **Theme variants:** No light/dark mode variants. Each theme has one look. Users who want dark can choose Noir or Phosphor.

## Recommended Implementation Order

1. **Phase 1 - Foundation (US-101 → US-104):** Install Tailwind, configure tokens, set up theming system, remove legacy CSS
2. **Phase 2 - Component Migration (US-105 → US-110):** Migrate all components to Tailwind utilities
3. **Phase 3 - Theme Design (US-111 → US-115):** Design and implement all 5 stunning themes (Paper, Noir, Aurora, Phosphor, Poster)
4. **Phase 4 - Customization (US-116 → US-117):** Add color customization and custom theme support
5. **Phase 5 - Polish & Features (US-118 → US-126):** Animations, build optimization, rename old theme references, TUI theme switcher, font embedding, documentation

### Theme Name Migration Map

| Old Name | New Name | Rationale |
|----------|----------|-----------|
| minimal | **paper** | Evokes clean whitespace, content-first |
| keynote | **noir** | Cinematic, sophisticated (avoids Apple trademark) |
| gradient | **aurora** | Northern lights, living color flows |
| terminal | **phosphor** | CRT coating that glows—more evocative |
| brutalist | **poster** | Graphic design impact, more accessible name |
