// Stand-in types for react/jsx-runtime, motion/react, and the global JSX
// namespace: they let the .jsx/.tsx templates type-check with nothing
// installed next to the deck. Delete this whole file once you install
// `react`, `@types/react`, and `motion` next to the deck; those packages'
// real types conflict with the stand-ins here.

declare module 'motion/react' {
	export const motion: any;
}

declare module 'react/jsx-runtime' {
	export const jsx: any;
	export const jsxs: any;
	export const Fragment: any;
}

declare namespace JSX {
	interface IntrinsicElements {
		[elemName: string]: any;
	}
	interface Element {}
	interface ElementClass {}
	interface ElementAttributesProperty {
		props: unknown;
	}
	interface ElementChildrenAttribute {
		children: unknown;
	}
}
