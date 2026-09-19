/**
 * Whether this render is server-backed (`tap dev`, `tap pdf`, `tap
 * screenshot`, or the frontend's own `npm run dev`) rather than a static
 * `tap build` output. The embedded frontend (embedded/dist) is always a
 * production Vite build, so `import.meta.env.DEV` is always false when it
 * runs inside the real `tap` binary - the signal that actually distinguishes
 * a static build is the `#presentation-data` script tag a static build's
 * index.html embeds (see internal/builder/builder.go), which the live
 * server's own index.html never carries (see internal/server/routes.go's
 * handleIndex, which every live command - dev, pdf, screenshot - serves the
 * same embedded index.html through). `frontend/src/lib/stores/websocket.ts`
 * uses the same element to skip connecting a websocket in a build.
 * `import.meta.env.DEV` still counts on its own, for the frontend's own Vite
 * dev server, which has no server-rendered index.html to check.
 */
export function isDevRuntime(): boolean {
	return import.meta.env.DEV || (typeof document !== 'undefined' && !document.getElementById('presentation-data'));
}
