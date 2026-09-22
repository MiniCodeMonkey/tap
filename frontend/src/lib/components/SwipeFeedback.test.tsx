import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { SwipeFeedback } from './SwipeFeedback';
import { resetPresentation, usePresentationStore } from '$lib/stores/presentation';
import type { Presentation } from '$lib/types';

const presentation: Presentation = {
	config: {},
	slides: [0, 1, 2].map((index) => ({
		index,
		layout: 'default',
		html: '',
		slots: {},
		slotOrder: [],
		fragmentCount: 0,
		steps: 0,
		skip: index === 1
	}))
};

afterEach(() => {
	cleanup();
	resetPresentation();
});

describe('SwipeFeedback', () => {
	it('shows the position among the slides that are not skipped', () => {
		usePresentationStore.setState({ presentation, currentSlideIndex: 2 });
		const { container } = render(<SwipeFeedback direction="next" moved nonce={1} />);
		expect(container.querySelector('.swipe-feedback-position')?.textContent).toBe('2 / 2');
	});
});
