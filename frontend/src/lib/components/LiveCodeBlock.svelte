<script lang="ts">
	import type { CodeBlock, ExecuteRequest, ExecuteResponse } from '$lib/types';
	import { highlight, type HighlightOptions } from '$lib/utils/highlighting';
	import { staticMode } from '$lib/stores/websocket';
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

	/** Current static mode value */
	let isStaticMode = $state(false);

	// ============================================================================
	// Computed Values
	// ============================================================================

	/** Whether this code block has a driver and can be executed */
	let hasDriver = $derived(!!codeBlock.driver);

	/** Whether execution is available (has driver and not in static mode) */
	let canExecute = $derived(hasDriver && !isStaticMode);

	/** Whether to show the static mode placeholder */
	let showStaticPlaceholder = $derived(hasDriver && isStaticMode);

	// ============================================================================
	// Lifecycle
	// ============================================================================

	onMount(() => {
		highlightCode();
		setupKeyboardShortcut();

		// Subscribe to static mode store
		const unsubscribe = staticMode.subscribe((value) => {
			isStaticMode = value;
		});

		return () => {
			cleanupKeyboardShortcut();
			unsubscribe();
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
	class:static-mode={showStaticPlaceholder}
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

		{#if showStaticPlaceholder}
			<div class="static-placeholder">
				<span class="static-placeholder-icon" aria-hidden="true">&#9889;</span>
				<span class="static-placeholder-text">Live execution available in presentation mode</span>
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
