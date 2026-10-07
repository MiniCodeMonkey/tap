/**
 * LayoutSplitMedia - image and text side by side.
 *
 * With the slide's `zoom` directive, step 1 glides the media slot to the
 * center of the slide at the largest size that fits inside the slide
 * padding, and fades the text. The PDF and every thumbnail show step 0, so
 * the text stays readable there.
 */

import { useContext, useLayoutEffect, useRef } from 'react';
import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';
import { SlideContext } from '../components/SlideContext';

export function LayoutSplitMedia({ slots, slide, step, printMode, preview }: LayoutProps) {
	const { printMode: pdf } = useContext(SlideContext);
	const zoomed = slide.zoom === true && step >= 1 && !pdf && !preview;
	const rootRef = useRef<HTMLDivElement>(null);

	useLayoutEffect(() => {
		const root = rootRef.current;
		const media = root?.querySelector<HTMLElement>(':scope > .slot-media');
		if (!root || !media) return;
		if (!zoomed) {
			media.style.transform = '';
			return;
		}
		const apply = () => {
			media.style.transform = zoomTransform(root, media);
		};
		apply();
		// A screenshot that loads late or a stage that resizes moves the target.
		const observer = new ResizeObserver(apply);
		observer.observe(root);
		observer.observe(media.firstElementChild ?? media);
		return () => observer.disconnect();
	}, [zoomed, slots.media]);

	return (
		<div
			ref={rootRef}
			className="layout-split-media"
			data-zoom={slide.zoom ? (zoomed ? 'in' : 'out') : undefined}
			data-settled={printMode ? '' : undefined}
		>
			<Slot html={slots.default} className="slot-default" />
			<Slot html={slots.media} className="slot-media" />
		</div>
	);
}

/**
 * The transform that moves the media's visible box to the center of the
 * layout and scales it to the largest size that fits inside the slide
 * padding. The visible box is the media's first child (the theme's image
 * frame), shrunk to the picture when an `object-fit: contain` image leaves
 * empty bands inside it. Measured with layout offsets, which ignore the
 * stage scale and any entrance animation still running on the slide.
 */
function zoomTransform(root: HTMLElement, media: HTMLElement): string {
	const frame = (media.firstElementChild as HTMLElement | null) ?? media;
	const box = visibleBox(frame, root);
	if (box.width === 0 || box.height === 0) return '';

	// --slide-padding resolves to px through scroll-padding (see layouts.css).
	const margin = parseFloat(getComputedStyle(root).scrollPaddingTop) || 0;
	const fit = Math.min(
		(root.offsetWidth - 2 * margin) / box.width,
		(root.offsetHeight - 2 * margin) / box.height
	);
	const centerX = box.left + box.width / 2;
	const centerY = box.top + box.height / 2;
	const mediaOffset = offsetWithin(media, root);
	media.style.transformOrigin = `${centerX - mediaOffset.left}px ${centerY - mediaOffset.top}px`;
	return `translate(${root.offsetWidth / 2 - centerX}px, ${root.offsetHeight / 2 - centerY}px) scale(${fit})`;
}

function visibleBox(frame: HTMLElement, root: HTMLElement) {
	const { left, top } = offsetWithin(frame, root);
	const box = { left, top, width: frame.offsetWidth, height: frame.offsetHeight };
	const image = frame instanceof HTMLImageElement ? frame : frame.querySelector('img');
	if (!image || !image.naturalWidth || getComputedStyle(image).objectFit !== 'contain') return box;
	// Shrink the frame by the empty bands the contained picture leaves.
	const ratio = image.naturalWidth / image.naturalHeight;
	const bandX = (image.offsetWidth - Math.min(image.offsetWidth, image.offsetHeight * ratio)) / 2;
	const bandY = (image.offsetHeight - Math.min(image.offsetHeight, image.offsetWidth / ratio)) / 2;
	return { left: left + bandX, top: top + bandY, width: box.width - 2 * bandX, height: box.height - 2 * bandY };
}

/** Position of element inside root, from the offsetParent chain (root is positioned). */
function offsetWithin(element: HTMLElement, root: HTMLElement) {
	let left = 0;
	let top = 0;
	for (let current: Element | null = element; current && current !== root; ) {
		const node = current as HTMLElement;
		left += node.offsetLeft;
		top += node.offsetTop;
		current = node.offsetParent;
	}
	return { left, top };
}
