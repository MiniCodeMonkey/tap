import { mount } from 'svelte'

// Font imports - using direct JS imports for proper Vite bundling
// Inter - Paper theme, Noir body text
import '@fontsource/inter/400.css'
import '@fontsource/inter/500.css'
import '@fontsource/inter/600.css'
import '@fontsource/inter/700.css'
// Playfair Display - Noir headings
import '@fontsource/playfair-display/400.css'
import '@fontsource/playfair-display/700.css'
// Space Grotesk - Aurora theme
import '@fontsource/space-grotesk/400.css'
import '@fontsource/space-grotesk/500.css'
import '@fontsource/space-grotesk/700.css'
// JetBrains Mono - Phosphor theme, code blocks
import '@fontsource/jetbrains-mono/400.css'
import '@fontsource/jetbrains-mono/700.css'
// Anton - Poster headings
import '@fontsource/anton/400.css'
// Noto Serif JP - Ink theme
import '@fontsource/noto-serif-jp/400.css'
import '@fontsource/noto-serif-jp/700.css'
// Bebas Neue - Bauhaus theme
import '@fontsource/bebas-neue/400.css'
// Playfair Display (additional weights) - Editorial theme
import '@fontsource/playfair-display/600.css'
// Source Serif Pro - Editorial theme
import '@fontsource/source-serif-pro/400.css'
import '@fontsource/source-serif-pro/600.css'
import '@fontsource/source-serif-pro/700.css'
// Instrument Sans - Signal theme
import '@fontsource/instrument-sans/400.css'
import '@fontsource/instrument-sans/500.css'
import '@fontsource/instrument-sans/600.css'
import '@fontsource/instrument-sans/700.css'
// IBM Plex Sans - Carbon theme
import '@fontsource/ibm-plex-sans/400.css'
import '@fontsource/ibm-plex-sans/500.css'
import '@fontsource/ibm-plex-sans/600.css'
import '@fontsource/ibm-plex-sans/700.css'
// IBM Plex Mono - Carbon theme
import '@fontsource/ibm-plex-mono/400.css'
import '@fontsource/ibm-plex-mono/500.css'
import '@fontsource/ibm-plex-mono/700.css'
// Sora - Spectrum theme
import '@fontsource/sora/400.css'
import '@fontsource/sora/500.css'
import '@fontsource/sora/600.css'
import '@fontsource/sora/700.css'
import '@fontsource/sora/800.css'
// Fira Code - Spectrum theme
import '@fontsource/fira-code/400.css'
import '@fontsource/fira-code/500.css'
import '@fontsource/fira-code/700.css'
// Outfit - Mono theme
import '@fontsource/outfit/300.css'
import '@fontsource/outfit/400.css'
import '@fontsource/outfit/500.css'
import '@fontsource/outfit/600.css'
import '@fontsource/outfit/700.css'
import '@fontsource/outfit/800.css'
import '@fontsource/outfit/900.css'
// Plus Jakarta Sans - Flux theme
import '@fontsource/plus-jakarta-sans/400.css'
import '@fontsource/plus-jakarta-sans/500.css'
import '@fontsource/plus-jakarta-sans/600.css'
import '@fontsource/plus-jakarta-sans/700.css'
import '@fontsource/plus-jakarta-sans/800.css'

import './app.css'
import Presenter from './Presenter.svelte'

const app = mount(Presenter, {
  target: document.getElementById('app')!,
})

export default app
