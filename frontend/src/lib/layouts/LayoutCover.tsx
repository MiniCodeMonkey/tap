/**
 * LayoutCover - full-bleed background layout for dramatic opening slides.
 * The background image or color comes from the slide's background config.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutCover({ slots }: LayoutProps) {
	return (
		<div className="layout-cover">
			<Slot html={slots.default} className="slot-default" />
		</div>
	);
}
