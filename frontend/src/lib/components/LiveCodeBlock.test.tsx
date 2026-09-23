import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { LiveCodeBlock } from './LiveCodeBlock';
import { useConnectionStore } from '../stores/websocket';
import { usePresentationStore } from '../stores/presentation';
import type { CodeBlock } from '$lib/types';

vi.mock('../utils/highlighting', () => ({
	highlight: vi.fn(async (code: string) => `<pre><code>${code}</code></pre>`)
}));

function sqlBlock(overrides: Partial<CodeBlock> = {}): CodeBlock {
	return {
		language: 'sql',
		code: 'SELECT 1;',
		driver: 'sqlite',
		connection: 'demo',
		block: 1,
		...overrides
	};
}

describe('LiveCodeBlock', () => {
	beforeEach(() => {
		useConnectionStore.setState({ connected: true, staticMode: false });
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
		useConnectionStore.setState({ connected: false, staticMode: false });
	});

	it('highlights its own code on mount', async () => {
		const { container } = render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);

		await waitFor(() => {
			expect(container.querySelector('.code-highlight pre code')).toHaveTextContent('SELECT 1;');
		});
	});

	it('shows a run button when live execution is available for a driver block', async () => {
		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
		expect(await screen.findByRole('button', { name: 'Run code' })).toBeInTheDocument();
	});

	it('does not show a run button for a code block with no driver', async () => {
		render(<LiveCodeBlock codeBlock={sqlBlock({ driver: undefined })} slideNumber={4} />);
		await waitFor(() => expect(document.querySelector('.code-highlight')?.innerHTML).not.toBe(''));
		expect(screen.queryByRole('button', { name: 'Run code' })).not.toBeInTheDocument();
	});

	it('shows the static placeholder instead of a run button when live execution is unavailable', async () => {
		useConnectionStore.setState({ connected: false, staticMode: true });
		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);

		expect(await screen.findByText('Live execution available in presentation mode')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Run code' })).not.toBeInTheDocument();
	});

	it('sends only the slide and block to POST /api/execute when run', async () => {
		const fetchMock = vi.fn(async () =>
			({ ok: true, json: async () => ({ success: true, output: 'ok' }) }) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);

		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run code' }));

		await waitFor(() => {
			expect(fetchMock).toHaveBeenCalledWith(
				'/api/execute',
				expect.objectContaining({
					method: 'POST',
					body: JSON.stringify({ slide: 4, block: 1 })
				})
			);
		});
	});

	it('sends the revision this page rendered from along with the reference', async () => {
		usePresentationStore.setState({
			presentation: { config: {}, slides: [], revision: 'rev-42' }
		});
		const fetchMock = vi.fn(async () =>
			({ ok: true, json: async () => ({ success: true, output: 'ok' }) }) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);

		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run code' }));

		await waitFor(() => {
			expect(fetchMock).toHaveBeenCalledWith(
				'/api/execute',
				expect.objectContaining({
					method: 'POST',
					body: JSON.stringify({ slide: 4, block: 1, revision: 'rev-42' })
				})
			);
		});
		usePresentationStore.setState({ presentation: null });
	});

	it('shows a distinct, non-retried refusal when the server reports the deck changed', async () => {
		const fetchMock = vi.fn(async () =>
			({
				ok: false,
				json: async () => ({
					success: false,
					code: 'stale_revision',
					error: 'The deck changed since this page loaded, so its Run buttons no longer match what is on screen. Reload the page and try again.'
				})
			}) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);

		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run code' }));

		expect(await screen.findByText('Deck changed')).toBeInTheDocument();
		expect(screen.queryByText('Error')).not.toBeInTheDocument();
		expect(
			await screen.findByText(
				'The deck changed since this page loaded, so its Run buttons no longer match what is on screen. Reload the page and try again.'
			)
		).toBeInTheDocument();

		// The refusal is not retried automatically: exactly one request was
		// made, and the deck-changed message stays on screen.
		await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
	});

	it('renders tabular results as a table', async () => {
		const fetchMock = vi.fn(async () =>
			({
				ok: true,
				json: async () => ({ success: true, data: [{ id: 1, name: 'Alice' }] })
			}) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);

		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run code' }));

		await waitFor(() => {
			expect(screen.getByRole('table')).toBeInTheDocument();
		});
		expect(screen.getByText('Alice')).toBeInTheDocument();
	});

	it('shows an error result when execution fails', async () => {
		const fetchMock = vi.fn(async () =>
			({ ok: true, json: async () => ({ success: false, error: 'syntax error' }) }) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);

		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run code' }));

		await waitFor(() => {
			expect(screen.getByText('Error')).toBeInTheDocument();
		});
		expect(screen.getByText('syntax error')).toBeInTheDocument();
	});

	it('shows a network error when the fetch itself rejects', async () => {
		const fetchMock = vi.fn(async () => {
			throw new Error('Network down');
		});
		vi.stubGlobal('fetch', fetchMock);

		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run code' }));

		await waitFor(() => {
			expect(screen.getByText('Network down')).toBeInTheDocument();
		});
	});

	it('runs the code on Ctrl+Enter within the block', async () => {
		const fetchMock = vi.fn(async () =>
			({ ok: true, json: async () => ({ success: true, output: 'ok' }) }) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);

		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
		fireEvent.keyDown(screen.getByRole('application'), { key: 'Enter', ctrlKey: true });

		await waitFor(() => {
			expect(fetchMock).toHaveBeenCalledTimes(1);
		});
	});

	describe('with live code status from the server', () => {
		afterEach(() => {
			usePresentationStore.setState({ presentation: null });
		});

		function withLiveCode(drivers: string[]) {
			usePresentationStore.setState({ presentation: { config: {}, slides: [], liveCode: { drivers } } });
		}

		it('shows the Run button for a driver this run allows', async () => {
			withLiveCode(['sqlite']);
			render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
			expect(await screen.findByRole('button', { name: 'Run code' })).toBeEnabled();
		});

		it('shows "Not approved" for a driver this run does not allow', async () => {
			withLiveCode([]);
			render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
			expect(await screen.findByRole('button', { name: 'Not approved' })).toBeDisabled();
			expect(screen.queryByRole('button', { name: 'Run code' })).not.toBeInTheDocument();
		});

		it('does not run on Ctrl+Enter when not approved', async () => {
			withLiveCode([]);
			const fetchMock = vi.fn();
			vi.stubGlobal('fetch', fetchMock);
			render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
			fireEvent.keyDown(screen.getByRole('application'), { key: 'Enter', ctrlKey: true });
			expect(fetchMock).not.toHaveBeenCalled();
		});
	});

	it('shows why a block with an undeclared driver cannot run', async () => {
		const problem = 'This deck does not declare the sqlite driver. Add "sqlite: {}" under drivers in the frontmatter.';
		render(<LiveCodeBlock codeBlock={sqlBlock({ problem })} slideNumber={4} />);
		expect(await screen.findByText(problem)).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Run code' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Not approved' })).not.toBeInTheDocument();
	});

	it("shows the server's reason when it refuses a run", async () => {
		const fetchMock = vi.fn(
			async () =>
				({
					ok: false,
					json: async () => ({ success: false, error: 'Not approved: this deck may not run code with this driver.' })
				}) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);
		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run code' }));
		expect(await screen.findByText('Not approved: this deck may not run code with this driver.')).toBeInTheDocument();
	});
});
