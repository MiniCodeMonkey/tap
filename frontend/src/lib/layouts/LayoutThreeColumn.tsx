/**
 * LayoutThreeColumn - three-column grid layout.
 * The default slot spans all three columns as a header; left, center and
 * right hold the column content.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutThreeColumn({ slots }: LayoutProps) {
	return (
		<div className="layout-three-column">
			<Slot html={slots.default} className="slot-default" />
			<Slot html={slots.left} className="slot-left" />
			<Slot html={slots.center} className="slot-center" />
			<Slot html={slots.right} className="slot-right" />
		</div>
	);
}
