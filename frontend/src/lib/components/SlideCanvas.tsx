/**
 * Fixed 1920x1080 canvas that scales down to fit the window, keeping the
 * slide's aspect ratio. Absolute pixel values give the same layout result
 * on every screen.
 */

import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import type { ThemeColors } from '$lib/types';

export interface SlideCanvasProps {
	/** Aspect ratio in format "16:9", "4:3", or "16:10". */
	aspectRatio?: string;
	/** Theme name, applied as data-theme on the canvas. */
	theme?: string;
	/** Theme color overrides from frontmatter. */
	themeColors?: ThemeColors;
	/** Whether fullscreen mode is active. */
	fullscreen?: boolean;
	/**
	 * True for a PDF export pass (`?print=true`) or a settled screenshot
	 * capture (`tap export images`, without `--wait` - see App.tsx's SETTLE).
	 * Applied as data-print on the same element as data-theme, so a
	 * theme's CSS can turn its own animations and transitions off without
	 * any JS of its own.
	 */
	printMode?: boolean;
	children?: ReactNode;
}

const HEX_COLOR = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/;
const COLOR_FUNCTION = /^(rgb|rgba|hsl|hsla|oklch|oklab|lch|lab)\(/i;
const NAMED_COLORS = new Set([
	'black',
	'white',
	'red',
	'green',
	'blue',
	'yellow',
	'orange',
	'purple',
	'pink',
	'gray',
	'grey',
	'transparent',
	'currentColor',
	'inherit'
]);

// Each themeColors key maps to the base bridge property every theme already
// aliases from its own tokens (see docs/reference/theme-porting.md section
// 4), plus the standard design-spec token name(s) useTheme() reads. Both
// are set on the same element so CSS (which reads --color-*) and deck
// components (which read the design-spec names through useTheme()) see the
// same override. "accent" sets both --accent and --accent-text, since
// themeColors has no separate key for the two accent roles. "codeBg" has
// no design-spec token of its own (--surface is a different, broader
// role), so it only sets its bridge property.
const COLOR_KEY_TO_PROPERTIES: Record<string, string[]> = {
	background: ['--color-bg', '--bg'],
	text: ['--color-text', '--fg'],
	muted: ['--color-muted', '--muted'],
	accent: ['--color-accent', '--accent', '--accent-text'],
	codeBg: ['--color-code-bg']
};

function isValidColor(value: string): boolean {
	return HEX_COLOR.test(value) || COLOR_FUNCTION.test(value) || NAMED_COLORS.has(value);
}

function parseAspectRatio(ratio: string): number {
	const [width, height] = ratio.split(':').map(Number);
	if (!width || !height || Number.isNaN(width) || Number.isNaN(height)) {
		return 16 / 9;
	}
	return width / height;
}

function getCSSAspectRatio(ratio: string): string {
	const [width, height] = ratio.split(':').map(Number);
	if (!width || !height || Number.isNaN(width) || Number.isNaN(height)) {
		return '16 / 9';
	}
	return `${width} / ${height}`;
}

function buildColorOverrideStyle(themeColors: ThemeColors | undefined): CSSProperties {
	if (!themeColors) {
		return {};
	}

	const style: Record<string, string> = {};

	for (const [key, value] of Object.entries(themeColors)) {
		if (!value) continue;

		const cssProperties = COLOR_KEY_TO_PROPERTIES[key];
		if (!cssProperties) {
			console.warn(`[tap] Invalid themeColors key "${key}". Valid keys: ${Object.keys(COLOR_KEY_TO_PROPERTIES).join(', ')}`);
			continue;
		}

		if (!isValidColor(value)) {
			console.warn(`[tap] Invalid color value "${value}" for themeColors.${key}. Skipping.`);
			continue;
		}

		for (const cssProperty of cssProperties) {
			style[cssProperty] = value;
		}
	}

	return style as CSSProperties;
}

export function SlideCanvas({
	aspectRatio = '16:9',
	theme = 'base',
	themeColors,
	fullscreen = false,
	printMode = false,
	children
}: SlideCanvasProps) {
	const containerRef = useRef<HTMLDivElement>(null);
	const [scale, setScale] = useState(1);

	const numericRatio = parseAspectRatio(aspectRatio);
	const cssAspectRatio = getCSSAspectRatio(aspectRatio);

	useEffect(() => {
		const container = containerRef.current;
		if (!container) return;

		const baseWidth = 1920;
		const baseHeight = baseWidth / numericRatio;

		function calculateScale(): void {
			if (!container) return;
			const rect = container.getBoundingClientRect();
			if (rect.width === 0 || rect.height === 0) return;
			setScale(Math.min(rect.width / baseWidth, rect.height / baseHeight));
		}

		calculateScale();

		const resizeObserver = new ResizeObserver(calculateScale);
		resizeObserver.observe(container);
		window.addEventListener('resize', calculateScale);

		return () => {
			resizeObserver.disconnect();
			window.removeEventListener('resize', calculateScale);
		};
	}, [numericRatio, fullscreen]);

	const colorOverrideStyle = buildColorOverrideStyle(themeColors);

	return (
		<div ref={containerRef} className={`slide-container${fullscreen ? ' fullscreen' : ''}`}>
			<div
				className="slide"
				data-theme={theme}
				data-print={printMode ? 'true' : undefined}
				style={{
					aspectRatio: cssAspectRatio,
					width: '1920px',
					height: `${1920 / numericRatio}px`,
					// .slide-container is a flex container and this frame is its
					// direct child; without flex: none, the default flex-shrink: 1
					// lets the browser shrink this frame below its inline
					// width/height whenever the container is smaller than the base
					// canvas (any viewport under 1920x1080, every presenter panel,
					// every overview thumbnail). Laying out at the full inline size
					// always, and leaving all the shrinking to the scale() transform
					// below (computed from the container's actual measured size), is
					// what keeps the aspect ratio intact at every container size.
					flex: 'none',
					transform: `scale(${scale})`,
					// data-theme lives on this frame, not on .slide-container, so a
					// theme's background (set through --color-bg here, the same
					// element the theme rule targets) never paints past the fixed
					// 1920x1080 canvas into the letterbox area around it - that area
					// is always .slide-container's own black background instead (see
					// app.css). themeColors overrides go on this same element for the
					// same reason: an override on an ancestor would only be
					// inherited here, and this frame's own theme rule (an
					// explicitly-set property beats an inherited one, regardless of
					// specificity) would win over it otherwise.
					...colorOverrideStyle
				}}
			>
				{children}
			</div>
		</div>
	);
}
