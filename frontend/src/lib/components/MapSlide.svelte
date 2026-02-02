<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import maplibregl from 'maplibre-gl';
	import type { MapConfig } from '$lib/types';
	import { resolveMapStyle, getMapLibreEasing } from '$lib/utils/map';

	// ============================================================================
	// Props
	// ============================================================================

	interface Props {
		/** Map configuration from parsed code block */
		config: MapConfig;
		/** Whether this slide is currently active */
		active?: boolean;
		/** Whether animation has been triggered */
		animationTriggered?: boolean;
		/** Whether in print/PDF mode (jump to end immediately) */
		isPrintMode?: boolean;
	}

	let {
		config,
		active = true,
		animationTriggered = false,
		isPrintMode = false
	}: Props = $props();

	// ============================================================================
	// State
	// ============================================================================

	let mapContainer: HTMLDivElement | undefined = $state();
	let map: maplibregl.Map | undefined = $state();
	let startMarker: maplibregl.Marker | undefined;
	let endMarker: maplibregl.Marker | undefined;
	let isMapReady = $state(false);
	let hasAnimated = $state(false);

	// ============================================================================
	// Computed
	// ============================================================================

	/**
	 * Check if reduced motion is preferred.
	 */
	function prefersReducedMotion(): boolean {
		if (typeof window === 'undefined') return false;
		return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
	}

	/**
	 * Convert [lat, lng] to MapLibre's [lng, lat] format.
	 */
	function toMapLibreCoords(coords: [number, number]): [number, number] {
		return [coords[1], coords[0]];
	}

	// ============================================================================
	// Map Initialization
	// ============================================================================

	/**
	 * Initialize the map when the component mounts.
	 */
	function initializeMap(): void {
		if (!mapContainer || map) return;

		const styleUrl = resolveMapStyle(config.style);

		// Determine initial position based on mode
		const shouldShowEnd = isPrintMode || prefersReducedMotion();
		const initialCoords = shouldShowEnd ? config.end : config.start;
		const initialZoom = shouldShowEnd ? config.endZoom : config.zoom;

		map = new maplibregl.Map({
			container: mapContainer,
			style: styleUrl,
			center: toMapLibreCoords(initialCoords),
			zoom: initialZoom,
			pitch: config.pitch,
			bearing: config.bearing,
			attributionControl: { compact: true },
			interactive: false // Disable user interaction for presentation
		});

		// Set up load handler
		map.on('load', () => {
			if (!map) return;

			isMapReady = true;

			// Add path line if enabled
			if (config.showPath) {
				addPathLine();
			}

			// Add markers if enabled
			if (config.markers) {
				addMarkers();
			}

			// In print mode, ensure we're at end position and mark ready
			if (isPrintMode || prefersReducedMotion()) {
				jumpToEnd();
				hasAnimated = true;
			}

			// Expose map on window for E2E testing and PDF export
			if (typeof window !== 'undefined') {
				(window as unknown as { __tapMap: maplibregl.Map }).__tapMap = map;
				(window as unknown as { __tapMapReady: boolean }).__tapMapReady = true;
			}
		});

		map.on('error', (e) => {
			console.error('Map error:', e);
		});
	}

	/**
	 * Add start and end markers to the map.
	 */
	function addMarkers(): void {
		if (!map) return;

		// Create start marker (green)
		const startEl = document.createElement('div');
		startEl.className = 'map-marker map-marker-start';
		startEl.innerHTML = `
			<svg width="24" height="36" viewBox="0 0 24 36" fill="none" xmlns="http://www.w3.org/2000/svg">
				<path d="M12 0C5.373 0 0 5.373 0 12c0 9 12 24 12 24s12-15 12-24c0-6.627-5.373-12-12-12z" fill="#22c55e"/>
				<circle cx="12" cy="12" r="6" fill="white"/>
			</svg>
		`;

		startMarker = new maplibregl.Marker({ element: startEl })
			.setLngLat(toMapLibreCoords(config.start))
			.addTo(map);

		// Create end marker (red)
		const endEl = document.createElement('div');
		endEl.className = 'map-marker map-marker-end';
		endEl.innerHTML = `
			<svg width="24" height="36" viewBox="0 0 24 36" fill="none" xmlns="http://www.w3.org/2000/svg">
				<path d="M12 0C5.373 0 0 5.373 0 12c0 9 12 24 12 24s12-15 12-24c0-6.627-5.373-12-12-12z" fill="#ef4444"/>
				<circle cx="12" cy="12" r="6" fill="white"/>
			</svg>
		`;

		endMarker = new maplibregl.Marker({ element: endEl })
			.setLngLat(toMapLibreCoords(config.end))
			.addTo(map);
	}

	/**
	 * Add a path line connecting start and end points.
	 */
	function addPathLine(): void {
		if (!map) return;

		const geojson: GeoJSON.Feature<GeoJSON.LineString> = {
			type: 'Feature',
			properties: {},
			geometry: {
				type: 'LineString',
				coordinates: [toMapLibreCoords(config.start), toMapLibreCoords(config.end)]
			}
		};

		map.addSource('path-line', {
			type: 'geojson',
			data: geojson
		});

		map.addLayer({
			id: 'path-line',
			type: 'line',
			source: 'path-line',
			layout: {
				'line-join': 'round',
				'line-cap': 'round'
			},
			paint: {
				'line-color': '#3b82f6',
				'line-width': 3,
				'line-dasharray': [2, 2],
				'line-opacity': 0.8
			}
		});
	}

	// ============================================================================
	// Animation
	// ============================================================================

	/**
	 * Trigger the fly animation to the end position.
	 */
	export function triggerAnimation(): void {
		if (!map || !isMapReady || hasAnimated) return;

		// Check for reduced motion
		if (prefersReducedMotion()) {
			jumpToEnd();
			hasAnimated = true;
			return;
		}

		const easingFn = getMapLibreEasing(config.easing);

		map.flyTo({
			center: toMapLibreCoords(config.end),
			zoom: config.endZoom,
			pitch: config.pitch,
			bearing: config.bearing,
			duration: config.duration,
			easing: easingFn
		});

		hasAnimated = true;
	}

	/**
	 * Jump directly to the end position (no animation).
	 */
	function jumpToEnd(): void {
		if (!map) return;

		map.jumpTo({
			center: toMapLibreCoords(config.end),
			zoom: config.endZoom,
			pitch: config.pitch,
			bearing: config.bearing
		});
	}

	/**
	 * Reset the map to the start position.
	 */
	export function resetToStart(): void {
		if (!map || !isMapReady) return;

		// Use a quick animation or jump based on preference
		if (prefersReducedMotion()) {
			map.jumpTo({
				center: toMapLibreCoords(config.start),
				zoom: config.zoom,
				pitch: config.pitch,
				bearing: config.bearing
			});
		} else {
			map.flyTo({
				center: toMapLibreCoords(config.start),
				zoom: config.zoom,
				pitch: config.pitch,
				bearing: config.bearing,
				duration: 500 // Quick reset animation
			});
		}

		hasAnimated = false;
	}

	/**
	 * Check if the map has been animated.
	 */
	export function hasBeenAnimated(): boolean {
		return hasAnimated;
	}

	// ============================================================================
	// Lifecycle
	// ============================================================================

	onMount(() => {
		// Small delay to ensure container is properly sized
		requestAnimationFrame(() => {
			initializeMap();
		});
	});

	onDestroy(() => {
		// Clean up map instance to avoid WebGL context limits
		if (startMarker) {
			startMarker.remove();
			startMarker = undefined;
		}
		if (endMarker) {
			endMarker.remove();
			endMarker = undefined;
		}
		if (map) {
			map.remove();
			map = undefined;
		}

		// Clean up window references
		if (typeof window !== 'undefined') {
			delete (window as unknown as { __tapMap?: maplibregl.Map }).__tapMap;
			delete (window as unknown as { __tapMapReady?: boolean }).__tapMapReady;
		}
	});

	// Watch for animation trigger
	$effect(() => {
		if (animationTriggered && isMapReady && !hasAnimated) {
			triggerAnimation();
		}
	});

	// Watch for active state changes (reset when slide becomes active again)
	$effect(() => {
		if (active && map && isMapReady) {
			// Resize map in case container dimensions changed
			map.resize();
		}
	});
</script>

<div class="map-slide" bind:this={mapContainer}></div>

<style>
	.map-slide {
		width: 100%;
		height: 100%;
		position: absolute;
		top: 0;
		left: 0;
	}

	/* Import MapLibre CSS */
	:global(.maplibregl-map) {
		font-family: inherit;
	}

	/* Marker styles */
	:global(.map-marker) {
		cursor: default;
	}

	:global(.map-marker svg) {
		display: block;
	}

	/* Attribution styling to match presentation themes */
	:global(.maplibregl-ctrl-attrib) {
		font-size: 10px;
		background: rgba(255, 255, 255, 0.7);
		padding: 2px 5px;
		border-radius: 3px;
	}

	/* Hide attribution in fullscreen/print mode */
	@media print {
		:global(.maplibregl-ctrl-attrib) {
			display: none;
		}
	}
</style>
