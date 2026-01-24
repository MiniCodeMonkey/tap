# Tap Documentation

This directory contains the documentation website for Tap, built with [VitePress](https://vitepress.dev/).

## Development

```bash
# Install dependencies
npm install

# Start dev server
npm run docs:dev

# Build for production
npm run docs:build

# Preview production build
npm run docs:preview
```

## Structure

```
docs/
├── .vitepress/       # VitePress configuration
├── guide/            # User guides and tutorials
├── reference/        # API and configuration reference
├── examples/         # Example presentations
├── public/           # Static assets
├── getting-started.md
├── index.md          # Homepage
└── PERFORMANCE.md    # Performance documentation
```

## Adding Documentation

- **Guides**: Add markdown files to `guide/` for tutorials and how-to content
- **Reference**: Add markdown files to `reference/` for API docs and specifications
- **Examples**: Add markdown files to `examples/` for example presentations

Update `.vitepress/config.js` to include new pages in the sidebar navigation.
