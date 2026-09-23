/**
 * A slide holds a "component" blocker while its rich blocks (mermaid,
 * asciinema, Shiki) render, so the ready signal waits for them.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/react';
import { useRef } from 'react';
import type { Slide } from '$lib/types';
import { heldBlockers, resetBlockersForTests } from '$lib/ready/blockers';
import { useRichBlocks } from './useRichBlocks';
import { renderMermaidBlocksInElement } from '../utils/mermaid';

vi.mock('../utils/mermaid', () => ({ renderMermaidBlocksInElement: vi.fn(() => Promise.resolve()) }));
vi.mock('../utils/highlighting', () => ({ highlightCodeBlocksInElement: vi.fn(() => Promise.resolve()) }));
vi.mock('../utils/asciinema', () => ({ renderAsciinemaBlocksInElement: vi.fn(() => Promise.resolve([])) }));

function makeSlide(): Slide {
	return {
		index: 0,
		layout: 'default',
		html: '',
		slots: { default: '<pre><code class="language-mermaid">graph TD; A-->B</code></pre>' },
		slotOrder: ['default'],
		fragmentCount: 0,
		steps: 0
	};
}

function Harness({ slide, active }: { slide: Slide; active: boolean }) {
	const elementRef = useRef<HTMLDivElement>(null);
	useRichBlocks(elementRef, { slide, active, printMode: false });
	return <div ref={elementRef} />;
}

beforeEach(() => resetBlockersForTests());
afterEach(() => cleanup());

describe('useRichBlocks and the ready signal', () => {
	it('holds a component blocker until the rich blocks have rendered', async () => {
		let finishMermaid!: () => void;
		vi.mocked(renderMermaidBlocksInElement).mockReturnValueOnce(
			new Promise<void>((resolve) => {
				finishMermaid = resolve;
			})
		);
		render(<Harness slide={makeSlide()} active />);
		expect(heldBlockers()).toEqual(['component']);

		finishMermaid();
		await waitFor(() => expect(heldBlockers()).toEqual([]));
	});

	it('holds nothing for a slide that is neither active nor printed', () => {
		render(<Harness slide={makeSlide()} active={false} />);
		expect(heldBlockers()).toEqual([]);
	});
});
