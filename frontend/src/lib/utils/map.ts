/**
 * Map configuration parsing and validation utilities.
 * Handles parsing YAML-style config from ```map code blocks.
 */
import type { MapConfig, MapEasing } from '$lib/types';

/**
 * Default values for map configuration.
 */
const MAP_DEFAULTS: Omit<MapConfig, 'start' | 'end'> = {
	zoom: 12,
	endZoom: 12, // Will be set to zoom value if not specified
	duration: 3000,
	easing: 'ease-in-out',
	pitch: 0,
	bearing: 0,
	style: 'geocodio',
	markers: true,
	showPath: false
};

/**
 * Valid easing values for map animations.
 */
const VALID_EASINGS: MapEasing[] = ['linear', 'ease-in', 'ease-out', 'ease-in-out'];

/**
 * Parse coordinates from a string like "40.7128, -74.0060".
 * Returns [lat, lng] tuple or null if invalid.
 */
function parseCoordinates(value: string): [number, number] | null {
	const parts = value.split(',').map((p) => p.trim());
	if (parts.length !== 2) return null;

	const latStr = parts[0];
	const lngStr = parts[1];
	if (!latStr || !lngStr) return null;

	const lat = parseFloat(latStr);
	const lng = parseFloat(lngStr);

	if (isNaN(lat) || isNaN(lng)) return null;

	// Validate coordinate ranges
	if (lat < -90 || lat > 90) return null;
	if (lng < -180 || lng > 180) return null;

	return [lat, lng];
}

/**
 * Parse a boolean value from string.
 * Handles "true", "false", "yes", "no", "1", "0".
 */
function parseBoolean(value: string): boolean | null {
	const v = value.toLowerCase().trim();
	if (v === 'true' || v === 'yes' || v === '1') return true;
	if (v === 'false' || v === 'no' || v === '0') return false;
	return null;
}

/**
 * Parse map configuration from a code block content string.
 * Returns null if required fields are missing or invalid.
 *
 * @param content The raw text content from a ```map code block
 * @returns Parsed MapConfig or null if invalid
 */
export function parseMapConfig(content: string): MapConfig | null {
	const lines = content.split('\n');
	const values: Record<string, string> = {};

	// Parse key-value pairs
	for (const line of lines) {
		const trimmed = line.trim();
		if (!trimmed || trimmed.startsWith('#')) continue;

		const colonIndex = trimmed.indexOf(':');
		if (colonIndex === -1) continue;

		const key = trimmed.slice(0, colonIndex).trim().toLowerCase();
		const value = trimmed.slice(colonIndex + 1).trim();

		if (key && value) {
			values[key] = value;
		}
	}

	// Parse required fields
	const start = values['start'] ? parseCoordinates(values['start']) : null;
	const end = values['end'] ? parseCoordinates(values['end']) : null;

	if (!start || !end) {
		return null;
	}

	// Parse optional fields with defaults
	const zoom = values['zoom'] ? parseFloat(values['zoom']) : MAP_DEFAULTS.zoom;
	if (isNaN(zoom) || zoom < 1 || zoom > 20) {
		return null;
	}

	// endZoom defaults to zoom if not specified
	let endZoom = values['endzoom'] ? parseFloat(values['endzoom']) : zoom;
	if (isNaN(endZoom) || endZoom < 1 || endZoom > 20) {
		endZoom = zoom;
	}

	const duration = values['duration'] ? parseInt(values['duration'], 10) : MAP_DEFAULTS.duration;
	if (isNaN(duration) || duration < 0) {
		return null;
	}

	const easingValue = values['easing']?.toLowerCase() as MapEasing;
	const easing = VALID_EASINGS.includes(easingValue) ? easingValue : MAP_DEFAULTS.easing;

	const pitch = values['pitch'] ? parseFloat(values['pitch']) : MAP_DEFAULTS.pitch;
	if (isNaN(pitch) || pitch < 0 || pitch > 85) {
		return null;
	}

	let bearing = values['bearing'] ? parseFloat(values['bearing']) : MAP_DEFAULTS.bearing;
	if (isNaN(bearing)) {
		bearing = MAP_DEFAULTS.bearing;
	}
	// Normalize bearing to 0-360 range
	bearing = ((bearing % 360) + 360) % 360;

	const style = values['style'] || MAP_DEFAULTS.style;

	const markersValue = values['markers'] ? parseBoolean(values['markers']) : null;
	const markers = markersValue !== null ? markersValue : MAP_DEFAULTS.markers;

	const showPathValue = values['showpath'] ? parseBoolean(values['showpath']) : null;
	const showPath = showPathValue !== null ? showPathValue : MAP_DEFAULTS.showPath;

	return {
		start,
		end,
		zoom,
		endZoom,
		duration,
		easing,
		pitch,
		bearing,
		style,
		markers,
		showPath
	};
}

/**
 * Get the Geocodio (OSM) map style URL.
 * This is a free, open-source map style based on OpenStreetMap.
 */
export function getDefaultMapStyle(): string {
	// Using Carto's Positron style - clean, light style suitable for presentations
	// Free for all users with attribution
	return 'https://basemaps.cartocdn.com/gl/positron-gl-style/style.json';
}

/**
 * Resolve a style value to a full URL.
 * Handles 'geocodio' as special value for default style.
 */
export function resolveMapStyle(style: string): string {
	if (style === 'geocodio' || style === 'default' || !style) {
		return getDefaultMapStyle();
	}
	return style;
}

/**
 * Convert map easing to MapLibre easing function.
 * MapLibre expects a function that takes t (0-1) and returns a value (0-1).
 */
export function getMapLibreEasing(easing: MapEasing): (t: number) => number {
	switch (easing) {
		case 'linear':
			return (t) => t;
		case 'ease-in':
			return (t) => t * t;
		case 'ease-out':
			return (t) => 1 - (1 - t) * (1 - t);
		case 'ease-in-out':
		default:
			return (t) => (t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2);
	}
}
