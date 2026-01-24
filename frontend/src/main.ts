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

import './app.css'
import App from './App.svelte'
import { initializeMermaid } from '$lib/utils/mermaid'

// Initialize mermaid for diagram rendering (startOnLoad: false for manual control)
initializeMermaid()

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
