import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { ScrollReveal } from './ScrollReveal';

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

describe('ScrollReveal', () => {
	it('renders children directly, with no wrapper, when disabled', () => {
		const { container } = render(
			<ScrollReveal enabled={false} revealed={false} speed={500} triggerCount={0}>
				<p>content</p>
			</ScrollReveal>
		);

		expect(container.querySelector('.scroll-content')).not.toBeInTheDocument();
		expect(container.textContent).toContain('content');
	});

	it('wraps children in a scroll-content element when enabled', () => {
		const { container } = render(
			<ScrollReveal enabled revealed={false} speed={500} triggerCount={0}>
				<p>content</p>
			</ScrollReveal>
		);

		expect(container.querySelector('.scroll-content')).toBeInTheDocument();
	});

	it('keeps the scroll position at 0 when not revealed', () => {
		const { container } = render(
			<ScrollReveal enabled revealed={false} speed={500} triggerCount={0}>
				<p>content</p>
			</ScrollReveal>
		);

		const content = container.querySelector('.scroll-content') as HTMLElement;
		expect(content.style.transform).toBe('translateY(-0px)');
	});

	it('stays at 0 when revealed but there is nothing to scroll (jsdom reports no overflow)', () => {
		const { container } = render(
			<ScrollReveal enabled revealed speed={500} triggerCount={1}>
				<p>content</p>
			</ScrollReveal>
		);

		const content = container.querySelector('.scroll-content') as HTMLElement;
		expect(content.style.transform).toBe('translateY(-0px)');
	});

	it('measures the scroll distance against the parent content box, excluding its vertical padding', () => {
		// jsdom has no layout, so stub the measurements: the child reports
		// scrollHeight 600, the parent reports clientHeight 500 with 20px top
		// and 30px bottom padding, so the parent's content-box height is 450
		// and the correct scroll distance is 600 - 450 = 150, not 600 - 500 = 100.
		vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(600);
		vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(500);
		vi.spyOn(window, 'getComputedStyle').mockReturnValue({
			paddingTop: '20px',
			paddingBottom: '30px'
		} as CSSStyleDeclaration);

		const { container } = render(
			<ScrollReveal enabled revealed speed={500} triggerCount={1}>
				<p>content</p>
			</ScrollReveal>
		);

		const content = container.querySelector('.scroll-content') as HTMLElement;
		expect(content.style.transform).toBe('translateY(-150px)');
	});

	it('does not animate the first position it applies', () => {
		const { container } = render(
			<ScrollReveal enabled revealed={false} speed={500} triggerCount={0}>
				<p>content</p>
			</ScrollReveal>
		);

		const content = container.querySelector('.scroll-content') as HTMLElement;
		expect(content.style.transition).toBe('none');
	});
});
