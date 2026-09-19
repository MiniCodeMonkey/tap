import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import type { Slide as SlideData } from '$lib/types';

const deckComponentSpy = vi.fn(() => <div data-testid="deck-component" />);
vi.mock('../components/DeckComponent', () => ({
	DeckComponent: (props: Record<string, unknown>) => deckComponentSpy(props)
}));

const { LayoutComponent } = await import('./LayoutComponent');

afterEach(() => {
	cleanup();
	deckComponentSpy.mockClear();
});

function makeSlide(overrides: Partial<SlideData> = {}): SlideData {
	return {
		index: 0,
		layout: 'component',
		html: '',
		slots: { default: '<p>slot content</p>' },
		slotOrder: ['default'],
		fragmentCount: 0,
		steps: 3,
		...overrides
	};
}

describe('LayoutComponent', () => {
	it('falls back to the default layout when the slide has no component info', () => {
		const slide = makeSlide({ component: undefined });
		const { container } = render(
			<LayoutComponent slots={slide.slots} slide={slide} step={0} active printMode={false} />
		);
		expect(container.querySelector('.layout-default')).not.toBeNull();
		expect(deckComponentSpy).not.toHaveBeenCalled();
	});

	it('passes the slide.component source, url, css, and error through to DeckComponent, with empty props', () => {
		const slide = makeSlide({
			component: { source: 'slides/Deploy.jsx', url: '/components/Deploy-1.js', css: '/components/Deploy-1.css' }
		});

		render(<LayoutComponent slots={slide.slots} slide={slide} step={1} active printMode={false} />);

		expect(deckComponentSpy).toHaveBeenCalledTimes(1);
		const props = deckComponentSpy.mock.calls[0][0];
		expect(props.source).toBe('slides/Deploy.jsx');
		expect(props.url).toBe('/components/Deploy-1.js');
		expect(props.css).toBe('/components/Deploy-1.css');
		expect(props.props).toEqual({});
		expect(props.slots).toBe(slide.slots);
	});

	it('passes the live step/active/printMode through outside of a preview render', () => {
		const slide = makeSlide({
			component: { source: 'slides/Deploy.jsx', url: '/components/Deploy-1.js' },
			steps: 3
		});

		render(<LayoutComponent slots={slide.slots} slide={slide} step={2} active printMode={false} />);

		const props = deckComponentSpy.mock.calls[0][0];
		expect(props.step).toBe(2);
		expect(props.steps).toBe(3);
		expect(props.active).toBe(true);
		expect(props.printMode).toBe(false);
		expect(props.preview).toBe(false);
	});

	it('overrides step/active/printMode for a preview render: step at the slide total, active false, printMode true', () => {
		const slide = makeSlide({
			component: { source: 'slides/Deploy.jsx', url: '/components/Deploy-1.js' },
			steps: 3
		});

		render(<LayoutComponent slots={slide.slots} slide={slide} step={1} active printMode={false} preview />);

		const props = deckComponentSpy.mock.calls[0][0];
		expect(props.step).toBe(3);
		expect(props.active).toBe(false);
		expect(props.printMode).toBe(true);
		expect(props.preview).toBe(true);
	});

	it('forwards the slide.component build error as buildError', () => {
		const slide = makeSlide({
			component: {
				source: 'slides/Broken.jsx',
				url: '',
				error: 'slides/Broken.jsx:3:7: Unexpected "}"'
			}
		});

		render(<LayoutComponent slots={slide.slots} slide={slide} step={0} active printMode={false} />);

		const props = deckComponentSpy.mock.calls[0][0];
		expect(props.buildError).toBe('slides/Broken.jsx:3:7: Unexpected "}"');
	});
});
