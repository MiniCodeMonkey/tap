/**
 * Whether an error should render in its audience-safe form (see
 * shouldUseSafeErrorForm in runtime.ts), kept live across
 * `document.fullscreenElement` changes. shouldUseSafeErrorForm is decided
 * at render time only when called directly, so a card already on screen
 * would otherwise keep its form until something unrelated re-renders it;
 * this hook subscribes to `fullscreenchange` so the form switches the
 * moment the viewer enters or leaves fullscreen.
 */

import { useEffect, useState } from 'react';
import { shouldUseSafeErrorForm } from '$lib/utils/runtime';

export function useSafeErrorForm(): boolean {
	const [safe, setSafe] = useState(shouldUseSafeErrorForm());

	useEffect(() => {
		const update = () => setSafe(shouldUseSafeErrorForm());
		update();
		document.addEventListener('fullscreenchange', update);
		return () => document.removeEventListener('fullscreenchange', update);
	}, []);

	return safe;
}
