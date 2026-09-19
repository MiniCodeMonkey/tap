/**
 * Resolves the theme currently in effect (store override, then `?theme=`,
 * then the deck's own `theme:` frontmatter - see selectCurrentThemeSlug)
 * into a loaded ThemeDefinition, so both the viewer and the presenter apply
 * `data-theme` only once that theme's CSS and fonts have actually loaded.
 *
 * Both the viewer and PresenterApp call this instead of setting
 * `data-theme` directly from themeOverride/config.theme, so the CSS for
 * any non-base theme is always fetched through loadTheme before it takes
 * effect - including in the presenter's preview panels, and for
 * `?theme=`.
 */

import { useEffect, useState } from 'react';
import { usePresentationStore, selectCurrentThemeSlug } from '$lib/stores/presentation';
import { loadTheme, type ThemeDefinition } from '$lib/themes/loader';

export function useResolvedTheme(): ThemeDefinition | null {
	const requestedTheme = usePresentationStore(selectCurrentThemeSlug);

	// The theme actually applied lags one step behind requestedTheme: it only
	// updates once loadTheme resolves (its CSS and fonts loaded), so
	// data-theme never switches to a theme whose styles aren't ready yet.
	const [resolvedTheme, setResolvedTheme] = useState<ThemeDefinition | null>(null);
	useEffect(() => {
		let cancelled = false;
		void loadTheme(requestedTheme).then((definition) => {
			if (!cancelled) setResolvedTheme(definition);
		});
		return () => {
			cancelled = true;
		};
	}, [requestedTheme]);

	return resolvedTheme;
}
