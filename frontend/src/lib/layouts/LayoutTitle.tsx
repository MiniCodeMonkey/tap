/**
 * LayoutTitle - centered title slide with an optional subtitle.
 * Used for the opening slide of a presentation or a section title.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutTitle({ slots }: LayoutProps) {
	return (
		<div className="layout-title">
			<Slot html={slots.default} className="slot-default" />
		</div>
	);
}
