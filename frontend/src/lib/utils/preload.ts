/**
 * Utilities for preloading presentation assets (images, etc.)
 * to prevent flashes during slide transitions.
 */

import type { Presentation } from '$lib/types';

/**
 * Extract all image URLs from HTML content.
 * Finds both <img src="..."> tags and inline style background-image: url(...).
 */
function extractImagesFromHtml(html: string): string[] {
	const images: string[] = [];

	// Match <img> src attributes
	const imgRegex = /<img[^>]+src=["']([^"']+)["'][^>]*>/gi;
	let match;
	while ((match = imgRegex.exec(html)) !== null) {
		if (match[1]) {
			images.push(match[1]);
		}
	}

	// Match inline background-image: url(...)
	const bgRegex = /background-image:\s*url\(['"]?([^'")\s]+)['"]?\)/gi;
	while ((match = bgRegex.exec(html)) !== null) {
		if (match[1]) {
			images.push(match[1]);
		}
	}

	return images;
}

/**
 * Extract all image URLs from a presentation.
 * Includes images from slide HTML and background configurations.
 */
export function extractAllImages(presentation: Presentation): string[] {
	const imageSet = new Set<string>();

	for (const slide of presentation.slides) {
		// Extract images from slide HTML
		const htmlImages = extractImagesFromHtml(slide.html);
		for (const img of htmlImages) {
			imageSet.add(img);
		}

		// Add background images
		if (slide.background?.type === 'image' && slide.background.value) {
			imageSet.add(slide.background.value);
		}
	}

	return Array.from(imageSet);
}

/**
 * Preload a single image and return a promise.
 * Resolves when loaded, rejects on error (with graceful fallback).
 */
function preloadImage(src: string): Promise<void> {
	return new Promise((resolve) => {
		const img = new Image();
		img.onload = () => resolve();
		img.onerror = () => {
			// Log warning but don't fail - the presentation should still work
			console.warn(`[tap] Failed to preload image: ${src}`);
			resolve();
		};
		img.src = src;
	});
}

/**
 * Preload all images in a presentation.
 * Returns a promise that resolves when all images are loaded.
 *
 * @param presentation - The presentation data
 * @param onProgress - Optional callback for progress updates (0-1)
 */
export async function preloadPresentationImages(
	presentation: Presentation,
	onProgress?: (progress: number) => void
): Promise<void> {
	const images = extractAllImages(presentation);

	if (images.length === 0) {
		onProgress?.(1);
		return;
	}

	let loaded = 0;
	const total = images.length;

	await Promise.all(
		images.map(async (src) => {
			await preloadImage(src);
			loaded++;
			onProgress?.(loaded / total);
		})
	);
}
