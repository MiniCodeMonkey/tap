<script lang="ts">
	import type { CodeBlock, ExecuteRequest, ExecuteResponse } from '$lib/types';
	import { highlight, type HighlightOptions } from '$lib/utils/highlighting';
	import type { BundledTheme } from 'shiki';
	import { onMount } from 'svelte';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** The code block data */
		codeBlock: CodeBlock;
		/** Optional code theme for syntax highlighting */
		theme?: string;
	}

	let { codeBlock, theme }: Props = $props();

	// ============================================================================
	// State
	// ============================================================================

	/** Whether the code is currently executing */
	let isExecuting = $state(false);

	/** Execution result from the API */
	let result = $state<ExecuteResponse | null>(null);

	/** Whether there was an error during execution */
	let hasError = $state(false);

	/** Highlighted HTML for the code block */
	let highlightedCode = $state('');

	/** Reference to the container element for keyboard handling */
	let containerRef: HTMLElement | undefined = $state();

	// ============================================================================
	// Computed Values
	// ============================================================================

	/** Whether this code block has a driver and can be executed */
	let canExecute = $derived(!!codeBlock.driver);

	// ============================================================================
	// Lifecycle
	// ============================================================================

	onMount(() => {
		highlightCode();
		setupKeyboardShortcut();

		return () => {
			cleanupKeyboardShortcut();
		};
	});

	// ============================================================================
	// Syntax Highlighting
	// ============================================================================

	/**
	 * Highlight the code block content.
	 */
	async function highlightCode(): Promise<void> {
		try {
			const options: HighlightOptions = {
				language: codeBlock.language
			};
			if (theme) {
				options.theme = theme as BundledTheme;
			}
			highlightedCode = await highlight(codeBlock.code, options);
		} catch {
			// Fallback to plain text if highlighting fails
			highlightedCode = `<pre><code>${escapeHtml(codeBlock.code)}</code></pre>`;
		}
	}

	/**
	 * Escape HTML special characters for safe rendering.
	 */
	function escapeHtml(text: string): string {
		return text
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;')
			.replace(/'/g, '&#039;');
	}

	// ============================================================================
	// Code Execution
	// ============================================================================

	/**
	 * Execute the code via the API.
	 */
	async function executeCode(): Promise<void> {
		if (!canExecute || isExecuting) return;

		isExecuting = true;
		hasError = false;
		result = null;

		const request: ExecuteRequest = {
			driver: codeBlock.driver!,
			code: codeBlock.code,
			connection: codeBlock.connection
		};

		try {
			const response = await fetch('/api/execute', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify(request)
			});

			const data: ExecuteResponse = await response.json();
			result = data;
			hasError = !data.success;
		} catch (err) {
			result = {
				success: false,
				error: err instanceof Error ? err.message : 'Network error occurred'
			};
			hasError = true;
		} finally {
			isExecuting = false;
		}
	}

	// ============================================================================
	// Keyboard Handling
	// ============================================================================

	/**
	 * Handle keyboard shortcuts for code execution.
	 */
	function handleKeyDown(event: KeyboardEvent): void {
		// Ctrl+Enter or Cmd+Enter to execute
		if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
			event.preventDefault();
			executeCode();
		}
	}

	/**
	 * Set up global keyboard shortcut listener.
	 */
	function setupKeyboardShortcut(): void {
		if (containerRef && canExecute) {
			containerRef.addEventListener('keydown', handleKeyDown);
		}
	}

	/**
	 * Clean up keyboard shortcut listener.
	 */
	function cleanupKeyboardShortcut(): void {
		if (containerRef) {
			containerRef.removeEventListener('keydown', handleKeyDown);
		}
	}

	// ============================================================================
	// Result Formatting
	// ============================================================================

	/**
	 * Format tabular data as an HTML table.
	 */
	function formatTableData(data: Record<string, unknown>[]): string {
		if (!data || data.length === 0) return '';

		// Get column names from first row
		const columns = Object.keys(data[0] ?? {});
		if (columns.length === 0) return '';

		// Build table HTML
		let html = '<table class="result-table">';

		// Header row
		html += '<thead><tr>';
		for (const col of columns) {
			html += `<th>${escapeHtml(col)}</th>`;
		}
		html += '</tr></thead>';

		// Data rows
		html += '<tbody>';
		for (const row of data) {
			html += '<tr>';
			for (const col of columns) {
				const value = row[col];
				const displayValue = value === null || value === undefined ? 'NULL' : String(value);
				html += `<td>${escapeHtml(displayValue)}</td>`;
			}
			html += '</tr>';
		}
		html += '</tbody>';

		html += '</table>';
		return html;
	}

	/**
	 * Get the formatted result HTML.
	 */
	let formattedResult = $derived.by(() => {
		if (!result) return '';

		// If there's tabular data, format as table
		if (result.data && result.data.length > 0) {
			return formatTableData(result.data);
		}

		// Otherwise, show text output or error
		if (result.error) {
			return `<pre class="result-error">${escapeHtml(result.error)}</pre>`;
		}

		if (result.output) {
			return `<pre class="result-output">${escapeHtml(result.output)}</pre>`;
		}

		return '<span class="result-empty">No output</span>';
	});
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<div
	class="live-code-block"
	class:can-execute={canExecute}
	class:is-executing={isExecuting}
	class:has-error={hasError}
	bind:this={containerRef}
	tabindex={canExecute ? 0 : -1}
	role="application"
	aria-label="Live code block with execution capability"
>
	<div class="code-container">
		{@html highlightedCode}

		{#if canExecute}
			<div class="code-actions">
				<button
					class="run-button"
					onclick={executeCode}
					disabled={isExecuting}
					aria-label="Run code"
					title="Run code (Ctrl/Cmd+Enter)"
				>
					{#if isExecuting}
						<span class="loading-spinner" aria-hidden="true"></span>
						Running...
					{:else}
						<span class="play-icon" aria-hidden="true">&#9655;</span>
						Run
					{/if}
				</button>
			</div>
		{/if}
	</div>

	{#if result}
		<div class="result-container" class:error={hasError}>
			<div class="result-header">
				{#if hasError}
					<span class="result-status error">Error</span>
				{:else}
					<span class="result-status success">Output</span>
				{/if}
			</div>
			<div class="result-content">
				{@html formattedResult}
			</div>
		</div>
	{/if}
</div>

<style>
	.live-code-block {
		position: relative;
		margin: 0 0 0.75em;
		border-radius: 8px;
		overflow: hidden;
	}

	.live-code-block:focus {
		outline: 2px solid var(--accent-color, #7c3aed);
		outline-offset: 2px;
	}

	.code-container {
		position: relative;
	}

	/* Override pre styles from SlideRenderer */
	.code-container :global(pre) {
		margin: 0;
		border-radius: 8px 8px 0 0;
	}

	.live-code-block.can-execute .code-container :global(pre) {
		border-radius: 8px 8px 0 0;
	}

	/* Code actions (Run button) */
	.code-actions {
		position: absolute;
		top: 0.75em;
		right: 0.75em;
		z-index: 10;
	}

	.run-button {
		display: inline-flex;
		align-items: center;
		gap: 0.5em;
		padding: 0.5em 1em;
		font-size: 0.875rem;
		font-weight: 500;
		color: #fff;
		background-color: var(--accent-color, #7c3aed);
		border: none;
		border-radius: 6px;
		cursor: pointer;
		transition:
			background-color 0.2s ease,
			transform 0.1s ease;
	}

	.run-button:hover:not(:disabled) {
		background-color: var(--accent-hover, #6d28d9);
		transform: translateY(-1px);
	}

	.run-button:active:not(:disabled) {
		transform: translateY(0);
	}

	.run-button:disabled {
		opacity: 0.7;
		cursor: not-allowed;
	}

	.play-icon {
		font-size: 0.75em;
	}

	/* Loading spinner */
	.loading-spinner {
		display: inline-block;
		width: 14px;
		height: 14px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top-color: #fff;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	/* Result container */
	.result-container {
		background-color: var(--result-bg, #1a1a1a);
		border-top: 1px solid var(--result-border, #333);
		border-radius: 0 0 8px 8px;
	}

	.result-header {
		padding: 0.5em 1em;
		border-bottom: 1px solid var(--result-border, #333);
	}

	.result-status {
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.result-status.success {
		color: var(--success-color, #10b981);
	}

	.result-status.error {
		color: var(--error-color, #ef4444);
	}

	.result-content {
		padding: 1em;
		overflow-x: auto;
	}

	/* Result table */
	.result-content :global(.result-table) {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.875rem;
		color: var(--result-text, #e5e5e5);
	}

	.result-content :global(.result-table th),
	.result-content :global(.result-table td) {
		padding: 0.5em 0.75em;
		text-align: left;
		border-bottom: 1px solid var(--result-border, #333);
	}

	.result-content :global(.result-table th) {
		font-weight: 600;
		background-color: var(--result-header-bg, rgba(255, 255, 255, 0.05));
		color: var(--result-header-text, #fff);
	}

	.result-content :global(.result-table tr:last-child td) {
		border-bottom: none;
	}

	.result-content :global(.result-table tr:hover td) {
		background-color: rgba(255, 255, 255, 0.02);
	}

	/* Result output (text) */
	.result-content :global(.result-output) {
		margin: 0;
		padding: 0;
		background: none;
		color: var(--result-text, #e5e5e5);
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		font-size: 0.875rem;
		white-space: pre-wrap;
		word-break: break-word;
	}

	/* Result error */
	.result-content :global(.result-error) {
		margin: 0;
		padding: 0;
		background: none;
		color: var(--error-color, #ef4444);
		font-family: var(--mono-font-family, 'SF Mono', Monaco, Consolas, monospace);
		font-size: 0.875rem;
		white-space: pre-wrap;
		word-break: break-word;
	}

	/* Result empty */
	.result-content :global(.result-empty) {
		color: var(--muted-color, #666);
		font-style: italic;
	}

	/* Error state */
	.result-container.error {
		border-top-color: var(--error-color, #ef4444);
	}

	/* Theme-specific styles */
	:global(.theme-terminal) .run-button {
		background-color: var(--terminal-green, #00ff00);
		color: #000;
	}

	:global(.theme-terminal) .run-button:hover:not(:disabled) {
		background-color: var(--terminal-green-bright, #00ff88);
	}

	:global(.theme-brutalist) .run-button {
		background-color: #000;
		border: 3px solid #000;
		border-radius: 0;
		text-transform: uppercase;
		font-weight: 700;
	}

	:global(.theme-brutalist) .run-button:hover:not(:disabled) {
		background-color: var(--brutalist-red, #ef4444);
		color: #fff;
	}

	:global(.theme-keynote) .run-button {
		background: linear-gradient(135deg, #0066cc, #0055aa);
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
	}

	:global(.theme-keynote) .run-button:hover:not(:disabled) {
		background: linear-gradient(135deg, #0077ee, #0066cc);
	}

	/* Reduced motion support */
	@media (prefers-reduced-motion: reduce) {
		.run-button {
			transition: none;
		}

		.loading-spinner {
			animation: none;
		}
	}
</style>
