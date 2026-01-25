---
title: Themes
---

# Themes

Themes control the visual appearance of your presentation, including typography, colors, animations, and transitions. Tap comes with twenty built-in themes designed for different presentation styles, ranging from clean professional looks to dramatic artistic expressions.

## Setting a Theme

Set the theme in your presentation's frontmatter:

```yaml
---
theme: paper
---
```

The theme applies to all slides in your presentation.

## Built-in Themes

### Paper

![Paper theme preview](/images/themes/theme-paper.png)

Ultra-clean and premium, like a fresh sheet of premium paper where content takes center stage.

```yaml
---
theme: paper
---
```

**Best for:** Professional presentations, corporate settings, content-heavy slides where readability is paramount.

**Characteristics:**
- Pure white background with near-black text
- Inter/system-ui typography with confident letter-spacing
- Warm accent colors (#78716c)
- Dark code blocks (#1e1e1e) with excellent contrast
- Smooth 400ms ease-out transitions

### Noir

![Noir theme preview](/images/themes/theme-noir.png)

Cinematic and sophisticated, drawing from film noir elegance with deep blacks and gold accents.

```yaml
---
theme: noir
---
```

**Best for:** Executive briefings, client pitches, investor meetings, premium product presentations.

**Characteristics:**
- Deep charcoal backgrounds (#0a0a0a)
- Crisp white text (#fafafa)
- Sophisticated gold accent (#d4af37)
- Playfair Display for headings, Inter for body
- Vignette overlay and multi-layer shadows

### Aurora

![Aurora theme preview](/images/themes/theme-aurora.png)

Vibrant and dynamic like the northern lights, with animated gradient mesh and glassmorphism effects.

```yaml
---
theme: aurora
---
```

**Best for:** Startup pitches, creative presentations, product launches, conference talks.

**Characteristics:**
- Animated gradient backgrounds (purple to blue to teal)
- Glassmorphism with backdrop-blur-xl
- Space Grotesk typography
- Semi-transparent dark glass code blocks with cyan glow
- Mesmerizing 20s gradient mesh animation

### Phosphor

![Phosphor theme preview](/images/themes/theme-phosphor.png)

Authentic CRT monitor aesthetic with glowing phosphor green text, scanlines, and retro-futuristic hacker vibes.

```yaml
---
theme: phosphor
---
```

**Best for:** Developer conferences, security talks, technical deep-dives, hacking demos.

**Characteristics:**
- True black (#000) background
- Phosphor green (#00ff00) primary color
- Multi-layer text shadows for glow effect
- Scanline overlay via repeating-linear-gradient
- JetBrains Mono throughout
- Screen curve vignette

### Poster

![Poster theme preview](/images/themes/theme-poster.png)

Bold graphic design with giant typography, thick borders, and high contrast that's impossible to ignore.

```yaml
---
theme: poster
---
```

**Best for:** Design talks, making statements, standing out, architectural presentations.

**Characteristics:**
- Stark black (#000) and white (#fff)
- Electric red accent (#ef4444)
- Anton font for ALL CAPS headings
- Thick 4px borders with 8px 8px 0 drop shadows
- No rounded corners - everything sharp
- Inverted code blocks (white on black)

### Ink

![Ink theme preview](/images/themes/theme-ink.png)

Japanese calligraphy-inspired zen minimalism with brush stroke aesthetics and washi paper texture.

```yaml
---
theme: ink
---
```

**Best for:** Zen presentations, mindfulness talks, Japanese culture topics, minimalist design showcases.

**Characteristics:**
- Cream/off-white background (#f5f1e8) with sumi black text (#1a1a1a)
- Vermillion red accent (#c41e3a) for emphasis
- Noto Serif JP typography for elegant serif styling
- Subtle washi paper texture via CSS gradients
- Brush stroke decorative elements for blockquotes and dividers
- Hanko seal accent on title slides

### Manuscript

![Manuscript theme preview](/images/themes/theme-manuscript.png)

Medieval illuminated manuscript aesthetic with aged parchment, ornate flourishes, and gold accents.

```yaml
---
theme: manuscript
---
```

**Best for:** Historical presentations, literature talks, academic lectures, medieval studies.

**Characteristics:**
- Aged parchment background (#f4e4bc) with dark brown text (#2c1810)
- Deep burgundy (#722f37), gold (#c9a227), and royal blue (#1e3a8a) accents
- Cinzel Decorative for display headings, Cinzel for body
- Ornate initial caps styling for first letters
- Decorative borders with gold flourish accents
- Aged paper texture effect

### Deco

![Deco theme preview](/images/themes/theme-deco.png)

Art Deco glamour from the 1920s with geometric luxury, gold accents, and sunburst patterns.

```yaml
---
theme: deco
---
```

**Best for:** Luxury brand presentations, Gatsby-themed events, design history talks, glamorous reveals.

**Characteristics:**
- Deep black background (#0a0a0a) with cream text (#f5f0e1)
- Gold (#d4af37), champagne (#f7e7ce), and emerald (#047857) accents
- Poiret One for display headings, Bodoni Moda for body
- Geometric sunburst and chevron patterns
- Thin gold line decorations and symmetrical layouts
- Art deco corner ornaments

### Stained Glass

![Stained Glass theme preview](/images/themes/theme-stained-glass.png)

Gothic cathedral aesthetic with jewel tones, gold leading lines, and luminous glow effects.

```yaml
---
theme: stained-glass
---
```

**Best for:** Religious presentations, art history talks, architecture discussions, dramatic storytelling.

**Characteristics:**
- Deep navy/black background (#0c1222) with white text (#f8fafc)
- Jewel tones: ruby (#be123c), sapphire (#1d4ed8), emerald (#047857), amber (#d97706)
- Gold leading lines (#c9a227) for borders and dividers
- Uncial Antiqua for display headings, Crimson Pro for body
- Luminous glow effects on colored elements
- Rose window decorative effects

### Bauhaus

![Bauhaus theme preview](/images/themes/theme-bauhaus.png)

Geometric modernism with bold primary colors and constructivist design principles.

```yaml
---
theme: bauhaus
---
```

**Best for:** Design school presentations, modernism discussions, architecture talks, bold statements.

**Characteristics:**
- Stark white background (#ffffff) with black text (#000000)
- Primary colors only: red (#e53935), yellow (#fdd835), blue (#1e88e5)
- Bebas Neue geometric sans-serif typography
- Bold geometric shapes as decorative elements
- Thick black borders with sharp corners
- Asymmetric grid-based layouts

### Watercolor

![Watercolor theme preview](/images/themes/theme-watercolor.png)

Soft, organic, hand-painted aesthetic with gentle color washes and artistic flowing edges.

```yaml
---
theme: watercolor
---
```

**Best for:** Creative presentations, art workshops, wedding/event planning, gentle storytelling.

**Characteristics:**
- Soft white background (#fefefe) with dark gray text (#374151)
- Soft wash colors: blush (#fce4ec), sage (#e8f5e9), sky (#e3f2fd), lavender (#ede7f6)
- Cormorant Garamond elegant serif typography
- Soft bleeding edge effects via CSS gradients and blur
- Paint splatter and drip accents
- Organic color transitions

### Comic

![Comic theme preview](/images/themes/theme-comic.png)

Pop art comic book aesthetic with halftone dots, bold outlines, and speech bubble styling.

```yaml
---
theme: comic
---
```

**Best for:** Fun presentations, youth-oriented content, creative pitches, pop culture talks.

**Characteristics:**
- White background (#ffffff) with black text (#000000)
- Bold primaries: red (#ef4444), yellow (#facc15), blue (#3b82f6)
- Bangers for explosive headings, Comic Neue for body
- Halftone dot pattern backgrounds
- Thick black outlines (3-4px) on elements
- Speech bubble styling for blockquotes

### Blueprint

![Blueprint theme preview](/images/themes/theme-blueprint.png)

Technical architectural drawing aesthetic with grid lines, dimension markers, and engineering precision.

```yaml
---
theme: blueprint
---
```

**Best for:** Engineering presentations, architecture talks, technical specifications, construction projects.

**Characteristics:**
- Blueprint blue background (#1e3a5f) with white/cyan text (#e0f2fe)
- Grid lines in lighter blue via repeating-linear-gradient
- Orange (#f97316) for annotation accents
- Share Tech Mono for code, Oswald for headings
- Dimension line styling for borders
- Technical annotation callouts

### Editorial

![Editorial theme preview](/images/themes/theme-editorial.png)

Classic magazine publishing design with elegant typography and sophisticated layout.

```yaml
---
theme: editorial
---
```

**Best for:** Publishing talks, journalism presentations, content strategy, brand storytelling.

**Characteristics:**
- Crisp white background (#ffffff) with true black text (#000000)
- Single spot color: deep burgundy (#7f1d1d)
- Playfair Display for headlines, Source Serif Pro for body
- Drop cap styling for first paragraphs
- Fine hairline rules (1px borders)
- Large quotation marks for pull quotes

### Synthwave

![Synthwave theme preview](/images/themes/theme-synthwave.png)

Neon 80s retro-futurism with glowing colors, chrome effects, and perspective grid horizons.

```yaml
---
theme: synthwave
---
```

**Best for:** Retro gaming events, 80s nostalgia content, music presentations, futuristic themes.

**Characteristics:**
- Deep purple/black gradient background (#0f0a1e to #1a0a2e)
- Neon colors: hot pink (#ff2d95), electric cyan (#00fff5), orange (#ff6b35)
- Orbitron for display headings, Audiowide for accents
- Multi-layer neon glow effects
- Perspective grid horizon pattern
- Chrome metallic text effects

### Safari

![Safari theme preview](/images/themes/theme-safari.png)

Victorian exploration aesthetic with aged parchment, compass motifs, and expedition journal styling.

```yaml
---
theme: safari
---
```

**Best for:** Travel presentations, exploration stories, history talks, adventure-themed content.

**Characteristics:**
- Aged cream/sepia background (#f5f0e1) with dark brown text (#3d2914)
- Forest green (#2d5016), brass/copper (#b8860b), burgundy (#6b1c23) accents
- Spectral for headings, Special Elite (typewriter) for accents
- Vintage map texture effects
- Compass rose decorative elements
- Dotted travel line borders and aged paper effects

### Botanical

![Botanical theme preview](/images/themes/theme-botanical.png)

Victorian scientific illustration aesthetic with specimen labels and naturalist elegance.

```yaml
---
theme: botanical
---
```

**Best for:** Science presentations, nature topics, museum talks, botanical studies, natural history.

**Characteristics:**
- Cream/ivory background (#faf8f5) with sepia-toned text (#3d3225)
- Muted greens (#4a5d4a), soft florals (#c9a9a9), sepia (#8b7355) accents
- Libre Baskerville for headings, EB Garamond for body
- Delicate line borders and frames
- Specimen label styling for captions
- Botanical flourish decorations

### Cyber

![Cyber theme preview](/images/themes/theme-cyber.png)

Cyberpunk dystopian tech aesthetic with glitch effects, scan lines, and angular UI elements.

```yaml
---
theme: cyber
---
```

**Best for:** Cybersecurity talks, tech conferences, sci-fi presentations, hacker culture content.

**Characteristics:**
- Near-black background (#0a0a0f) with white text (#f0f0f0)
- Electric blue (#00d4ff), warning yellow (#ffcc00), glitch magenta (#ff00ff)
- Rajdhani for headings, Share Tech Mono for code
- Subtle glitch effects via CSS animations
- Scan line overlay and holographic borders
- Angular cut-corner shapes via clip-path

### Origami

![Origami theme preview](/images/themes/theme-origami.png)

Clean paper craft aesthetic with folded paper effects, subtle shadows, and geometric precision.

```yaml
---
theme: origami
---
```

**Best for:** Minimalist presentations, Japanese culture, design talks, creative workshops.

**Characteristics:**
- Pure white background (#ffffff) with dark gray text (#1f2937)
- Subtle shadows for depth, context-aware accent colors
- Quicksand clean geometric sans-serif typography
- Folded paper corner effects via CSS triangles
- Layered paper plane effects with drop shadows
- Clean angles and geometric precision

### Chalkboard

![Chalkboard theme preview](/images/themes/theme-chalkboard.png)

Friendly classroom aesthetic with chalk typography, dusty texture, and hand-drawn borders.

```yaml
---
theme: chalkboard
---
```

**Best for:** Educational presentations, teacher talks, workshop sessions, friendly tutorials.

**Characteristics:**
- Dark green chalkboard background (#2d4a3e)
- White and pastel chalk colors: yellow (#fff59d), pink (#f8bbd9), blue (#90caf9)
- Caveat and Patrick Hand for handwritten feel
- Chalk dust texture via CSS gradients
- Sketchy underlines and hand-drawn borders
- Dashed chalk-like lines

## What Themes Control

Each theme defines:

| Aspect | Description |
|--------|-------------|
| **Typography** | Font families, sizes, weights, and line heights for headings, body text, and code |
| **Colors** | Background colors, text colors, accent colors, and syntax highlighting palette |
| **Animations** | How elements appear on slides (fade, slide, bounce, etc.) |
| **Transitions** | How slides transition between each other (fade, push, slide, zoom) |
| **Spacing** | Padding, margins, and overall layout density |
| **Code styling** | Code block appearance, syntax highlighting colors, and font sizing |

## Theme Reference Table

### Original Themes

| Theme | Vibe | Background | Typography |
|-------|------|------------|------------|
| `paper` | Ultra-clean, premium | Light (#ffffff) | Inter/system-ui |
| `noir` | Cinematic, sophisticated | Dark (#0a0a0a) | Playfair Display + Inter |
| `aurora` | Vibrant, dynamic | Animated gradient | Space Grotesk |
| `phosphor` | CRT, hacker aesthetic | Black (#000) | JetBrains Mono |
| `poster` | Bold, graphic | High contrast | Anton + system sans |

### Cultural Themes

| Theme | Vibe | Background | Typography |
|-------|------|------------|------------|
| `ink` | Zen, calligraphy | Cream (#f5f1e8) | Noto Serif JP |
| `manuscript` | Medieval, illuminated | Parchment (#f4e4bc) | Cinzel Decorative + Cinzel |
| `safari` | Victorian explorer | Sepia (#f5f0e1) | Spectral + Special Elite |
| `botanical` | Scientific illustration | Ivory (#faf8f5) | Libre Baskerville + EB Garamond |

### Art Movement Themes

| Theme | Vibe | Background | Typography |
|-------|------|------------|------------|
| `deco` | Art Deco glamour | Black (#0a0a0a) | Poiret One + Bodoni Moda |
| `stained-glass` | Gothic cathedral | Navy (#0c1222) | Uncial Antiqua + Crimson Pro |
| `bauhaus` | Geometric modernism | White (#ffffff) | Bebas Neue |
| `watercolor` | Artistic, painterly | White (#fefefe) | Cormorant Garamond |
| `comic` | Pop art | White (#ffffff) | Bangers + Comic Neue |

### Technical Themes

| Theme | Vibe | Background | Typography |
|-------|------|------------|------------|
| `blueprint` | Engineering | Blue (#1e3a5f) | Oswald + Share Tech Mono |
| `editorial` | Magazine publishing | White (#ffffff) | Playfair Display + Source Serif Pro |
| `cyber` | Cyberpunk | Near-black (#0a0a0f) | Rajdhani + Share Tech Mono |

### Retro & Modern Themes

| Theme | Vibe | Background | Typography |
|-------|------|------------|------------|
| `synthwave` | 80s neon | Purple gradient | Orbitron + Audiowide |
| `origami` | Paper craft | White (#ffffff) | Quicksand |
| `chalkboard` | Classroom | Green (#2d4a3e) | Caveat + Patrick Hand |

## Customizing Themes

### Color Overrides

Override theme colors using `themeColors` in frontmatter:

```yaml
---
theme: paper
themeColors:
  accent: "#ff0000"
  background: "#f5f5f5"
---
```

Available color keys: `background`, `text`, `muted`, `accent`, `codeBg`

### Custom Theme CSS

For complete customization, create your own theme CSS file:

```yaml
---
customTheme: "./my-theme.css"
---
```

Your CSS file should define these CSS custom properties:

```css
.theme-custom {
  --color-bg: #ffffff;
  --color-text: #0a0a0a;
  --color-muted: #71717a;
  --color-accent: #3b82f6;
  --color-code-bg: #1e1e1e;
  --font-sans: Inter, system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', monospace;
}
```
