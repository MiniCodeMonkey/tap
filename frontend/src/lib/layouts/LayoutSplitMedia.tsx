/**
 * LayoutSplitMedia - image and text side by side.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutSplitMedia({ slots }: LayoutProps) {
	return (
		<div className="layout-split-media">
			<Slot html={slots.default} className="slot-default" />
			<Slot html={slots.media} className="slot-media" />
		</div>
	);
}
