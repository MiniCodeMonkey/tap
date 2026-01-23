import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Tap',
  description: 'Presentations for developers',

  srcExclude: ['**/ralph/**'],

  head: [
    ['link', { rel: 'icon', href: '/favicon.ico' }],
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }]
  ],

  themeConfig: {
    nav: [
      { text: 'Getting Started', link: '/getting-started' },
      { text: 'Guide', link: '/guide/writing-slides' },
      { text: 'Reference', link: '/reference/cli-commands' },
      { text: 'Examples', link: '/examples/' },
      { text: 'GitHub', link: 'https://github.com/tap-slides/tap' }
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Guide',
          items: [
            { text: 'Writing Slides', link: '/guide/writing-slides' },
            { text: 'Layouts', link: '/guide/layouts' },
            { text: 'Themes', link: '/guide/themes' },
            { text: 'Animations & Transitions', link: '/guide/animations-transitions' },
            { text: 'Code Blocks', link: '/guide/code-blocks' },
            { text: 'Live Code Execution', link: '/guide/live-code-execution' },
            { text: 'Presenter Mode', link: '/guide/presenter-mode' },
            { text: 'Images & Media', link: '/guide/images-media' },
            { text: 'Building & Export', link: '/guide/building-export' }
          ]
        }
      ],

      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'CLI Commands', link: '/reference/cli-commands' },
            { text: 'Frontmatter Options', link: '/reference/frontmatter-options' },
            { text: 'Slide Directives', link: '/reference/slide-directives' },
            { text: 'Layouts Reference', link: '/reference/layouts-reference' },
            { text: 'Drivers', link: '/reference/drivers' },
            { text: 'Keyboard Shortcuts', link: '/reference/keyboard-shortcuts' }
          ]
        }
      ],

      '/examples/': [
        {
          text: 'Examples',
          items: []
        }
      ]
    }
  }
})
