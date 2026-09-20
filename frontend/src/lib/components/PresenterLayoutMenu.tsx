/**
 * The presenter view's layout picker: a header button that opens a list of
 * the layouts this screen can show, each with a diagram, plus the notes size
 * control.
 *
 * Whether the list is open is the caller's state, because the presenter
 * view's keyboard handler only treats 1 to 5 as layout choices while it is
 * open.
 */

import { useEffect, useRef, type ReactElement } from 'react';
import type {
	NotesSizeMode,
	PresenterLayout,
	PresenterLayoutInfo
} from '$lib/utils/presenterLayout';

export interface PresenterLayoutMenuProps {
	layout: PresenterLayout;
	availableLayouts: PresenterLayoutInfo[];
	onSelectLayout: (layout: PresenterLayout) => void;
	notesSizeMode: NotesSizeMode;
	onSelectNotesSizeMode: (mode: NotesSizeMode) => void;
	isNarrow: boolean;
	isOpen: boolean;
	onOpenChange: (open: boolean) => void;
}

/** A small diagram of where each layout puts its panels. */
function LayoutGlyph({ layout }: { layout: PresenterLayout }): ReactElement {
	const frame = 'presenter-layout-glyph';
	switch (layout) {
		case 'notes-first':
			return (
				<span className={frame} aria-hidden="true">
					<span className="glyph-column narrow">
						<span className="glyph-box current" />
						<span className="glyph-box" />
					</span>
					<span className="glyph-box notes wide" />
				</span>
			);
		case 'duo':
			return (
				<span className={`${frame} stacked`} aria-hidden="true">
					<span className="glyph-row">
						<span className="glyph-box current" />
						<span className="glyph-box" />
					</span>
					<span className="glyph-box notes short" />
				</span>
			);
		case 'slide-only':
			return (
				<span className={frame} aria-hidden="true">
					<span className="glyph-box current" />
				</span>
			);
		case 'notes-only':
			return (
				<span className={`${frame} lines`} aria-hidden="true">
					<span className="glyph-line" />
					<span className="glyph-line" />
					<span className="glyph-line short" />
				</span>
			);
		case 'standard':
		default:
			return (
				<span className={frame} aria-hidden="true">
					<span className="glyph-box current wide" />
					<span className="glyph-column narrow">
						<span className="glyph-box" />
						<span className="glyph-box" />
					</span>
				</span>
			);
	}
}

export function PresenterLayoutMenu({
	layout,
	availableLayouts,
	onSelectLayout,
	notesSizeMode,
	onSelectNotesSizeMode,
	isNarrow,
	isOpen,
	onOpenChange
}: PresenterLayoutMenuProps): ReactElement {
	const triggerRef = useRef<HTMLButtonElement | null>(null);
	const containerRef = useRef<HTMLDivElement | null>(null);
	const current = availableLayouts.find((entry) => entry.id === layout);

	useEffect(() => {
		if (!isOpen) return;
		function handlePointerDown(event: MouseEvent): void {
			if (!containerRef.current?.contains(event.target as Node)) {
				onOpenChange(false);
			}
		}
		document.addEventListener('mousedown', handlePointerDown);
		return () => document.removeEventListener('mousedown', handlePointerDown);
	}, [isOpen, onOpenChange]);

	function choose(next: PresenterLayout): void {
		onSelectLayout(next);
		onOpenChange(false);
		triggerRef.current?.focus();
	}

	return (
		<div className="presenter-layout-menu" ref={containerRef}>
			<button
				type="button"
				ref={triggerRef}
				className="presenter-layout-trigger"
				aria-haspopup="menu"
				aria-expanded={isOpen}
				aria-label={`Layout: ${current?.label ?? 'Standard'}. Change the presenter layout`}
				onClick={() => onOpenChange(!isOpen)}
			>
				<LayoutGlyph layout={layout} />
				<span className="presenter-layout-trigger-label">{current?.label ?? 'Standard'}</span>
			</button>

			{isOpen ? (
				<div className={`presenter-layout-popover${isNarrow ? ' sheet' : ''}`} role="menu">
					<p className="presenter-layout-popover-title">Layout</p>
					{availableLayouts.map((entry, index) => (
						<button
							type="button"
							key={entry.id}
							role="menuitemradio"
							aria-checked={entry.id === layout}
							className={`presenter-layout-option${entry.id === layout ? ' selected' : ''}`}
							onClick={() => choose(entry.id)}
						>
							<LayoutGlyph layout={entry.id} />
							<span className="presenter-layout-option-text">
								<span className="presenter-layout-option-label">{entry.label}</span>
								<span className="presenter-layout-option-description">{entry.description}</span>
							</span>
							<kbd className="presenter-layout-option-key">{index + 1}</kbd>
						</button>
					))}

					<hr className="presenter-layout-separator" />

					<div className="presenter-notes-size-row">
						<span className="presenter-notes-size-label">Notes size</span>
						<span className="presenter-notes-size-options">
							<button
								type="button"
								className={`presenter-notes-size-option${notesSizeMode === 'manual' ? ' selected' : ''}`}
								aria-pressed={notesSizeMode === 'manual'}
								onClick={() => onSelectNotesSizeMode('manual')}
							>
								Manual
							</button>
							<button
								type="button"
								className={`presenter-notes-size-option${notesSizeMode === 'fit' ? ' selected' : ''}`}
								aria-pressed={notesSizeMode === 'fit'}
								onClick={() => onSelectNotesSizeMode('fit')}
							>
								Fit to panel
							</button>
						</span>
					</div>

					<p className="presenter-layout-hint">
						Kept on this device. <kbd>V</kbd> cycles, <kbd>1</kbd> to <kbd>5</kbd> pick one.
					</p>
				</div>
			) : null}
		</div>
	);
}
