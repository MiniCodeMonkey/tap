/**
 * Asciinema player utilities for rendering terminal recordings in slides.
 * Finds <pre><code class="language-asciinema"> blocks and replaces them
 * with interactive asciinema-player instances.
 */

/** CDN URLs for asciinema-player */
const ASCIINEMA_PLAYER_JS =
	'https://cdn.jsdelivr.net/npm/asciinema-player@3.9.0/dist/bundle/asciinema-player.min.js';
const ASCIINEMA_PLAYER_CSS =
	'https://cdn.jsdelivr.net/npm/asciinema-player@3.9.0/dist/bundle/asciinema-player.min.css';

/** Available playback speeds */
const SPEED_OPTIONS = [0.5, 1.0, 1.5, 2.0, 3.0];

/** Track loaded state */
let libraryLoaded = false;

/** Parsed asciinema config from code block content */
interface AsciinemaConfig {
	src: string;
	autoPlay?: boolean;
	loop?: boolean;
	speed?: number;
	startAt?: number;
	cols?: number;
	rows?: number;
	idleTimeLimit?: number;
	fit?: string;
	poster?: string;
	controls?: boolean;
}

/**
 * Load the asciinema-player library from CDN.
 */
async function loadLibrary(): Promise<void> {
	if (libraryLoaded && (window as any).AsciinemaPlayer) {
		return;
	}

	// Load CSS if not present
	if (!document.querySelector(`link[href="${ASCIINEMA_PLAYER_CSS}"]`)) {
		const link = document.createElement('link');
		link.rel = 'stylesheet';
		link.href = ASCIINEMA_PLAYER_CSS;
		document.head.appendChild(link);
	}

	// Load JS if not present
	const existingScript = document.querySelector(`script[src="${ASCIINEMA_PLAYER_JS}"]`);
	if (!existingScript) {
		await new Promise<void>((resolve, reject) => {
			const script = document.createElement('script');
			script.src = ASCIINEMA_PLAYER_JS;
			script.async = true;
			script.onload = () => resolve();
			script.onerror = () => reject(new Error('Failed to load asciinema-player library'));
			document.head.appendChild(script);
		});
	} else {
		// Wait for existing script to finish loading
		await new Promise<void>((resolve) => {
			const checkLoaded = () => {
				if ((window as any).AsciinemaPlayer) {
					resolve();
				} else {
					setTimeout(checkLoaded, 50);
				}
			};
			checkLoaded();
		});
	}

	libraryLoaded = true;
}

/**
 * Parse YAML-like config from code block content.
 * Content is lines like "src: ./demo.cast" or "autoPlay: true".
 */
function parseConfig(content: string): AsciinemaConfig | null {
	const config: Record<string, string> = {};

	const lines = content.split('\n');
	for (const line of lines) {
		const trimmed = line.trim();
		if (!trimmed) continue;

		// Support both "key: value" and "key: \"value\"" formats
		const colonIdx = trimmed.indexOf(':');
		if (colonIdx === -1) continue;

		const key = trimmed.slice(0, colonIdx).trim();
		let value = trimmed.slice(colonIdx + 1).trim();

		// Strip surrounding quotes
		if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
			value = value.slice(1, -1);
		}

		config[key] = value;
	}

	if (!config.src) {
		return null;
	}

	const result: AsciinemaConfig = { src: config.src };

	if (config.autoPlay !== undefined) result.autoPlay = config.autoPlay === 'true';
	if (config.loop !== undefined) result.loop = config.loop === 'true';
	if (config.speed !== undefined) result.speed = parseFloat(config.speed);
	if (config.startAt !== undefined) result.startAt = parseFloat(config.startAt);
	if (config.cols !== undefined) result.cols = parseInt(config.cols, 10);
	if (config.rows !== undefined) result.rows = parseInt(config.rows, 10);
	if (config.idleTimeLimit !== undefined) result.idleTimeLimit = parseFloat(config.idleTimeLimit);
	if (config.fit !== undefined) result.fit = config.fit;
	if (config.poster !== undefined) result.poster = config.poster;
	if (config.controls !== undefined) result.controls = config.controls === 'true';

	return result;
}

/**
 * Create controls overlay for the player.
 */
function createControls(
	player: any,
	config: AsciinemaConfig
): HTMLElement {
	const overlay = document.createElement('div');
	overlay.className = 'controls-overlay';

	let isPlaying = config.autoPlay ?? false;
	let currentSpeed = config.speed ?? 1.0;

	// Play/Pause button
	const playPauseBtn = document.createElement('button');
	playPauseBtn.className = 'control-button play-pause';
	playPauseBtn.title = isPlaying ? 'Pause' : 'Play';
	playPauseBtn.textContent = isPlaying ? '\u23F8' : '\u25B7';
	playPauseBtn.addEventListener('click', () => {
		if (isPlaying) {
			player.pause();
			isPlaying = false;
		} else {
			player.play();
			isPlaying = true;
		}
		playPauseBtn.textContent = isPlaying ? '\u23F8' : '\u25B7';
		playPauseBtn.title = isPlaying ? 'Pause' : 'Play';
	});

	// Speed button
	const speedBtn = document.createElement('button');
	speedBtn.className = 'control-button speed';
	speedBtn.title = `Playback speed: ${currentSpeed}x`;
	speedBtn.textContent = `${currentSpeed}x`;
	speedBtn.addEventListener('click', () => {
		const currentIndex = SPEED_OPTIONS.indexOf(currentSpeed);
		const nextIndex = (currentIndex + 1) % SPEED_OPTIONS.length;
		currentSpeed = SPEED_OPTIONS[nextIndex] ?? 1.0;
		speedBtn.textContent = `${currentSpeed}x`;
		speedBtn.title = `Playback speed: ${currentSpeed}x`;
		// asciinema-player doesn't support runtime speed changes,
		// so we'd need to re-create the player. For now just update the display.
	});

	overlay.appendChild(playPauseBtn);
	overlay.appendChild(speedBtn);
	return overlay;
}

/**
 * Find and render all asciinema code blocks within an element.
 * Replaces <pre><code class="language-asciinema"> blocks with asciinema players.
 */
export async function renderAsciinemaBlocksInElement(element: HTMLElement): Promise<void> {
	const codeBlocks = element.querySelectorAll<HTMLElement>('pre > code.language-asciinema');

	if (codeBlocks.length === 0) {
		return;
	}

	try {
		await loadLibrary();
	} catch (err) {
		console.error('Failed to load asciinema-player library:', err);
		// Show error in each block
		codeBlocks.forEach((codeBlock) => {
			const pre = codeBlock.parentElement;
			if (!pre) return;
			const errorDiv = document.createElement('div');
			errorDiv.className = 'asciinema-player-wrapper error';
			errorDiv.innerHTML =
				'<div class="error-state">' +
				'<span class="error-icon">\u26A0</span>' +
				'<span class="error-text">Failed to load asciinema player library</span>' +
				'</div>';
			pre.replaceWith(errorDiv);
		});
		return;
	}

	const AsciinemaPlayer = (window as any).AsciinemaPlayer;
	if (!AsciinemaPlayer) return;

	for (const codeBlock of Array.from(codeBlocks)) {
		const pre = codeBlock.parentElement;
		if (!pre) continue;

		// Decode HTML entities in code content
		const rawContent = codeBlock.textContent ?? '';
		const config = parseConfig(rawContent);

		if (!config) {
			// Show error for missing src
			const errorDiv = document.createElement('div');
			errorDiv.className = 'asciinema-player-wrapper error';
			errorDiv.innerHTML =
				'<div class="error-state">' +
				'<span class="error-icon">\u26A0</span>' +
				'<span class="error-text">Missing src in asciinema config</span>' +
				'<span class="error-hint">Use: ```asciinema {src: "./recording.cast"}</span>' +
				'</div>';
			pre.replaceWith(errorDiv);
			continue;
		}

		// Create wrapper
		const wrapper = document.createElement('div');
		wrapper.className = 'asciinema-player-wrapper';

		// Create player container (matches existing CSS in ui-components.css)
		const playerContainer = document.createElement('div');
		playerContainer.className = 'player-container';
		wrapper.appendChild(playerContainer);

		// Replace the pre block
		pre.replaceWith(wrapper);

		try {
			// Build player options
			const showControls = config.controls ?? false;
			const options: Record<string, any> = {
				autoPlay: config.autoPlay ?? false,
				speed: config.speed ?? 1.0,
				loop: config.loop ?? false,
				preload: true,
				fit: config.fit === 'none' ? false : (config.fit ?? 'width'),
				controls: showControls,
				theme: 'monokai'
			};

			if (config.startAt) options.startAt = config.startAt;
			if (config.cols) options.cols = config.cols;
			if (config.rows) options.rows = config.rows;
			if (config.idleTimeLimit) options.idleTimeLimit = config.idleTimeLimit;
			if (config.poster) options.poster = config.poster;

			const player = AsciinemaPlayer.create(config.src, playerContainer, options);

			// Add custom controls
			const controls = createControls(player, config);
			wrapper.appendChild(controls);
		} catch (err) {
			const errorDiv = document.createElement('div');
			errorDiv.className = 'asciinema-player-wrapper error';
			errorDiv.innerHTML =
				'<div class="error-state">' +
				`<span class="error-icon">\u26A0</span>` +
				`<span class="error-text">Failed to create asciinema player: ${err instanceof Error ? err.message : 'Unknown error'}</span>` +
				`<span class="error-hint">Check that ${config.src} exists and is accessible.</span>` +
				'</div>';
			wrapper.replaceWith(errorDiv);
		}
	}
}
