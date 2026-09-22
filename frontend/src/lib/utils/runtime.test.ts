import { afterEach, describe, expect, it } from 'vitest';
import { isDevRuntime } from './runtime';

describe('isDevRuntime', () => {
	const originalDev = import.meta.env.DEV;

	afterEach(() => {
		(import.meta.env as { DEV: boolean }).DEV = originalDev;
		document.getElementById('presentation-data')?.remove();
	});

	it('is true when import.meta.env.DEV is true, regardless of #presentation-data (the frontend\'s own Vite dev server)', () => {
		(import.meta.env as { DEV: boolean }).DEV = true;
		expect(isDevRuntime()).toBe(true);
	});

	it('is true when the page has no #presentation-data element (a live server: tap dev, tap export pdf, tap export images)', () => {
		(import.meta.env as { DEV: boolean }).DEV = false;
		expect(document.getElementById('presentation-data')).toBeNull();
		expect(isDevRuntime()).toBe(true);
	});

	it('is false when the page carries #presentation-data (a static tap build output)', () => {
		(import.meta.env as { DEV: boolean }).DEV = false;
		const script = document.createElement('script');
		script.id = 'presentation-data';
		script.type = 'application/json';
		script.textContent = '{}';
		document.body.appendChild(script);

		expect(isDevRuntime()).toBe(false);
	});
});
