import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { SlideErrorBoundary } from './SlideErrorBoundary';

afterEach(() => cleanup());

function Throws(): never {
	throw new Error('slide boom');
}

describe('SlideErrorBoundary', () => {
	const originalDev = import.meta.env.DEV;

	afterEach(() => {
		(import.meta.env as { DEV: boolean }).DEV = originalDev;
	});

	it('renders the fallback next to the marker in the audience-safe form, with ?present=true, in dev', () => {
		(import.meta.env as { DEV: boolean }).DEV = true;
		window.history.pushState({}, '', '/?present=true');
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

		try {
			const { container, getByTestId } = render(
				<SlideErrorBoundary slideNumber={1} fallback={<p data-testid="fallback">normal slide content</p>}>
					<Throws />
				</SlideErrorBoundary>
			);

			expect(getByTestId('fallback')).toBeTruthy();
			const card = container.querySelector('.slide-error.deck-error-card');
			expect(card?.hasAttribute('hidden')).toBe(true);
			expect(container.querySelector('.deck-error-marker')?.textContent).toBe('component error');
		} finally {
			window.history.pushState({}, '', '/');
			consoleSpy.mockRestore();
		}
	});
});
