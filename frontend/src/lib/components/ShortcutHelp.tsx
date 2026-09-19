/**
 * Overlay listing the keyboard shortcuts of the current view, opened with
 * the ? key. It carries data-theme itself, because theme tokens live on the
 * slide canvas rather than the document, so the panel reads the active
 * theme's colors and fonts. The view's own key handler opens and closes it;
 * a click on the backdrop or the close button also closes it. Titles are
 * plain elements, not headings, because themes decorate h2 and h3 under
 * data-theme (the isometric theme draws a cube before every h3).
 */

import type { Theme } from '$lib/types';
import type { ShortcutGroup } from '$lib/utils/shortcuts';

export interface ShortcutHelpProps {
	/** Shortcut groups to list, one table per group. */
	groups: ShortcutGroup[];
	/** Whether the overlay is currently visible. */
	isOpen?: boolean;
	/** Current theme name. */
	theme?: Theme;
	/** Called when the overlay should close. */
	onClose?: () => void;
}

export function ShortcutHelp({ groups, isOpen = false, theme = 'base' as Theme, onClose }: ShortcutHelpProps) {
	if (!isOpen) {
		return null;
	}

	return (
		<div className="shortcut-help" data-theme={theme} role="dialog" aria-modal="true" aria-label="Keyboard shortcuts">
			<div className="shortcut-help-backdrop" role="presentation" onClick={() => onClose?.()} />

			<div className="shortcut-help-panel">
				<header className="shortcut-help-header">
					<div className="shortcut-help-title">Keyboard shortcuts</div>
					<button type="button" className="shortcut-help-close" onClick={() => onClose?.()} aria-label="Close">
						×
					</button>
				</header>

				{groups.map((group) => (
					<section key={group.title} className="shortcut-help-group">
						<div className="shortcut-help-group-title">{group.title}</div>
						<dl>
							{group.shortcuts.map((shortcut) => (
								<div key={shortcut.action} className="shortcut-help-row">
									<dt>
										{shortcut.keys.map((key) => (
											<kbd key={key}>{key}</kbd>
										))}
									</dt>
									<dd>{shortcut.action}</dd>
								</div>
							))}
						</dl>
						{group.note ? <p className="shortcut-help-note">{group.note}</p> : null}
					</section>
				))}
			</div>
		</div>
	);
}
