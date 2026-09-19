/**
 * LayoutBigStat - large number emphasis layout for statistics.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutBigStat({ slots }: LayoutProps) {
	return (
		<div className="layout-big-stat">
			<Slot html={slots.default} className="slot-default" />
			<Slot html={slots.caption} className="slot-caption" />
			<Slot html={slots.figure} className="slot-figure" />
		</div>
	);
}
