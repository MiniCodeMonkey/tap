/**
 * The keyboard shortcuts each browser view listens for, as data. The
 * shortcut overlay (the ? key) renders these lists, so a key added to a
 * handler in keyboard.ts or PresenterApp.tsx belongs here too.
 */

export interface Shortcut {
	/** Key labels, any one of which triggers the action. */
	keys: string[];
	/** What the key does. */
	action: string;
}

export interface ShortcutGroup {
	title: string;
	shortcuts: Shortcut[];
	/** Optional sentence shown under the group. */
	note?: string;
}

/** Key that opens and closes the shortcut overlay in both views. */
export const HELP_KEY = '?';

export const AUDIENCE_SHORTCUTS: ShortcutGroup[] = [
	{
		title: 'Navigation',
		shortcuts: [
			{ keys: ['→', '↓', 'Space', 'Enter', 'PageDown'], action: 'Next fragment, step, or slide' },
			{ keys: ['←', '↑', 'Backspace', 'PageUp'], action: 'Previous fragment, step, or slide' },
			{ keys: ['Home'], action: 'First slide' },
			{ keys: ['End'], action: 'Last slide' }
		]
	},
	{
		title: 'View',
		shortcuts: [
			{ keys: ['S'], action: 'Open the presenter view' },
			{ keys: ['O'], action: 'Toggle the slide overview' },
			{ keys: ['T'], action: 'Cycle to the next theme' },
			{ keys: ['F'], action: 'Toggle fullscreen' },
			{ keys: ['?'], action: 'Show or hide these shortcuts' },
			{ keys: ['Esc'], action: 'Close the overview or this list, or exit fullscreen' }
		],
		note: 'In the overview, arrow keys move the selection and Enter jumps to the selected slide.'
	}
];

export const PRESENTER_SHORTCUTS: ShortcutGroup[] = [
	{
		title: 'Navigation',
		shortcuts: [
			{ keys: ['→', '↓', 'Space', 'Enter', 'PageDown'], action: 'Next fragment, step, or slide' },
			{ keys: ['←', '↑', 'Backspace', 'PageUp'], action: 'Previous fragment, step, or slide' },
			{ keys: ['Home'], action: 'First slide' },
			{ keys: ['End'], action: 'Last slide' }
		]
	},
	{
		title: 'Presenter',
		shortcuts: [
			{ keys: ['R'], action: 'Reset the timer' },
			{ keys: ['-'], action: 'Smaller speaker notes' },
			{ keys: ['='], action: 'Larger speaker notes' },
			{ keys: ['?'], action: 'Show or hide these shortcuts' },
			{ keys: ['Esc'], action: 'Close this list' }
		]
	},
	{
		title: 'Layout',
		shortcuts: [
			{ keys: ['V'], action: 'Next presenter layout' },
			{ keys: ['1', '2', '3', '4', '5'], action: 'Pick a layout while the layout menu is open' }
		]
	}
];
