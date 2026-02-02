# Themes

Themes control typography, colors, animations, and transitions.

## Setting a Theme

```yaml
---
theme: paper
---
```

## Built-in Themes

### Paper
Ultra-clean and premium.
```yaml
theme: paper
```
- Pure white background, near-black text
- Inter/system-ui typography
- Warm accent colors (#78716c)
- **Best for:** Professional presentations, corporate settings

### Noir
Cinematic and sophisticated.
```yaml
theme: noir
```
- Deep charcoal backgrounds (#0a0a0a)
- Crisp white text, gold accent (#d4af37)
- Playfair Display headings, Inter body
- **Best for:** Executive briefings, client pitches

### Aurora
Vibrant and dynamic.
```yaml
theme: aurora
```
- Animated gradient backgrounds (purple to blue to teal)
- Glassmorphism with backdrop blur
- Space Grotesk typography
- **Best for:** Startup pitches, creative presentations

### Phosphor
CRT monitor aesthetic.
```yaml
theme: phosphor
```
- True black (#000) background
- Phosphor green (#00ff00) primary
- Scanline overlay effect
- JetBrains Mono throughout
- **Best for:** Developer conferences, security talks

### Poster
Bold graphic design.
```yaml
theme: poster
```
- Stark black and white
- Electric red accent (#ef4444)
- Anton font for ALL CAPS headings
- Thick 4px borders, no rounded corners
- **Best for:** Design talks, making statements

### Ink
Japanese calligraphy-inspired.
```yaml
theme: ink
```
- Cream/off-white background (#f5f1e8)
- Sumi black text, vermillion accent
- Noto Serif JP typography
- **Best for:** Zen presentations, minimalist design

### Bauhaus
Geometric modernism.
```yaml
theme: bauhaus
```
- Stark white background, black text
- Primary colors: red, yellow, blue
- Bebas Neue typography
- **Best for:** Design talks, architecture presentations

### Editorial
Classic magazine publishing.
```yaml
theme: editorial
```
- Crisp white background, true black text
- Deep burgundy accent (#7f1d1d)
- Playfair Display + Source Serif Pro
- **Best for:** Publishing talks, brand storytelling

## Theme Reference

| Theme | Background | Typography |
|-------|------------|------------|
| `paper` | Light (#ffffff) | Inter/system-ui |
| `noir` | Dark (#0a0a0a) | Playfair Display + Inter |
| `aurora` | Animated gradient | Space Grotesk |
| `phosphor` | Black (#000) | JetBrains Mono |
| `poster` | High contrast | Anton + system sans |
| `ink` | Cream (#f5f1e8) | Noto Serif JP |
| `bauhaus` | White (#ffffff) | Bebas Neue |
| `editorial` | White (#ffffff) | Playfair Display + Source Serif Pro |

## What Themes Control

- **Typography:** Font families, sizes, weights
- **Colors:** Background, text, accent, syntax highlighting
- **Animations:** Element appearance effects
- **Transitions:** Slide transition styles
- **Spacing:** Padding, margins, layout density
- **Code styling:** Code block appearance

## Customizing Themes

### Color Overrides
```yaml
---
theme: paper
themeColors:
  accent: "#ff0000"
  background: "#f5f5f5"
---
```

Available keys: `background`, `text`, `muted`, `accent`, `codeBg`

### Custom Theme CSS
```yaml
---
customTheme: "./my-theme.css"
---
```

CSS file:
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

## Choosing a Theme

| Context | Recommended Theme |
|---------|-------------------|
| Corporate/professional | `paper` |
| Executive/premium | `noir` |
| Startup/creative | `aurora` |
| Technical/developer | `phosphor` |
| Design/bold statement | `poster` |
| Minimalist/zen | `ink` |
| Modern/geometric | `bauhaus` |
| Publishing/editorial | `editorial` |
