import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { resolve } from 'path';

export default defineConfig({
	plugins: [react()],
	test: {
		// Enable globals for testing-library matchers
		globals: true,
		// Use jsdom for DOM testing
		environment: 'jsdom',
		// Setup files for testing-library extensions
		setupFiles: ['./src/test/setup.ts'],
		include: ['src/**/*.{test,spec}.{js,ts,tsx}'],
		// Coverage configuration
		coverage: {
			provider: 'v8',
			reporter: ['text', 'json', 'html'],
			exclude: [
				'src/**/*.test.ts',
				'src/**/*.test.tsx',
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
