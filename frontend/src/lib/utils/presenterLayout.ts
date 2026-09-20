/**
 * The presenter view's layouts, and the rules that pick one.
 *
 * A layout is chosen per device rather than per deck: the same deck wants a
 * wide layout on a laptop and a notes-only one on a phone, so localStorage
 * outranks the deck's own `presenterLayout` frontmatter key. A URL parameter
 * outranks both and is never written back, which is how a QR code can send a
 * phone straight to a phone-shaped layout.
 *
 * Nothing here touches the DOM beyond localStorage, so it is all directly
 * testable.
 */

export type PresenterLayout = 'standard' | 'notes-first' | 'duo' | 'slide-only' | 'notes-only';

export type NotesSizeMode = 'manual' | 'fit';

export interface PresenterLayoutInfo {
	id: PresenterLayout;
	label: string;
	description: string;
	/** Whether the layout is offered below NARROW_MAX_WIDTH. */
	availableWhenNarrow: boolean;
}

/** Matches the breakpoint presenter-view.css uses for its phone rules. */
export const NARROW_MAX_WIDTH = 768;
export const NARROW_QUERY = `(max-width: ${NARROW_MAX_WIDTH}px)`;

export const DEFAULT_LAYOUT: PresenterLayout = 'standard';
/** Used when the winning layout needs more width than the screen has. */
export const NARROW_FALLBACK_LAYOUT: PresenterLayout = 'notes-first';
export const DEFAULT_NOTES_SIZE_MODE: NotesSizeMode = 'manual';

export const LAYOUT_STORAGE_KEY = 'tap-presenter-layout';
export const NOTES_SIZE_MODE_STORAGE_KEY = 'tap-presenter-notes-size-mode';

export const PRESENTER_LAYOUTS: PresenterLayoutInfo[] = [
	{
		id: 'standard',
		label: 'Standard',
		description: 'Big current slide, next slide and notes beside it',
		availableWhenNarrow: true
	},
	{
		id: 'notes-first',
		label: 'Notes first',
		description: 'Notes take the room; both slides shrink to cues',
		availableWhenNarrow: true
	},
	{
		id: 'duo',
		label: 'Duo',
		description: 'Current and next at equal size, notes on a strip below',
		availableWhenNarrow: false
	},
	{
		id: 'slide-only',
		label: 'Slide only',
		description: 'A confidence monitor: no notes, no look-ahead',
		availableWhenNarrow: false
	},
	{
		id: 'notes-only',
		label: 'Notes only',
		description: 'The script, nothing else. The projector has the slide',
		availableWhenNarrow: true
	}
];

const NOTES_SIZE_MODES: NotesSizeMode[] = ['manual', 'fit'];

export function isPresenterLayout(value: unknown): value is PresenterLayout {
	return PRESENTER_LAYOUTS.some((entry) => entry.id === value);
}

export function isNotesSizeMode(value: unknown): value is NotesSizeMode {
	return NOTES_SIZE_MODES.some((mode) => mode === value);
}

/** The layouts offered at the current width, in menu order. */
export function availableLayouts(isNarrow: boolean): PresenterLayoutInfo[] {
	return PRESENTER_LAYOUTS.filter((entry) => !isNarrow || entry.availableWhenNarrow);
}

export interface ResolveLayoutSources {
	fromUrl?: string | null;
	fromStorage?: string | null;
	fromConfig?: string | null;
	isNarrow: boolean;
}

/**
 * Picks the layout from the highest source that names a known one. A layout
 * the screen is too narrow for becomes NARROW_FALLBACK_LAYOUT; the stored
 * value is left alone, so widening the window brings it back.
 */
export function resolveLayout({
	fromUrl,
	fromStorage,
	fromConfig,
	isNarrow
}: ResolveLayoutSources): PresenterLayout {
	for (const candidate of [fromUrl, fromStorage, fromConfig]) {
		if (!isPresenterLayout(candidate)) continue;
		const offered = availableLayouts(isNarrow).some((entry) => entry.id === candidate);
		return offered ? candidate : NARROW_FALLBACK_LAYOUT;
	}
	return DEFAULT_LAYOUT;
}

/** The next layout in menu order, wrapping at the end. */
export function nextLayout(current: PresenterLayout, isNarrow: boolean): PresenterLayout {
	const offered = availableLayouts(isNarrow);
	const index = offered.findIndex((entry) => entry.id === current);
	return offered[(index + 1) % offered.length].id;
}

export function resolveNotesSizeMode({
	fromUrl,
	fromStorage
}: {
	fromUrl?: string | null;
	fromStorage?: string | null;
}): NotesSizeMode {
	for (const candidate of [fromUrl, fromStorage]) {
		if (isNotesSizeMode(candidate)) return candidate;
	}
	return DEFAULT_NOTES_SIZE_MODE;
}

function readStoredValue(key: string): string | null {
	try {
		return window.localStorage.getItem(key);
	} catch {
		// Storage can be unavailable (private window, blocked site data).
		return null;
	}
}

function writeStoredValue(key: string, value: string): void {
	try {
		window.localStorage.setItem(key, value);
	} catch {
		// The choice then lasts only until reload.
	}
}

export function readStoredLayout(): PresenterLayout | null {
	const stored = readStoredValue(LAYOUT_STORAGE_KEY);
	return isPresenterLayout(stored) ? stored : null;
}

export function writeStoredLayout(layout: PresenterLayout): void {
	writeStoredValue(LAYOUT_STORAGE_KEY, layout);
}

export function readStoredNotesSizeMode(): NotesSizeMode | null {
	const stored = readStoredValue(NOTES_SIZE_MODE_STORAGE_KEY);
	return isNotesSizeMode(stored) ? stored : null;
}

export function writeStoredNotesSizeMode(mode: NotesSizeMode): void {
	writeStoredValue(NOTES_SIZE_MODE_STORAGE_KEY, mode);
}
