/**
 * Unit tests for the asciinema player loader: it must load the player and
 * its CSS as a bundled, dynamically-imported chunk, never touch
 * document.head with a CDN URL, and hand back the players it creates so a
 * caller can dispose them.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { renderAsciinemaBlocksInElement, resetAsciinemaLoaderState } from './asciinema';

const createMock = vi.fn((_src: string, _container: HTMLElement, _options?: unknown) => ({
	dispose: vi.fn(),
	getCurrentTime: vi.fn(() => 0),
	getDuration: vi.fn(() => 0),
	play: vi.fn(async () => {}),
	pause: vi.fn(async () => {}),
	seek: vi.fn(async () => {}),
	addEventListener: vi.fn(),
	element: document.createElement('div')
}));

vi.mock('asciinema-player', () => ({
	create: (...args: [string, HTMLElement, unknown?]) => createMock(...args)
}));

vi.mock('asciinema-player/dist/bundle/asciinema-player.css', () => ({}));

function setBody(html: string): HTMLElement {
	document.body.innerHTML = `<div class="slot">${html}</div>`;
	return document.body;
}

describe('renderAsciinemaBlocksInElement', () => {
	beforeEach(() => {
		resetAsciinemaLoaderState();
		createMock.mockClear();
	});

	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('never adds a CDN <link> or <script> to document.head', async () => {
		const element = setBody('<pre><code class="language-asciinema">src: ./demo.cast</code></pre>');

		await renderAsciinemaBlocksInElement(element);

		const headHTML = document.head.innerHTML;
		expect(headHTML).not.toContain('cdn.jsdelivr.net');
		expect(headHTML).not.toContain('asciinema-player');
	});

	it('replaces the code block with a player wrapper and returns the created player', async () => {
		const element = setBody('<pre><code class="language-asciinema">src: ./demo.cast</code></pre>');

		const players = await renderAsciinemaBlocksInElement(element);

		expect(element.querySelector('.asciinema-player-wrapper')).not.toBeNull();
		expect(element.querySelector('pre')).toBeNull();
		expect(players).toHaveLength(1);
		expect(createMock).toHaveBeenCalledTimes(1);
		expect(createMock.mock.calls[0]?.[0]).toBe('./demo.cast');
	});

	it('returns an empty array and does nothing when there is no asciinema block', async () => {
		const element = setBody('<p>No terminal here.</p>');

		const players = await renderAsciinemaBlocksInElement(element);

		expect(players).toEqual([]);
		expect(createMock).not.toHaveBeenCalled();
	});

	it('shows an error state and creates no player when src is missing', async () => {
		const element = setBody('<pre><code class="language-asciinema">autoPlay: true</code></pre>');

		const players = await renderAsciinemaBlocksInElement(element);

		expect(players).toEqual([]);
		expect(element.querySelector('.asciinema-player-wrapper.error')).not.toBeNull();
		expect(createMock).not.toHaveBeenCalled();
	});

	it('creates one player per asciinema block on the slide', async () => {
		const element = setBody(
			'<pre><code class="language-asciinema">src: ./one.cast</code></pre>' +
				'<pre><code class="language-asciinema">src: ./two.cast</code></pre>'
		);

		const players = await renderAsciinemaBlocksInElement(element);

		expect(players).toHaveLength(2);
		expect(createMock).toHaveBeenCalledTimes(2);
	});
});
