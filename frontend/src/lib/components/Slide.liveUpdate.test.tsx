/**
 * A live update to the slide on screen keeps the slide mounted and never
 * replays its entrance animations; arriving at a slide still plays them.
 *
 * jsdom runs no CSS, so every element reports one fake CSS animation from
 * getAnimations(), and each test checks which of them the slide finished.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render } from '@testing-library/react';
import { Slide } from './Slide';
import type { Slide as SlideData } from '$lib/types';

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

function renderSlide(slide: SlideData, fragmentIndex = -1) {
	return <Slide slide={slide} active printMode={false} fragmentIndex={fragmentIndex} step={0} total={3} />;
}

beforeEach(() => {
	animations = [];
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
});
