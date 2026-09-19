/**
 * Loads presentation data, preferring the JSON embedded in the page (used by
 * static builds) and falling back to the API endpoint served by the dev
 * server. Shared by the audience view and the presenter view so both start
 * from the same data.
 */

import type { Presentation } from '$lib/types';

export async function fetchPresentation(): Promise<Presentation> {
	const embeddedScript = document.getElementById('presentation-data');
	if (embeddedScript) {
		try {
			const parsed = JSON.parse(embeddedScript.textContent || '{}') as Presentation;
			if (parsed.slides && parsed.slides.length > 0) {
				return parsed;
			}
		} catch {
			// Fall through to the API fetch.
		}
	}

	const response = await fetch('/api/presentation');
	if (!response.ok) {
		throw new Error(`Failed to load presentation: ${response.statusText}`);
	}
	return (await response.json()) as Presentation;
}
