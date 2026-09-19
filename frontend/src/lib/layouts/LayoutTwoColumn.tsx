/**
 * LayoutTwoColumn - side-by-side two-column layout.
 * The default slot spans both columns as a header; left and right hold the
 * column content.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutTwoColumn({ slots }: LayoutProps) {
	return (
		<div className="layout-two-column">
			<Slot html={slots.default} className="slot-default" />
			<Slot html={slots.left} className="slot-left" />
			<Slot html={slots.right} className="slot-right" />
		</div>
	);
}
