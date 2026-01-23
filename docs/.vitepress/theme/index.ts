import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import HomeHero from './components/HomeHero.vue'
import HomeFeatures from './components/HomeFeatures.vue'
import CopyButton from './components/CopyButton.vue'
import './style.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('HomeHero', HomeHero)
    app.component('HomeFeatures', HomeFeatures)
    app.component('CopyButton', CopyButton)
  }
} satisfies Theme
