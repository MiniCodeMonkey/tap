# Tap frontend

React 19 + Vite frontend for Tap. The Go backend embeds the built output
(`frontend/dist`) from `embedded/dist` and serves it alongside the deck API.

## Development

```bash
npm install
npm run dev        # Vite dev server, proxies the deck API to the Go backend
npm run build       # production build
```

## Checks

```bash
npm run check       # tsc --noEmit
npm run lint         # eslint
npm run test          # vitest unit tests
npm run test:e2e       # playwright, main app suite
npm run test:themes     # playwright, per-theme check suite (needs BASE_URL, see CONTRIBUTING.md)
```

See the repo root `CONTRIBUTING.md` for the full verification sequence and
the two-server setup the theme suite needs.

## Layout

- `src/lib/themes/` - CSS-only themes (`base` plus 20 named themes), see
  `docs/reference/theme-porting.md` for how to add one.
- `src/lib/layouts/` - slide layout components.
- `e2e/` - main app Playwright suite (port 3000 by default).
- `e2e-themes/` - per-theme overflow/contrast/size/isolation checks plus
  visual snapshots, run against a Go + Vite server pair via `BASE_URL`.
