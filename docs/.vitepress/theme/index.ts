import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import HomeHero from './components/HomeHero.vue'
import HomeFeatures from './components/HomeFeatures.vue'
import HomeCode from './components/HomeCode.vue'
import HomeThemes from './components/HomeThemes.vue'
import CopyButton from './components/CopyButton.vue'
import './style.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('HomeHero', HomeHero)
    app.component('HomeFeatures', HomeFeatures)
    app.component('HomeCode', HomeCode)
    app.component('HomeThemes', HomeThemes)
    app.component('CopyButton', CopyButton)
  }
} satisfies Theme
