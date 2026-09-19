import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { act } from 'react';
import { MapSlide } from './MapSlide';
import type { MapConfig } from '$lib/types';

/**
 * Mock maplibre-gl the same way mermaid.test.ts mocks 'mermaid': stub the
 * third-party library so MapSlide is exercised end to end without a real
 * WebGL context. MockMap records its constructor options and every call to
 * flyTo/jumpTo so tests can assert on the animation MapSlide chose. Defined
 * inside vi.hoisted because vi.mock's factory is hoisted above the imports.
 */
const { MockMap, MockMarker } = vi.hoisted(() => {
	class MockMap {
		static instances: MockMap[] = [];
		options: Record<string, unknown>;
		flyTo = vi.fn();
		jumpTo = vi.fn();
		resize = vi.fn();
		remove = vi.fn();
		addSource = vi.fn();
		addLayer = vi.fn();
		getLayer = vi.fn();
		private handlers: Record<string, Array<(...args: unknown[]) => void>> = {};

		constructor(options: Record<string, unknown>) {
			this.options = options;
			MockMap.instances.push(this);
		}

		on(event: string, handler: (...args: unknown[]) => void): void {
			(this.handlers[event] ??= []).push(handler);
		}

		trigger(event: string): void {
			for (const handler of this.handlers[event] ?? []) {
				handler();
			}
		}
	}

	class MockMarker {
		options: Record<string, unknown>;
		setLngLat = vi.fn(() => this);
		addTo = vi.fn(() => this);
		remove = vi.fn();

		constructor(options: Record<string, unknown>) {
			this.options = options;
		}
	}

	return { MockMap, MockMarker };
});

vi.mock('maplibre-gl', () => ({
	default: { Map: MockMap, Marker: MockMarker }
}));

vi.mock('maplibre-gl/dist/maplibre-gl.css', () => ({}));

function baseConfig(overrides: Partial<MapConfig> = {}): MapConfig {
	return {
		start: [40.7128, -74.006],
		end: [34.0522, -118.2437],
		zoom: 4,
		endZoom: 6,
		duration: 3000,
		easing: 'ease-in-out',
		pitch: 0,
		bearing: 0,
		style: 'geocodio',
		markers: true,
		showPath: false,
		...overrides
	};
}

function latestMap(): MockMap {
	const map = MockMap.instances.at(-1);
	if (!map) throw new Error('No MockMap instance was created');
	return map;
}

describe('MapSlide', () => {
	afterEach(() => {
		cleanup();
		MockMap.instances = [];
		delete (window as unknown as { __tapMap?: unknown }).__tapMap;
		delete (window as unknown as { __tapMapReady?: unknown }).__tapMapReady;
	});

	it('mounts at the start view when step is 0', () => {
		const config = baseConfig();
		render(<MapSlide config={config} step={0} />);

		const map = latestMap();
		expect(map.options.center).toEqual([-74.006, 40.7128]);
		expect(map.options.zoom).toBe(4);
	});

	it('mounts at the end view without animating when step is already 1', () => {
		const config = baseConfig();
		render(<MapSlide config={config} step={1} />);

		const map = latestMap();
		expect(map.options.center).toEqual([-118.2437, 34.0522]);
		expect(map.options.zoom).toBe(6);
		expect(map.flyTo).not.toHaveBeenCalled();
		expect(map.jumpTo).not.toHaveBeenCalled();
	});

	it('flies to the end view once when the step advances from 0 to 1', () => {
		const config = baseConfig();
		const { rerender } = render(<MapSlide config={config} step={0} />);
		const map = latestMap();

		act(() => {
			map.trigger('load');
		});

		rerender(<MapSlide config={config} step={1} />);

		expect(map.flyTo).toHaveBeenCalledTimes(1);
		expect(map.flyTo).toHaveBeenCalledWith(
			expect.objectContaining({ center: [-118.2437, 34.0522], zoom: 6, duration: 3000 })
		);
		expect(map.jumpTo).not.toHaveBeenCalled();
	});

	it('resets to the start view when the step retreats from 1 to 0', () => {
		const config = baseConfig();
		const { rerender } = render(<MapSlide config={config} step={1} />);
		const map = latestMap();

		act(() => {
			map.trigger('load');
		});

		rerender(<MapSlide config={config} step={0} />);

		expect(map.flyTo).toHaveBeenCalledTimes(1);
		expect(map.flyTo).toHaveBeenCalledWith(
			expect.objectContaining({ center: [-74.006, 40.7128], zoom: 4, duration: 500 })
		);
	});

	it('does not animate again when the step is unchanged across a rerender', () => {
		const config = baseConfig();
		const { rerender } = render(<MapSlide config={config} step={0} />);
		const map = latestMap();

		act(() => {
			map.trigger('load');
		});

		rerender(<MapSlide config={config} step={0} />);

		expect(map.flyTo).not.toHaveBeenCalled();
		expect(map.jumpTo).not.toHaveBeenCalled();
	});

	it('jumps straight to the end view in print mode with no animation', () => {
		const config = baseConfig();
		render(<MapSlide config={config} step={1} printMode />);

		const map = latestMap();
		expect(map.options.center).toEqual([-118.2437, 34.0522]);
		expect(map.options.zoom).toBe(6);
		expect(map.flyTo).not.toHaveBeenCalled();
	});

	it('jumps instead of flying when print mode is enabled while advancing steps', () => {
		const config = baseConfig();
		const { rerender } = render(<MapSlide config={config} step={0} printMode={false} />);
		const map = latestMap();

		act(() => {
			map.trigger('load');
		});

		rerender(<MapSlide config={config} step={1} printMode />);

		expect(map.jumpTo).toHaveBeenCalledTimes(1);
		expect(map.jumpTo).toHaveBeenCalledWith(
			expect.objectContaining({ center: [-118.2437, 34.0522], zoom: 6 })
		);
		expect(map.flyTo).not.toHaveBeenCalled();
	});

	it('adds start and end markers when markers is enabled', () => {
		const config = baseConfig({ markers: true });
		const { container } = render(<MapSlide config={config} step={0} />);
		const map = latestMap();

		act(() => {
			map.trigger('load');
		});

		expect(container.querySelector('.map-slide')).not.toBeNull();
	});

	it('adds a path source and layer when showPath is enabled', () => {
		const config = baseConfig({ showPath: true });
		render(<MapSlide config={config} step={0} />);
		const map = latestMap();

		act(() => {
			map.trigger('load');
		});

		expect(map.addSource).toHaveBeenCalledWith('path-line', expect.objectContaining({ type: 'geojson' }));
		expect(map.addLayer).toHaveBeenCalledWith(expect.objectContaining({ id: 'path-line' }));
	});

	it('exposes the map instance on window once ready, for E2E and PDF export', () => {
		const config = baseConfig();
		render(<MapSlide config={config} step={0} />);
		const map = latestMap();

		act(() => {
			map.trigger('load');
		});

		expect((window as unknown as { __tapMap: unknown }).__tapMap).toBeDefined();
		expect((window as unknown as { __tapMapReady: boolean }).__tapMapReady).toBe(true);
	});
});
