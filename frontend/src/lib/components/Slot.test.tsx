import { describe, expect, it, afterEach } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { Slot } from './Slot';
import { SlideContext, type SlideContextValue } from './SlideContext';

afterEach(cleanup);

function renderSlot(html: string | undefined, contextValue: SlideContextValue, className = 'slot-default') {
	return render(
		<SlideContext.Provider value={contextValue}>
			<Slot html={html} className={className} />
		</SlideContext.Provider>
	);
}

describe('Slot', () => {
	it('returns null when html is undefined', () => {
		const { container } = renderSlot(undefined, { fragmentIndex: -1, printMode: false });
		expect(container.firstChild).toBeNull();
	});

	it('renders the html inside a slot slot-content element with the given class', () => {
		const { container } = renderSlot('<p>Hello</p>', { fragmentIndex: -1, printMode: false });
		const element = container.querySelector('.slot.slot-content.slot-default');
		expect(element).toBeInTheDocument();
		expect(element?.innerHTML).toBe('<p>Hello</p>');
	});

	it("passes a heading's data-length attribute through unchanged", () => {
		// data-length is written server-side (internal/parser/heading_length.go);
		// Slot only needs to render the backend's HTML verbatim, not compute
		// or touch the attribute itself.
		const html = '<h1 data-length="short">Ship it</h1>';
		const { container } = renderSlot(html, { fragmentIndex: -1, printMode: false });
		const heading = container.querySelector('h1');
		expect(heading).toHaveAttribute('data-length', 'short');
	});

	it('marks the fragment at the current index visible and later ones hidden', () => {
		const html = '<div data-fragment-index="0">a</div><div data-fragment-index="1">b</div>';
		const { container } = renderSlot(html, { fragmentIndex: 0, printMode: false });
		const fragments = container.querySelectorAll('[data-fragment-index]');

		expect(fragments[0]).toHaveClass('fragment-visible');
		expect(fragments[0]).not.toHaveClass('fragment-hidden');
		expect(fragments[1]).toHaveClass('fragment-hidden');
		expect(fragments[1]).not.toHaveClass('fragment-visible');
	});

	it('shows every fragment in print mode', () => {
		const html = '<div data-fragment-index="0">a</div><div data-fragment-index="1">b</div>';
		const { container } = renderSlot(html, { fragmentIndex: -1, printMode: true });
		const fragments = container.querySelectorAll('[data-fragment-index]');

		fragments.forEach((fragment) => {
			expect(fragment).toHaveClass('fragment-visible');
			expect(fragment).not.toHaveClass('fragment-hidden');
		});
	});
});
