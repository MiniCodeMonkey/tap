import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { resolve } from 'path';

export default defineConfig({
	plugins: [
		svelte({
			hot: !process.env.VITEST,
			// Force client-side compilation for tests
			compilerOptions: {
				dev: true
			}
		})
	],
	test: {
		// Enable globals for testing-library matchers
		globals: true,
		// Use jsdom for DOM testing
		environment: 'jsdom',
		// Setup files for testing-library extensions
		setupFiles: ['./src/test/setup.ts'],
		// Include test files
		include: ['src/**/*.{test,spec}.{js,ts}'],
		// Coverage configuration
		coverage: {
			provider: 'v8',
			reporter: ['text', 'json', 'html'],
			// Cover only the three components required by US-075
			include: [
				'src/lib/components/SlideRenderer.svelte',
				'src/lib/components/SlideContainer.svelte',
				'src/lib/components/FragmentContainer.svelte'
			],
			exclude: [
				'src/**/*.test.ts',
				'src/**/*.spec.ts',
				'src/test/**/*'
			]
		},
		// Disable CSS processing during tests
		css: false,
		// Ensure aliases are resolved correctly
		alias: {
			$lib: resolve(__dirname, 'src/lib')
		}
	},
	resolve: {
		alias: {
			$lib: resolve(__dirname, 'src/lib')
		},
		// Ensure we resolve to browser versions
		conditions: ['browser', 'development']
	}
});
