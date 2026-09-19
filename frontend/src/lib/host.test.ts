import { afterEach, describe, expect, it } from 'vitest';
import { HOST_MODULE_NAMES, installHost } from './host';

/**
 * Pins the exact module names a deck component's bundle may import from
 * the host, mirroring internal/components/host_shims.go's
 * hostModuleNames. Written out explicitly (not derived from HOST_MODULE_NAMES)
 * so a change to either list is caught here rather than the test quietly
 * tracking whatever the implementation currently says.
 */
const EXPECTED_HOST_MODULE_NAMES = [
	'react',
	'react/jsx-runtime',
	'react-dom',
	'react-dom/client',
	'motion',
	'motion/react',
	'tap'
];

afterEach(() => {
	// @ts-expect-error - test cleanup of a window global installHost sets.
	delete window.__TAP_HOST__;
});

describe('HOST_MODULE_NAMES', () => {
	it('matches the Go side exactly', () => {
		expect([...HOST_MODULE_NAMES].sort()).toEqual([...EXPECTED_HOST_MODULE_NAMES].sort());
	});
});

describe('installHost', () => {
	it('assigns window.__TAP_HOST__ with exactly the pinned module names', () => {
		installHost();
		expect(Object.keys(window.__TAP_HOST__).sort()).toEqual([...EXPECTED_HOST_MODULE_NAMES].sort());
	});

	it('gives every host entry a default export, for `import Something from "<module>"` style imports', () => {
		installHost();
		for (const name of EXPECTED_HOST_MODULE_NAMES) {
			const entry = window.__TAP_HOST__[name] as { default?: unknown };
			expect(entry.default, `${name} has no default export`).toBeDefined();
		}
	});

	it('exposes react as both a named-export namespace and via default, for named and default imports alike', () => {
		installHost();
		const react = window.__TAP_HOST__['react'] as { createElement?: unknown; default?: { createElement?: unknown } };
		expect(typeof react.createElement).toBe('function');
		expect(typeof react.default?.createElement).toBe('function');
	});

	it('exposes the tap helper module with its documented exports', () => {
		installHost();
		const tap = window.__TAP_HOST__['tap'] as Record<string, unknown>;
		expect(typeof tap.useStep).toBe('function');
		expect(typeof tap.Step).toBe('function');
		expect(typeof tap.usePrintMode).toBe('function');
		expect(typeof tap.useActive).toBe('function');
		expect(typeof tap.useTheme).toBe('function');
		expect(typeof tap.Slot).toBe('function');
		expect(typeof tap.textOn).toBe('function');
	});

	it('is safe to call more than once', () => {
		installHost();
		installHost();
		expect(Object.keys(window.__TAP_HOST__).sort()).toEqual([...EXPECTED_HOST_MODULE_NAMES].sort());
	});
});
