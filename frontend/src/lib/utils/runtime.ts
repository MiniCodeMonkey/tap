/**
 * Whether this render is server-backed (`tap dev`, `tap export pdf`, `tap
 * export images`, or the frontend's own `npm run dev`) rather than a static
 * `tap build` output. The embedded frontend (embedded/dist) is always a
 * production Vite build, so `import.meta.env.DEV` is always false when it
 * runs inside the real `tap` binary - the signal that actually distinguishes
 * a static build is the `#presentation-data` script tag a static build's
 * index.html embeds (see internal/builder/builder.go), which the live
 * server's own index.html never carries (see internal/server/routes.go's
 * handleIndex, which every live command - dev, present, export pdf, export
images - serves the
 * same embedded index.html through). `frontend/src/lib/stores/websocket.ts`
 * uses the same element to skip connecting a websocket in a build.
 * `import.meta.env.DEV` still counts on its own, for the frontend's own Vite
 * dev server, which has no server-rendered index.html to check.
 */
export function isDevRuntime(): boolean {
	return import.meta.env.DEV || (typeof document !== 'undefined' && !document.getElementById('presentation-data'));
}

/**
 * Whether a component or slide error should show only its safe, audience-facing
 * form (a muted marker, no message) instead of the full error card. A live
 * talk shows a throwing component's raw error to the whole room otherwise, so
 * the audience-facing viewer - fullscreen, or opened with `?present=true` -
 * switches to the safe form. The presenter view (served at /presenter) always
 * keeps the full card in its current-slide panel, so the speaker can read
 * what broke; a print/capture pass (`?print=true`, `?capture=true`) and any
 * page opened with `?debug=true` keep the full card too, since those exist
 * for the deck's author to see the failure while authoring.
 */
export function shouldUseSafeErrorForm(): boolean {
	if (typeof window === 'undefined') {
		return false;
	}
	if (window.location.pathname === '/presenter') {
		return false;
	}
	const params = new URLSearchParams(window.location.search);
	if (params.get('debug') === 'true' || params.get('print') === 'true' || params.get('capture') === 'true') {
		return false;
	}
	if (params.get('present') === 'true') {
		return true;
	}
	return typeof document !== 'undefined' && Boolean(document.fullscreenElement);
}
