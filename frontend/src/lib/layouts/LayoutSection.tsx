/**
 * LayoutSection - large section divider between major parts of a deck.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutSection({ slots }: LayoutProps) {
	return (
		<div className="layout-section">
			<Slot html={slots.default} className="slot-default" />
		</div>
	);
}
