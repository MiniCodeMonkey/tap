/**
 * Installs window.__TAP_HOST__: the shared React, React DOM, Motion, and
 * tap helper module instances that a deck component's bundle resolves its
 * host imports to (see internal/components/host_shims.go's
 * hostShimPlugin). A component never bundles its own copy of these
 * libraries, so it always shares the viewer's React tree and Motion
 * animation state.
 *
 * Call installHost() once, before the first createRoot(...).render(...),
 * in every entry point that can show a deck component (main.tsx and
 * presenter.tsx).
 */

import * as React from 'react';
import * as ReactJsxRuntime from 'react/jsx-runtime';
import * as ReactDOM from 'react-dom';
import * as ReactDOMClient from 'react-dom/client';
import * as MotionReact from 'motion/react';
import * as Tap from './tap';

/**
 * The exact module names a deck component's bundle may import from the
 * host. Mirrors internal/components/host_shims.go's hostModuleNames; a Go
 * test and this list both pin the same set so the two sides cannot drift.
 */
export const HOST_MODULE_NAMES = [
	'react',
	'react/jsx-runtime',
	'react-dom',
	'react-dom/client',
	'motion',
	'motion/react',
	'tap'
] as const;

/**
 * Returns a module namespace with a `default` export, adding one that
 * points back at the namespace itself when the module has none. This is
 * what makes `import Something from 'react'`-style default imports work
 * for a host module whose real package has no default export of its own
 * (esbuild's CommonJS interop reads `.default` first): a namespace object
 * that already has `default` is returned unchanged.
 */
function withDefault<T extends object>(moduleNamespace: T): T & { default: T } {
	if ('default' in moduleNamespace) {
		return moduleNamespace as T & { default: T };
	}
	return { ...moduleNamespace, default: moduleNamespace };
}

/**
 * Assigns window.__TAP_HOST__ with one entry per HOST_MODULE_NAMES name.
 * Safe to call more than once (a StrictMode double-invoke, or both entry
 * points loading in the same page during a test) since it always
 * reassigns the same values.
 */
export function installHost(): void {
	window.__TAP_HOST__ = {
		react: withDefault(React),
		'react/jsx-runtime': withDefault(ReactJsxRuntime),
		'react-dom': withDefault(ReactDOM),
		'react-dom/client': withDefault(ReactDOMClient),
		// motion/react re-exports everything the top-level motion package
		// exports (animate, scroll, inView, and the rest) on top of its own
		// React APIs, so one object serves both host module names; a separate
		// import of the top-level package would just bundle the same code
		// twice.
		motion: withDefault(MotionReact),
		'motion/react': withDefault(MotionReact),
		tap: withDefault(Tap)
	};
}

declare global {
	interface Window {
		__TAP_HOST__: Record<string, object>;
	}
}
