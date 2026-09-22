/**
 * Renders a code block that can be executed against a configured driver.
 * Highlights its own code on mount (it is mounted into a portal in place of
 * the plain `<pre>` before the slide's general highlighting pass runs, so it
 * never gets highlighted twice), shows a run control when live execution is
 * available, and renders the result returned by `POST /api/execute`, which
 * it sends only the slide and block number, never the code.
 */

import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import type { CodeBlock, ExecuteRequest, ExecuteResponse } from '$lib/types';
import { highlight } from '../utils/highlighting';
import { selectLiveExecutionAvailable, useConnectionStore } from '../stores/websocket';
import { usePresentationStore } from '../stores/presentation';

export interface LiveCodeBlockProps {
	/** The code block data, including the driver to execute it against, if any. */
	codeBlock: CodeBlock;
	/** The number of the slide the block is on, counted from 1. */
	slideNumber: number;
}

function escapeHtml(text: string): string {
	return text
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#039;');
}

/** Format tabular execution results as an HTML table. */
function formatTableData(data: Record<string, unknown>[]): string {
	if (!data || data.length === 0) return '';

	const columns = Object.keys(data[0] ?? {});
	if (columns.length === 0) return '';

	let html = '<table class="result-table">';

	html += '<thead><tr>';
	for (const column of columns) {
		html += `<th>${escapeHtml(column)}</th>`;
	}
	html += '</tr></thead>';

	html += '<tbody>';
	for (const row of data) {
		html += '<tr>';
		for (const column of columns) {
			const value = row[column];
			const displayValue = value === null || value === undefined ? 'NULL' : String(value);
			html += `<td>${escapeHtml(displayValue)}</td>`;
		}
		html += '</tr>';
	}
	html += '</tbody>';

	html += '</table>';
	return html;
}

export function LiveCodeBlock({ codeBlock, slideNumber }: LiveCodeBlockProps) {
	const [isExecuting, setIsExecuting] = useState(false);
	const [result, setResult] = useState<ExecuteResponse | null>(null);
	const [hasError, setHasError] = useState(false);
	const [highlightedCode, setHighlightedCode] = useState('');
	const containerRef = useRef<HTMLDivElement>(null);

	const liveExecutionAvailable = useConnectionStore(selectLiveExecutionAvailable);
	const liveCode = usePresentationStore((state) => state.presentation?.liveCode);

	const hasDriver = !!codeBlock.driver;
	const problem = hasDriver && liveExecutionAvailable ? codeBlock.problem : undefined;
	// A server that sends no live code status (tap export's) keeps the Run
	// button; /api/execute still refuses anything it may not run.
	const approved = !liveCode || (!!codeBlock.driver && liveCode.drivers.includes(codeBlock.driver));
	const runnable = hasDriver && liveExecutionAvailable && !problem && codeBlock.block !== undefined;
	const canExecute = runnable && approved;
	const notApproved = runnable && !approved;
	const showStaticPlaceholder = hasDriver && !liveExecutionAvailable;

	useEffect(() => {
		let cancelled = false;

		void (async () => {
			try {
				const html = await highlight(codeBlock.code, { language: codeBlock.language });
				if (!cancelled) setHighlightedCode(html);
			} catch {
				if (!cancelled) {
					setHighlightedCode(`<pre><code>${escapeHtml(codeBlock.code)}</code></pre>`);
				}
			}
		})();

		return () => {
			cancelled = true;
		};
	}, [codeBlock.code, codeBlock.language]);

	const executeCode = useCallback(async () => {
		if (!canExecute || isExecuting) return;

		setIsExecuting(true);
		setHasError(false);
		setResult(null);

		const request: ExecuteRequest = { slide: slideNumber, block: codeBlock.block! };

		try {
			const response = await fetch('/api/execute', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(request)
			});

			const data: ExecuteResponse = await response.json();
			setResult(data);
			setHasError(!data.success);
		} catch (err) {
			setResult({
				success: false,
				error: err instanceof Error ? err.message : 'Network error occurred'
			});
			setHasError(true);
		} finally {
			setIsExecuting(false);
		}
	}, [canExecute, isExecuting, codeBlock, slideNumber]);

	const handleKeyDown = useCallback(
		(event: KeyboardEvent<HTMLDivElement>) => {
			if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
				event.preventDefault();
				void executeCode();
			}
		},
		[executeCode]
	);

	const formattedResult = useMemo(() => {
		if (!result) return '';

		if (result.data && result.data.length > 0) {
			return formatTableData(result.data);
		}

		if (result.error) {
			return `<pre class="result-error">${escapeHtml(result.error)}</pre>`;
		}

		if (result.output) {
			return `<pre class="result-output">${escapeHtml(result.output)}</pre>`;
		}

		return '<span class="result-empty">No output</span>';
	}, [result]);

	const classes = [
		'live-code-block',
		canExecute && 'can-execute',
		isExecuting && 'is-executing',
		hasError && 'has-error',
		showStaticPlaceholder && 'static-mode'
	]
		.filter(Boolean)
		.join(' ');

	return (
		<div
			className={classes}
			ref={containerRef}
			tabIndex={canExecute ? 0 : -1}
			role="application"
			aria-label="Live code block with execution capability"
			onKeyDown={canExecute ? handleKeyDown : undefined}
		>
			<div className="code-container">
				<div className="code-highlight" dangerouslySetInnerHTML={{ __html: highlightedCode }} />

				{canExecute && (
					<div className="code-actions">
						<button
							className="run-button"
							onClick={() => void executeCode()}
							disabled={isExecuting}
							aria-label="Run code"
							title="Run code (Ctrl/Cmd+Enter)"
						>
							{isExecuting ? (
								<>
									<span className="loading-spinner" aria-hidden="true" />
									Running...
								</>
							) : (
								<>
									<span className="play-icon" aria-hidden="true">
										&#9655;
									</span>
									Run
								</>
							)}
						</button>
					</div>
				)}

				{notApproved && (
					<div className="code-actions">
						<button
							className="run-button not-approved"
							disabled
							title="Approve this deck when tap dev or tap present asks at startup, or pass --allow-code"
						>
							Not approved
						</button>
					</div>
				)}

				{showStaticPlaceholder && (
					<div className="static-placeholder">
						<span className="static-placeholder-icon" aria-hidden="true">
							&#9889;
						</span>
						<span className="static-placeholder-text">Live execution available in presentation mode</span>
					</div>
				)}
			</div>

			{problem && (
				<pre className="live-code-problem" role="note">
					{problem}
				</pre>
			)}

			{result && (
				<div className={`result-container${hasError ? ' error' : ''}`}>
					<div className="result-header">
						{hasError ? (
							<span className="result-status error">Error</span>
						) : (
							<span className="result-status success">Output</span>
						)}
					</div>
					<div className="result-content" dangerouslySetInnerHTML={{ __html: formattedResult }} />
				</div>
			)}
		</div>
	);
}
