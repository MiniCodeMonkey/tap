import { afterEach, describe, expect, it } from 'vitest';
import { act, cleanup, render } from '@testing-library/react';
import { createRef } from 'react';
import { DeckComponentContext, Step, textOn, useActive, usePrintMode, useStep, useTheme, type ThemeTokens } from './index';

afterEach(() => cleanup());

function StepReadout() {
	const { step, steps } = useStep();
	return (
		<span data-testid="step-readout">
			{step}/{steps}
		</span>
	);
}

function FlagReadout() {
	const printMode = usePrintMode();
	const active = useActive();
	return (
		<span data-testid="flag-readout">
			{String(printMode)}/{String(active)}
		</span>
	);
}

describe('useStep / usePrintMode / useActive', () => {
	it('read step, steps, printMode, and active from context', () => {
		const rootRef = createRef<HTMLElement>();
		const { getByTestId } = render(
			<DeckComponentContext.Provider
				value={{ step: 2, steps: 4, active: true, printMode: false, preview: false, rootRef }}
			>
				<StepReadout />
				<FlagReadout />
			</DeckComponentContext.Provider>
		);

		expect(getByTestId('step-readout').textContent).toBe('2/4');
		expect(getByTestId('flag-readout').textContent).toBe('false/true');
	});
});

function renderStep(contextOverrides: Partial<Parameters<typeof DeckComponentContext.Provider>[0]['value']>, step: JSX.Element) {
	const rootRef = createRef<HTMLElement>();
	return render(
		<DeckComponentContext.Provider
			value={{ step: 0, steps: 0, active: true, printMode: false, preview: false, rootRef, ...contextOverrides }}
		>
			{step}
		</DeckComponentContext.Provider>
	);
}

describe('Step', () => {
	it('at={n}: hidden before the step, visible from it onward', () => {
		const { queryByText, rerender } = renderStep({ step: 1 }, <Step at={2}>shown</Step>);
		expect(queryByText('shown')).toBeNull();

		rerender(
			<DeckComponentContext.Provider
				value={{ step: 2, steps: 2, active: true, printMode: false, preview: false, rootRef: createRef() }}
			>
				<Step at={2}>shown</Step>
			</DeckComponentContext.Provider>
		);
		expect(queryByText('shown')).not.toBeNull();
	});

	it('from/to: visible only within the inclusive range', () => {
		const make = (step: number) => (
			<DeckComponentContext.Provider
				value={{ step, steps: 5, active: true, printMode: false, preview: false, rootRef: createRef() }}
			>
				<Step from={2} to={3}>
					shown
				</Step>
			</DeckComponentContext.Provider>
		);

		const { queryByText, rerender } = render(make(1));
		expect(queryByText('shown')).toBeNull();
		rerender(make(2));
		expect(queryByText('shown')).not.toBeNull();
		rerender(make(3));
		expect(queryByText('shown')).not.toBeNull();
		rerender(make(4));
		expect(queryByText('shown')).toBeNull();
	});

	it('a true print/preview render (step forced to the slide total by the caller) shows a Step at or before that total', () => {
		// True print (?print=true) and a preview both force `step` to the
		// slide's total step count before it ever reaches here (see
		// LayoutComponent.tsx) - Step only ever compares against `step`.
		const { queryByText } = renderStep({ step: 3, steps: 3, printMode: true }, <Step at={3}>shown</Step>);
		expect(queryByText('shown')).not.toBeNull();
	});

	it('a settled capture (printMode true, but NOT forced to the slide total) hides a Step not yet reached at the REQUESTED step', () => {
		// Regression test: a `tap screenshot --step 0` capture on a slide
		// whose deck component settles via printMode must not show a Step
		// gated on a later step just because printMode is true - printMode
		// only says "don't animate", never "show the final step's content".
		const { queryByText } = renderStep({ step: 0, steps: 3, printMode: true }, <Step at={3}>shown</Step>);
		expect(queryByText('shown')).toBeNull();
	});

	it('print mode: a Step whose at exceeds the slide total step count stays hidden', () => {
		const { queryByText } = renderStep({ step: 1, steps: 1, printMode: true }, <Step at={3}>shown</Step>);
		expect(queryByText('shown')).toBeNull();
	});
});

function ThemeReadout() {
	const theme = useTheme();
	return <span data-testid="theme-readout">{JSON.stringify(theme)}</span>;
}

describe('useTheme', () => {
	// The real caller (DeckComponent.tsx) commits its `<div ref={rootRef}>`
	// in an earlier, separate commit from the component that reads
	// useTheme() - it mounts behind a <Suspense> boundary, which only
	// resolves once the bundle import settles. So `rootRef.current` is
	// already populated by the time the consumer's layout effect runs.
	// These tests set `rootRef.current` directly, the same way, instead of
	// mounting the ref owner and the consumer in one React commit, which
	// would attach the ref itself only after a descendant's layout effect.

	it('reads tokens from the nearest [data-theme] ancestor of the component root, empty string for an undefined token', () => {
		const themeHost = document.createElement('div');
		themeHost.setAttribute('data-theme', 'newsprint');
		themeHost.style.setProperty('--bg', '#f2ede1');
		themeHost.style.setProperty('--accent-text', '#980b13');
		document.body.appendChild(themeHost);

		const root = themeHost.appendChild(document.createElement('div'));
		const rootRef = { current: root as HTMLElement | null };

		const { getByTestId } = render(
			<DeckComponentContext.Provider
				value={{ step: 0, steps: 0, active: true, printMode: false, preview: false, rootRef }}
			>
				<ThemeReadout />
			</DeckComponentContext.Provider>,
			{ container: root }
		);

		const tokens = JSON.parse(getByTestId('theme-readout').textContent ?? '{}');
		expect(tokens.bg).toBe('#f2ede1');
		expect(tokens.accentText).toBe('#980b13');
		expect(tokens.radius).toBe('');

		document.body.removeChild(themeHost);
	});

	it('re-reads tokens when the ancestor data-theme attribute changes', async () => {
		const themeHost = document.createElement('div');
		themeHost.setAttribute('data-theme', 'a');
		themeHost.style.setProperty('--bg', 'red');
		document.body.appendChild(themeHost);

		const mountPoint = themeHost.appendChild(document.createElement('div'));
		const rootRef = { current: mountPoint as HTMLElement | null };

		const { getByTestId } = render(
			<DeckComponentContext.Provider
				value={{ step: 0, steps: 0, active: true, printMode: false, preview: false, rootRef }}
			>
				<ThemeReadout />
			</DeckComponentContext.Provider>,
			{ container: mountPoint }
		);

		expect(JSON.parse(getByTestId('theme-readout').textContent ?? '{}').bg).toBe('red');

		await act(async () => {
			themeHost.style.setProperty('--bg', 'blue');
			themeHost.setAttribute('data-theme', 'b');
			await new Promise((resolve) => setTimeout(resolve, 0));
		});

		expect(JSON.parse(getByTestId('theme-readout').textContent ?? '{}').bg).toBe('blue');

		document.body.removeChild(themeHost);
	});
});

describe('textOn', () => {
	const theme: ThemeTokens = {
		bg: '#0f1a16',
		fg: '#dcebe0',
		muted: '',
		accent: '',
		accentText: '',
		surface: '',
		fontDisplay: '',
		fontBody: '',
		fontMono: '',
		ease: '',
		dur: '',
		spaceUnit: '',
		radius: '',
		strokeWidth: ''
	};

	it('picks the dark token for a light fill', () => {
		expect(textOn('#ffffff', theme)).toBe(theme.bg);
	});

	it('picks the light token for a dark fill', () => {
		expect(textOn('#000000', theme)).toBe(theme.fg);
	});

	it('picks theme.fg for an empty fill (tokens before mount)', () => {
		expect(textOn('', theme)).toBe(theme.fg);
	});

	it('picks the dark token for a zine-like bright yellow accent fill', () => {
		expect(textOn('#ffe600', { ...theme, bg: '#e8e5dd', fg: '#0d0d0d' })).toBe('#0d0d0d');
	});

	it('accepts rgb() and rgba() fills', () => {
		expect(textOn('rgb(255, 255, 255)', theme)).toBe(theme.bg);
		expect(textOn('rgba(0, 0, 0, 0.8)', theme)).toBe(theme.fg);
	});

	it('falls back to theme.fg for an unparseable fill', () => {
		expect(textOn('not-a-color', theme)).toBe(theme.fg);
	});

	it('composites a translucent fill over theme.bg before picking, instead of reading its bare channels', () => {
		// The base theme's --surface: rgba(0, 0, 0, 0.02) is near-black by its
		// own channels but reads as near-white composited over a white bg.
		const lightTheme = { ...theme, bg: '#ffffff', fg: '#18181b' };
		expect(textOn('rgba(0, 0, 0, 0.02)', lightTheme)).toBe(lightTheme.fg);
	});
});
