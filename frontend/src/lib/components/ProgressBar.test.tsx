import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { ProgressBar } from './ProgressBar';
import { usePresentationStore, resetPresentation } from '$lib/stores/presentation';
import type { Presentation } from '$lib/types';

function makePresentation(slideCount: number, skipped: number[] = []): Presentation {
	return {
		config: {},
		slides: Array.from({ length: slideCount }, (_, index) => ({
			index,
			layout: 'default',
			html: '',
			slots: {},
			slotOrder: [],
			fragmentCount: 0,
			steps: 0,
			skip: skipped.includes(index)
		}))
	};
}

afterEach(() => {
	cleanup();
	resetPresentation();
});

describe('ProgressBar', () => {
	it('renders nothing when there is no presentation', () => {
		const { container } = render(<ProgressBar />);
		expect(container.firstChild).toBeNull();
	});

	it('renders nothing when show is false', () => {
		usePresentationStore.setState({ presentation: makePresentation(5), currentSlideIndex: 0 });
		const { container } = render(<ProgressBar show={false} />);
		expect(container.firstChild).toBeNull();
	});

	it('sets the fill width to (index + 1) / total on the first slide', () => {
		usePresentationStore.setState({ presentation: makePresentation(4), currentSlideIndex: 0 });

		const { container } = render(<ProgressBar />);

		const fill = container.querySelector('.progress-bar-fill') as HTMLElement;
		expect(fill.style.width).toBe('25%');
	});

	it('sets the fill width to 100% on the last slide', () => {
		usePresentationStore.setState({ presentation: makePresentation(4), currentSlideIndex: 3 });

		const { container } = render(<ProgressBar />);

		const fill = container.querySelector('.progress-bar-fill') as HTMLElement;
		expect(fill.style.width).toBe('100%');
	});

	it('exposes progress through ARIA attributes', () => {
		usePresentationStore.setState({ presentation: makePresentation(10), currentSlideIndex: 2 });

		const { container } = render(<ProgressBar />);

		const bar = container.querySelector('.progress-bar-container');
		expect(bar).toHaveAttribute('aria-valuenow', '3');
		expect(bar).toHaveAttribute('aria-valuemin', '1');
		expect(bar).toHaveAttribute('aria-valuemax', '10');
	});

	it('counts only the slides that are not skipped', () => {
		usePresentationStore.setState({ presentation: makePresentation(5, [1]), currentSlideIndex: 2 });

		const { container } = render(<ProgressBar />);

		const fill = container.querySelector('.progress-bar-fill') as HTMLElement;
		expect(fill.style.width).toBe('50%');
		expect(container.querySelector('[role="progressbar"]')?.getAttribute('aria-valuemax')).toBe('4');
	});

	it('reports an honest, in-range value on a skipped slide opened before any presented one', () => {
		// tap dev opens a skipped slide directly. Slide 0 here is skipped and
		// nothing presented comes before it, so there is no "Nth presented
		// slide" to report - the bar reports 0, not a value floored up into
		// range, and aria-valuemin drops to match so aria-valuenow stays valid.
		usePresentationStore.setState({ presentation: makePresentation(3, [0]), currentSlideIndex: 0 });

		const { container } = render(<ProgressBar />);

		const fill = container.querySelector('.progress-bar-fill') as HTMLElement;
		expect(fill.style.width).toBe('0%');
		const bar = container.querySelector('[role="progressbar"]');
		expect(bar).toHaveAttribute('aria-valuenow', '0');
		expect(bar).toHaveAttribute('aria-valuemin', '0');
		expect(bar).toHaveAttribute('aria-valuemax', '2');
	});
});
