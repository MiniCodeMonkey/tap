/**
 * A live update to the slide on screen keeps the slide mounted and never
 * replays its entrance animations; arriving at a slide still plays them.
 *
 * jsdom runs no CSS, so every element reports one fake CSS animation from
 * getAnimations(), and each test checks which of them the slide finished.
 * An inline deck component is a stand-in that counts its own mounts, since
 * a Motion entrance replays exactly when the component mounts again.
 */
import { useEffect, useState } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render } from '@testing-library/react';
import { Slide } from './Slide';
import type { CodeBlock, Slide as SlideData, SlideComponentInfo } from '$lib/types';

const componentMounts: string[] = [];

function FakeDeckComponent({ source, props }: { source: string; props: Record<string, unknown> }) {
	// Runs once per mount, never for a prop change on a mounted instance.
	const [mountedSource] = useState(source);
	useEffect(() => {
		componentMounts.push(mountedSource);
	}, [mountedSource]);
	return <p data-testid="deck-component">{`${source} ${String(props.label)}`}</p>;
}

const liveCodeMounts: string[] = [];

function FakeLiveCodeBlock({ codeBlock }: { codeBlock: CodeBlock }) {
	// Runs once per mount, never for a prop change on a mounted instance.
	const [mountedCode] = useState(codeBlock.code);
	useEffect(() => {
		liveCodeMounts.push(mountedCode);
	}, [mountedCode]);
	return <p data-testid="live-code">{codeBlock.code}</p>;
}

vi.mock('./LiveCodeBlock', () => ({
	LiveCodeBlock: (props: { codeBlock: CodeBlock }) => <FakeLiveCodeBlock {...props} />
}));

vi.mock('./DeckComponent', () => ({
	DeckComponent: (props: { source: string; props: Record<string, unknown> }) => <FakeDeckComponent {...props} />
}));

interface FakeAnimation {
	animationName: string;
	target: Element;
	infinite: boolean;
	finished: boolean;
}

let animations: FakeAnimation[] = [];
const originalGetAnimations = Element.prototype.getAnimations;

function animationFor(element: Element): FakeAnimation {
	let animation = animations.find((existing) => existing.target === element);
	if (!animation) {
		animation = {
			animationName: 'theme-rise',
			target: element,
			infinite: element.classList.contains('blink'),
			finished: false
		};
		animations.push(animation);
	}
	return animation;
}

function fakeGetAnimations(this: Element, options?: GetAnimationsOptions): Animation[] {
	const elements = options?.subtree ? [this, ...Array.from(this.querySelectorAll('*'))] : [this];
	return elements.map((element) => {
		const animation = animationFor(element);
		return {
			animationName: animation.animationName,
			effect: { getComputedTiming: () => ({ endTime: animation.infinite ? Infinity : 900 }) },
			finish: () => {
				if (animation.infinite) throw new DOMException('infinite', 'InvalidStateError');
				animation.finished = true;
			}
		} as unknown as Animation;
	});
}

function finishedTexts(): string[] {
	return animations.filter((animation) => animation.finished).map((animation) => animation.target.textContent ?? '');
}

function makeSlide(index: number, body: string, fragmentCount = 0): SlideData {
	return {
		index,
		layout: 'default',
		html: '',
		slots: { default: body },
		slotOrder: ['default'],
		fragmentCount,
		steps: 0,
		hash: `${index}:${body}`
	};
}

function makeComponentSlide(index: number, heading: string, components: SlideComponentInfo[]): SlideData {
	const placeholders = components
		.map((component) => `<div class="deck-component" data-component-index="${component.index}"></div>`)
		.join('');
	return { ...makeSlide(index, `<h2>${heading}</h2>${placeholders}`), components };
}

function makeLiveCodeSlide(index: number, heading: string, code: string): SlideData {
	const block = `<pre><code class="language-bash" data-code-block-index="0">${code}</code></pre>`;
	return {
		...makeSlide(index, `<h2>${heading}</h2>${block}`),
		codeBlocks: [{ language: 'bash', code, driver: 'shell', block: 1 }]
	};
}

function chart(label: string, source = './charts/LatencyDrop.jsx'): SlideComponentInfo {
	return { index: 0, source, url: '/components/LatencyDrop-abc.js', props: { label } };
}

function renderSlide(slide: SlideData, fragmentIndex = -1) {
	return <Slide slide={slide} active printMode={false} fragmentIndex={fragmentIndex} step={0} total={3} />;
}

beforeEach(() => {
	animations = [];
	componentMounts.length = 0;
	liveCodeMounts.length = 0;
	Element.prototype.getAnimations = fakeGetAnimations;
});

afterEach(() => {
	cleanup();
	Element.prototype.getAnimations = originalGetAnimations;
});

describe('Slide entrance animations', () => {
	it('finishes the entrance animations a live update to the current slide would replay, without remounting it', async () => {
		const { container, rerender } = render(renderSlide(makeSlide(1, '<h1>Title</h1><p>Body</p>')));
		const root = container.querySelector('.slide');

		await act(async () => {
			rerender(renderSlide(makeSlide(1, '<h1>Title</h1><p>Body edited</p>')));
		});

		expect(container.querySelector('.slide')).toBe(root);
		expect(container.querySelector('.slot')?.textContent).toBe('TitleBody edited');
		expect(finishedTexts()).toEqual(expect.arrayContaining(['Title', 'Body edited']));
		// The slide root itself kept its element, and its running animations with it.
		expect(animations.find((animation) => animation.target === root)?.finished ?? false).toBe(false);
	});

	it('plays the entrance animations when navigating to another slide', async () => {
		const { container, rerender } = render(renderSlide(makeSlide(1, '<h1>One</h1>')));

		await act(async () => {
			rerender(renderSlide(makeSlide(2, '<h1>Two</h1><p>More</p>')));
		});

		expect(container.querySelector('.slot')?.textContent).toBe('TwoMore');
		expect(finishedTexts()).toEqual([]);
	});

	it('plays them again when navigating after a live update', async () => {
		const { rerender } = render(renderSlide(makeSlide(1, '<h1>One</h1>')));
		await act(async () => {
			rerender(renderSlide(makeSlide(1, '<h1>One edited</h1>')));
		});
		animations = animations.filter((animation) => !animation.finished);

		await act(async () => {
			rerender(renderSlide(makeSlide(2, '<h1>Two</h1>')));
		});

		expect(finishedTexts()).toEqual([]);
	});

	it('keeps the revealed fragments and leaves infinite animations running across a live update', async () => {
		const fragments =
			'<p class="fragment" data-fragment-index="0">First</p><p class="fragment" data-fragment-index="1">Second</p><span class="blink">_</span>';
		const { container, rerender } = render(renderSlide(makeSlide(1, `<h1>Title</h1>${fragments}`, 2), 0));

		await act(async () => {
			rerender(renderSlide(makeSlide(1, `<h1>Title edited</h1>${fragments}`, 2), 0));
		});

		const classes = Array.from(container.querySelectorAll('[data-fragment-index]')).map((element) =>
			element.classList.contains('fragment-visible')
		);
		expect(classes).toEqual([true, false]);
		expect(finishedTexts()).toEqual(expect.arrayContaining(['Title edited', 'First', 'Second']));
		expect(animations.find((animation) => animation.infinite)?.finished).toBe(false);
	});

	it('keeps an inline deck component mounted through a live update to its slide', async () => {
		const { container, rerender } = render(renderSlide(makeComponentSlide(3, 'Results', [chart('p95')])));
		expect(componentMounts).toEqual(['./charts/LatencyDrop.jsx']);
		const placeholder = container.querySelector('.deck-component');

		await act(async () => {
			rerender(renderSlide(makeComponentSlide(3, 'Results edited', [chart('p95')])));
		});
		await act(async () => {
			rerender(renderSlide(makeComponentSlide(3, 'Results edited', [chart('p99')])));
		});

		expect(container.querySelector('h2')?.textContent).toBe('Results edited');
		expect(container.querySelector('[data-testid="deck-component"]')?.textContent).toBe('./charts/LatencyDrop.jsx p99');
		expect(container.querySelector('.deck-component')).toBe(placeholder);
		expect(componentMounts).toEqual(['./charts/LatencyDrop.jsx']);
	});

	it('mounts an inline deck component again when navigating, even to the same component', async () => {
		const { container, rerender } = render(renderSlide(makeComponentSlide(3, 'Results', [chart('p95')])));

		await act(async () => {
			rerender(renderSlide(makeComponentSlide(4, 'More results', [chart('us-east')])));
		});

		expect(container.querySelector('[data-testid="deck-component"]')?.textContent).toBe(
			'./charts/LatencyDrop.jsx us-east'
		);
		expect(componentMounts).toEqual(['./charts/LatencyDrop.jsx', './charts/LatencyDrop.jsx']);
	});

	it('mounts a different component fresh when a live update changes the source at its position', async () => {
		const { container, rerender } = render(renderSlide(makeComponentSlide(3, 'Results', [chart('p95')])));

		await act(async () => {
			rerender(renderSlide(makeComponentSlide(3, 'Results edited', [chart('p95', './charts/Other.jsx')])));
		});

		expect(container.querySelector('[data-testid="deck-component"]')?.textContent).toBe('./charts/Other.jsx p95');
		expect(componentMounts).toEqual(['./charts/LatencyDrop.jsx', './charts/Other.jsx']);
	});

	it('keeps a live code block mounted, with its output, through a live update elsewhere on its slide', async () => {
		const { container, rerender } = render(renderSlide(makeLiveCodeSlide(2, 'Demo', 'echo hi')));
		expect(liveCodeMounts).toEqual(['echo hi']);
		const wrapper = container.querySelector('.live-code-block-portal');

		await act(async () => {
			rerender(renderSlide(makeLiveCodeSlide(2, 'Demo edited', 'echo hi')));
		});

		expect(container.querySelector('h2')?.textContent).toBe('Demo edited');
		expect(container.querySelector('.live-code-block-portal')).toBe(wrapper);
		expect(container.querySelectorAll('[data-testid="live-code"]')).toHaveLength(1);
		expect(liveCodeMounts).toEqual(['echo hi']);
	});

	it('mounts a live code block fresh when a live update changes its code', async () => {
		const { container, rerender } = render(renderSlide(makeLiveCodeSlide(2, 'Demo', 'echo hi')));

		await act(async () => {
			rerender(renderSlide(makeLiveCodeSlide(2, 'Demo', 'echo bye')));
		});

		expect(container.querySelector('[data-testid="live-code"]')?.textContent).toBe('echo bye');
		expect(liveCodeMounts).toEqual(['echo hi', 'echo bye']);
	});

	it('mounts a live code block fresh when navigating, even to an identical block', async () => {
		const { rerender } = render(renderSlide(makeLiveCodeSlide(2, 'Demo', 'echo hi')));

		await act(async () => {
			rerender(renderSlide(makeLiveCodeSlide(3, 'Demo again', 'echo hi')));
		});

		expect(liveCodeMounts).toEqual(['echo hi', 'echo hi']);
	});
});
