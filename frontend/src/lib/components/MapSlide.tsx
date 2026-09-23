/**
 * Renders an animated MapLibre map for a `map` code block, driven entirely
 * by the presenter step passed from Slide: step 0 shows the start view,
 * step 1 flies to the end view, and print mode jumps straight to the end
 * view with no animation. This component has no store access; all state it
 * needs comes in as props.
 */

import { useEffect, useRef, type MutableRefObject } from 'react';
import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import type { MapConfig } from '$lib/types';
import { getMapLibreEasing, resolveMapStyle } from '../utils/map';
import { holdReady } from '$lib/ready/blockers';

export interface MapSlideProps {
	/** Parsed configuration for the `map` code block. */
	config: MapConfig;
	/** Presenter step: 0 shows the start view, 1 shows or animates to the end view. */
	step: number;
	/** Whether this slide is currently shown to the viewer. */
	active?: boolean;
	/** Whether rendering for PDF export: jumps to the end view with no animation. */
	printMode?: boolean;
}

/** Duration in milliseconds for the quick reset animation back to the start view. */
const RESET_DURATION = 500;

/** How long the ready signal waits for a new map to load its style. */
export const MAP_LOAD_TIMEOUT_MS = 10000;

/** How long the ready signal waits for a map to go idle after it loaded or moved. */
export const MAP_READY_TIMEOUT_MS = 3000;

/**
 * Holds a "map" blocker until the next "idle" event releases it through
 * `releaseRef`, or until `timeoutMs` passes. Releases the hold that
 * `releaseRef` already had, so a map holds at most one blocker.
 */
function holdMapUntilIdle(releaseRef: MutableRefObject<(() => void) | null>, timeoutMs: number): void {
	releaseRef.current?.();
	const release = holdReady('map');
	let timer: ReturnType<typeof setTimeout> | undefined;
	const releaseThisHold = (): void => {
		clearTimeout(timer);
		release();
		if (releaseRef.current === releaseThisHold) {
			releaseRef.current = null;
		}
	};
	timer = setTimeout(releaseThisHold, timeoutMs);
	releaseRef.current = releaseThisHold;
}

function prefersReducedMotion(): boolean {
	if (typeof window === 'undefined') return false;
	return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/** Convert [lat, lng] to MapLibre's [lng, lat] format. */
function toMapLibreCoords(coords: [number, number]): [number, number] {
	return [coords[1], coords[0]];
}

function createMarkerElement(className: string, color: string): HTMLDivElement {
	const element = document.createElement('div');
	element.className = className;
	element.innerHTML = `
		<svg width="24" height="36" viewBox="0 0 24 36" fill="none" xmlns="http://www.w3.org/2000/svg">
			<path d="M12 0C5.373 0 0 5.373 0 12c0 9 12 24 12 24s12-15 12-24c0-6.627-5.373-12-12-12z" fill="${color}"/>
			<circle cx="12" cy="12" r="6" fill="white"/>
		</svg>
	`;
	return element;
}

export function MapSlide({ config, step, active = true, printMode = false }: MapSlideProps) {
	const containerRef = useRef<HTMLDivElement>(null);
	const mapRef = useRef<maplibregl.Map | null>(null);
	const startMarkerRef = useRef<maplibregl.Marker | null>(null);
	const endMarkerRef = useRef<maplibregl.Marker | null>(null);
	const isReadyRef = useRef(false);
	const previousStepRef = useRef(step);
	// The ready signal waits while this map draws: from mount until its
	// first "idle" after "load", and during each move until the next "idle".
	const releaseMapHoldRef = useRef<(() => void) | null>(null);

	// Create the map once per config, positioned directly at the view that
	// matches the step it mounts at, so returning to an already-animated map
	// slide shows the end view without replaying the animation.
	useEffect(() => {
		const container = containerRef.current;
		if (!container) return;

		const showEnd = printMode || step >= 1;
		const initialCoords = showEnd ? config.end : config.start;
		const initialZoom = showEnd ? config.endZoom : config.zoom;

		const map = new maplibregl.Map({
			container,
			style: resolveMapStyle(config.style),
			center: toMapLibreCoords(initialCoords),
			zoom: initialZoom,
			pitch: config.pitch,
			bearing: config.bearing,
			attributionControl: { compact: true },
			interactive: false
		});
		mapRef.current = map;
		isReadyRef.current = false;
		holdMapUntilIdle(releaseMapHoldRef, MAP_LOAD_TIMEOUT_MS);

		map.on('load', () => {
			isReadyRef.current = true;

			if (config.showPath) {
				const geojson = {
					type: 'Feature' as const,
					properties: {},
					geometry: {
						type: 'LineString' as const,
						coordinates: [toMapLibreCoords(config.start), toMapLibreCoords(config.end)]
					}
				};
				map.addSource('path-line', { type: 'geojson', data: geojson });
				map.addLayer({
					id: 'path-line',
					type: 'line',
					source: 'path-line',
					layout: { 'line-join': 'round', 'line-cap': 'round' },
					paint: {
						'line-color': '#3b82f6',
						'line-width': 3,
						'line-dasharray': [2, 2],
						'line-opacity': 0.8
					}
				});
			}

			if (config.markers) {
				startMarkerRef.current = new maplibregl.Marker({
					element: createMarkerElement('map-marker map-marker-start', '#22c55e')
				})
					.setLngLat(toMapLibreCoords(config.start))
					.addTo(map);
				endMarkerRef.current = new maplibregl.Marker({
					element: createMarkerElement('map-marker map-marker-end', '#ef4444')
				})
					.setLngLat(toMapLibreCoords(config.end))
					.addTo(map);
			}

			// Expose the map instance for E2E testing and PDF export.
			if (typeof window !== 'undefined') {
				(window as unknown as { __tapMap: maplibregl.Map }).__tapMap = map;
				(window as unknown as { __tapMapReady: boolean }).__tapMapReady = true;
			}

			holdMapUntilIdle(releaseMapHoldRef, MAP_READY_TIMEOUT_MS);
		});

		map.on('error', (event) => {
			console.error('Map error:', event);
		});

		map.on('idle', () => {
			if (isReadyRef.current) {
				releaseMapHoldRef.current?.();
			}
		});

		return () => {
			releaseMapHoldRef.current?.();
			startMarkerRef.current?.remove();
			startMarkerRef.current = null;
			endMarkerRef.current?.remove();
			endMarkerRef.current = null;
			map.remove();
			mapRef.current = null;
			isReadyRef.current = false;

			if (typeof window !== 'undefined') {
				delete (window as unknown as { __tapMap?: maplibregl.Map }).__tapMap;
				delete (window as unknown as { __tapMapReady?: boolean }).__tapMapReady;
			}
		};
		// A new config identifies a distinct map slide, which gets a fresh map.
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [config]);

	// Animate between the start and end views when the step actually changes.
	// Skipped on mount (previousStepRef starts equal to step), which is what
	// lets a slide mounted at step 1 show the end view without replaying.
	useEffect(() => {
		const map = mapRef.current;
		const wasStep = previousStepRef.current;
		previousStepRef.current = step;

		if (!map || !isReadyRef.current || wasStep === step) {
			return;
		}

		if (step >= 1 && wasStep < 1) {
			if (printMode || prefersReducedMotion()) {
				holdMapUntilIdle(releaseMapHoldRef, MAP_READY_TIMEOUT_MS);
				map.jumpTo({
					center: toMapLibreCoords(config.end),
					zoom: config.endZoom,
					pitch: config.pitch,
					bearing: config.bearing
				});
			} else {
				holdMapUntilIdle(releaseMapHoldRef, config.duration + MAP_READY_TIMEOUT_MS);
				map.flyTo({
					center: toMapLibreCoords(config.end),
					zoom: config.endZoom,
					pitch: config.pitch,
					bearing: config.bearing,
					duration: config.duration,
					easing: getMapLibreEasing(config.easing)
				});
			}
		} else if (step < 1 && wasStep >= 1) {
			if (prefersReducedMotion()) {
				holdMapUntilIdle(releaseMapHoldRef, MAP_READY_TIMEOUT_MS);
				map.jumpTo({
					center: toMapLibreCoords(config.start),
					zoom: config.zoom,
					pitch: config.pitch,
					bearing: config.bearing
				});
			} else {
				holdMapUntilIdle(releaseMapHoldRef, RESET_DURATION + MAP_READY_TIMEOUT_MS);
				map.flyTo({
					center: toMapLibreCoords(config.start),
					zoom: config.zoom,
					pitch: config.pitch,
					bearing: config.bearing,
					duration: RESET_DURATION
				});
			}
		}
	}, [step, printMode, config]);

	// Resize the map when the slide becomes the active one again, in case its
	// container dimensions changed while it was hidden.
	useEffect(() => {
		const map = mapRef.current;
		if (active && map && isReadyRef.current) {
			map.resize();
		}
	}, [active]);

	return <div className="map-slide" ref={containerRef} />;
}
