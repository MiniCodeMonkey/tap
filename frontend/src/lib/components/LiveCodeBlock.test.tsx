import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { LiveCodeBlock } from './LiveCodeBlock';
import { useConnectionStore } from '../stores/websocket';
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
		const { container } = render(<LiveCodeBlock codeBlock={sqlBlock()} />);

		await waitFor(() => {
			expect(container.querySelector('.code-highlight pre code')).toHaveTextContent('SELECT 1;');
		});
	});

	it('shows a run button when live execution is available for a driver block', async () => {
		render(<LiveCodeBlock codeBlock={sqlBlock()} />);
		expect(await screen.findByRole('button', { name: 'Run code' })).toBeInTheDocument();
	});

	it('does not show a run button for a code block with no driver', async () => {
		render(<LiveCodeBlock codeBlock={sqlBlock({ driver: undefined })} />);
		await waitFor(() => expect(document.querySelector('.code-highlight')?.innerHTML).not.toBe(''));
		expect(screen.queryByRole('button', { name: 'Run code' })).not.toBeInTheDocument();
	});

	it('shows the static placeholder instead of a run button when live execution is unavailable', async () => {
		useConnectionStore.setState({ connected: false, staticMode: true });
		render(<LiveCodeBlock codeBlock={sqlBlock()} />);

		expect(await screen.findByText('Live execution available in presentation mode')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Run code' })).not.toBeInTheDocument();
	});

	it('sends the code, driver and connection to POST /api/execute when run', async () => {
		const fetchMock = vi.fn(async () =>
			({ ok: true, json: async () => ({ success: true, output: 'ok' }) }) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);

		render(<LiveCodeBlock codeBlock={sqlBlock()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run code' }));

		await waitFor(() => {
			expect(fetchMock).toHaveBeenCalledWith(
				'/api/execute',
				expect.objectContaining({
					method: 'POST',
					body: JSON.stringify({ driver: 'sqlite', code: 'SELECT 1;', connection: 'demo' })
				})
			);
		});
	});

	it('renders tabular results as a table', async () => {
		const fetchMock = vi.fn(async () =>
			({
				ok: true,
				json: async () => ({ success: true, data: [{ id: 1, name: 'Alice' }] })
			}) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);

		render(<LiveCodeBlock codeBlock={sqlBlock()} />);
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

		render(<LiveCodeBlock codeBlock={sqlBlock()} />);
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

		render(<LiveCodeBlock codeBlock={sqlBlock()} />);
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

		render(<LiveCodeBlock codeBlock={sqlBlock()} />);
		fireEvent.keyDown(screen.getByRole('application'), { key: 'Enter', ctrlKey: true });

		await waitFor(() => {
			expect(fetchMock).toHaveBeenCalledTimes(1);
		});
	});
});
