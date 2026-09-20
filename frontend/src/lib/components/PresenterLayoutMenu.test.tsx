import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { PresenterLayoutMenu, type PresenterLayoutMenuProps } from './PresenterLayoutMenu';
import { availableLayouts } from '$lib/utils/presenterLayout';

afterEach(cleanup);

function renderMenu(overrides: Partial<PresenterLayoutMenuProps> = {}) {
	const props: PresenterLayoutMenuProps = {
		layout: 'standard',
		availableLayouts: availableLayouts(false),
		onSelectLayout: vi.fn(),
		notesSizeMode: 'manual',
		onSelectNotesSizeMode: vi.fn(),
		isNarrow: false,
		isOpen: false,
		onOpenChange: vi.fn(),
		...overrides
	};
	render(<PresenterLayoutMenu {...props} />);
	return props;
}

describe('PresenterLayoutMenu', () => {
	it('shows the current layout on the trigger', () => {
		renderMenu({ layout: 'notes-only' });
		expect(screen.getByRole('button', { name: /notes only/i })).toBeTruthy();
	});

	it('asks to open when the trigger is pressed', () => {
		const props = renderMenu();
		fireEvent.click(screen.getByRole('button', { name: /standard/i }));
		expect(props.onOpenChange).toHaveBeenCalledWith(true);
	});

	it('lists nothing while closed', () => {
		renderMenu();
		expect(screen.queryByRole('menu')).toBeNull();
	});

	it('lists every available layout while open', () => {
		renderMenu({ isOpen: true });
		expect(screen.getAllByRole('menuitemradio').length).toBe(5);
	});

	it('lists only the narrow layouts on a narrow screen', () => {
		renderMenu({ isOpen: true, isNarrow: true, availableLayouts: availableLayouts(true) });
		expect(screen.getAllByRole('menuitemradio').length).toBe(3);
	});

	it('reports a chosen layout and closes', () => {
		const props = renderMenu({ isOpen: true });
		fireEvent.click(screen.getByRole('menuitemradio', { name: /duo/i }));
		expect(props.onSelectLayout).toHaveBeenCalledWith('duo');
		expect(props.onOpenChange).toHaveBeenCalledWith(false);
	});

	it('reports a chosen notes size mode', () => {
		const props = renderMenu({ isOpen: true });
		fireEvent.click(screen.getByRole('button', { name: /fit to panel/i }));
		expect(props.onSelectNotesSizeMode).toHaveBeenCalledWith('fit');
	});

	it('marks the current layout as checked', () => {
		renderMenu({ isOpen: true, layout: 'duo' });
		const chosen = screen.getByRole('menuitemradio', { name: /duo/i });
		expect(chosen.getAttribute('aria-checked')).toBe('true');
	});
});
