import { describe, it, expect } from 'vitest';
import { parseMapConfig, resolveMapStyle, getMapLibreEasing, getDefaultMapStyle } from './map';

describe('parseMapConfig', () => {
	it('parses valid map config with required fields', () => {
		const content = `start: 40.7128, -74.0060
end: 34.0522, -118.2437`;

		const config = parseMapConfig(content);

		expect(config).not.toBeNull();
		expect(config?.start).toEqual([40.7128, -74.006]);
		expect(config?.end).toEqual([34.0522, -118.2437]);
	});

	it('applies default values for optional fields', () => {
		const content = `start: 40.7128, -74.0060
end: 34.0522, -118.2437`;

		const config = parseMapConfig(content);

		expect(config?.zoom).toBe(12);
		expect(config?.duration).toBe(3000);
		expect(config?.easing).toBe('ease-in-out');
		expect(config?.pitch).toBe(0);
		expect(config?.bearing).toBe(0);
		expect(config?.markers).toBe(true);
		expect(config?.showPath).toBe(false);
		expect(config?.style).toBe('geocodio');
	});

	it('parses all optional fields', () => {
		const content = `start: 48.8566, 2.3522
end: 51.5074, -0.1278
zoom: 6
endZoom: 10
duration: 5000
easing: ease-in
pitch: 45
bearing: -30
markers: false
showPath: true
style: https://example.com/style.json`;

		const config = parseMapConfig(content);

		expect(config?.zoom).toBe(6);
		expect(config?.endZoom).toBe(10);
		expect(config?.duration).toBe(5000);
		expect(config?.easing).toBe('ease-in');
		expect(config?.pitch).toBe(45);
		expect(config?.bearing).toBe(330); // Normalized from -30
		expect(config?.markers).toBe(false);
		expect(config?.showPath).toBe(true);
		expect(config?.style).toBe('https://example.com/style.json');
	});

	it('returns null for missing start', () => {
		const content = `end: 34.0522, -118.2437`;
		expect(parseMapConfig(content)).toBeNull();
	});

	it('returns null for missing end', () => {
		const content = `start: 40.7128, -74.0060`;
		expect(parseMapConfig(content)).toBeNull();
	});

	it('returns null for invalid latitude (>90)', () => {
		const content = `start: 91.0, -74.0060
end: 34.0522, -118.2437`;
		expect(parseMapConfig(content)).toBeNull();
	});

	it('returns null for invalid latitude (<-90)', () => {
		const content = `start: -91.0, -74.0060
end: 34.0522, -118.2437`;
		expect(parseMapConfig(content)).toBeNull();
	});

	it('returns null for invalid longitude (>180)', () => {
		const content = `start: 40.7128, 181.0
end: 34.0522, -118.2437`;
		expect(parseMapConfig(content)).toBeNull();
	});

	it('returns null for invalid longitude (<-180)', () => {
		const content = `start: 40.7128, -181.0
end: 34.0522, -118.2437`;
		expect(parseMapConfig(content)).toBeNull();
	});

	it('sets endZoom to zoom when not specified', () => {
		const content = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
zoom: 8`;

		const config = parseMapConfig(content);
		expect(config?.endZoom).toBe(8);
	});

	it('handles extra whitespace', () => {
		const content = `  start:   40.7128,   -74.0060
  end:  34.0522  ,  -118.2437  `;

		const config = parseMapConfig(content);
		expect(config?.start).toEqual([40.7128, -74.006]);
		expect(config?.end).toEqual([34.0522, -118.2437]);
	});

	it('ignores comment lines', () => {
		const content = `# This is a comment
start: 40.7128, -74.0060
# Another comment
end: 34.0522, -118.2437`;

		const config = parseMapConfig(content);
		expect(config).not.toBeNull();
		expect(config?.start).toEqual([40.7128, -74.006]);
	});

	it('ignores empty lines', () => {
		const content = `
start: 40.7128, -74.0060

end: 34.0522, -118.2437

`;

		const config = parseMapConfig(content);
		expect(config).not.toBeNull();
	});

	it('handles case-insensitive keys', () => {
		const content = `START: 40.7128, -74.0060
END: 34.0522, -118.2437
ZOOM: 10
ENDZOOM: 15`;

		const config = parseMapConfig(content);
		expect(config?.start).toEqual([40.7128, -74.006]);
		expect(config?.zoom).toBe(10);
		expect(config?.endZoom).toBe(15);
	});

	it('parses boolean values correctly', () => {
		const testCases = [
			{ markers: 'true', showPath: 'false', expectedMarkers: true, expectedShowPath: false },
			{ markers: 'yes', showPath: 'no', expectedMarkers: true, expectedShowPath: false },
			{ markers: '1', showPath: '0', expectedMarkers: true, expectedShowPath: false },
			{ markers: 'TRUE', showPath: 'FALSE', expectedMarkers: true, expectedShowPath: false }
		];

		for (const tc of testCases) {
			const content = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
markers: ${tc.markers}
showPath: ${tc.showPath}`;

			const config = parseMapConfig(content);
			expect(config?.markers).toBe(tc.expectedMarkers);
			expect(config?.showPath).toBe(tc.expectedShowPath);
		}
	});

	it('validates zoom range (1-20)', () => {
		const invalidZoom0 = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
zoom: 0`;
		expect(parseMapConfig(invalidZoom0)).toBeNull();

		const invalidZoom21 = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
zoom: 21`;
		expect(parseMapConfig(invalidZoom21)).toBeNull();

		const validZoom1 = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
zoom: 1`;
		expect(parseMapConfig(validZoom1)?.zoom).toBe(1);

		const validZoom20 = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
zoom: 20`;
		expect(parseMapConfig(validZoom20)?.zoom).toBe(20);
	});

	it('validates pitch range (0-85)', () => {
		const invalidPitch = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
pitch: 90`;
		expect(parseMapConfig(invalidPitch)).toBeNull();

		const validPitch0 = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
pitch: 0`;
		expect(parseMapConfig(validPitch0)?.pitch).toBe(0);

		const validPitch85 = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
pitch: 85`;
		expect(parseMapConfig(validPitch85)?.pitch).toBe(85);
	});

	it('normalizes bearing to 0-360 range', () => {
		const negativeBearing = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
bearing: -90`;
		expect(parseMapConfig(negativeBearing)?.bearing).toBe(270);

		const largeBearing = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
bearing: 450`;
		expect(parseMapConfig(largeBearing)?.bearing).toBe(90);
	});

	it('uses default for invalid easing', () => {
		const content = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
easing: invalid-easing`;

		const config = parseMapConfig(content);
		expect(config?.easing).toBe('ease-in-out');
	});

	it('parses valid easing values', () => {
		const easings = ['linear', 'ease-in', 'ease-out', 'ease-in-out'];

		for (const easing of easings) {
			const content = `start: 40.7128, -74.0060
end: 34.0522, -118.2437
easing: ${easing}`;

			const config = parseMapConfig(content);
			expect(config?.easing).toBe(easing);
		}
	});

	it('handles same start and end coordinates (zoom animation)', () => {
		const content = `start: 51.5074, -0.1278
end: 51.5074, -0.1278
zoom: 10
endZoom: 16`;

		const config = parseMapConfig(content);
		expect(config?.start).toEqual([51.5074, -0.1278]);
		expect(config?.end).toEqual([51.5074, -0.1278]);
		expect(config?.zoom).toBe(10);
		expect(config?.endZoom).toBe(16);
	});
});

describe('resolveMapStyle', () => {
	it('returns default style for "geocodio"', () => {
		const result = resolveMapStyle('geocodio');
		expect(result).toBe(getDefaultMapStyle());
	});

	it('returns default style for "default"', () => {
		const result = resolveMapStyle('default');
		expect(result).toBe(getDefaultMapStyle());
	});

	it('returns default style for empty string', () => {
		const result = resolveMapStyle('');
		expect(result).toBe(getDefaultMapStyle());
	});

	it('returns custom URL as-is', () => {
		const customUrl = 'https://example.com/custom-style.json';
		const result = resolveMapStyle(customUrl);
		expect(result).toBe(customUrl);
	});
});

describe('getDefaultMapStyle', () => {
	it('returns a valid URL', () => {
		const style = getDefaultMapStyle();
		expect(style).toMatch(/^https?:\/\//);
		expect(style).toContain('style.json');
	});
});

describe('getMapLibreEasing', () => {
	it('returns linear function for linear easing', () => {
		const fn = getMapLibreEasing('linear');
		expect(fn(0)).toBe(0);
		expect(fn(0.5)).toBe(0.5);
		expect(fn(1)).toBe(1);
	});

	it('returns ease-in function (accelerating)', () => {
		const fn = getMapLibreEasing('ease-in');
		expect(fn(0)).toBe(0);
		expect(fn(0.5)).toBe(0.25); // t^2 at 0.5 = 0.25
		expect(fn(1)).toBe(1);
	});

	it('returns ease-out function (decelerating)', () => {
		const fn = getMapLibreEasing('ease-out');
		expect(fn(0)).toBe(0);
		expect(fn(0.5)).toBe(0.75); // 1 - (1-0.5)^2 = 0.75
		expect(fn(1)).toBe(1);
	});

	it('returns ease-in-out function', () => {
		const fn = getMapLibreEasing('ease-in-out');
		expect(fn(0)).toBe(0);
		expect(fn(1)).toBe(1);
		// Middle value should be around 0.5
		expect(fn(0.5)).toBeCloseTo(0.5, 5);
	});
});
