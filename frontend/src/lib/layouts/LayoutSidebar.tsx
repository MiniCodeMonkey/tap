/**
 * LayoutSidebar - main content with a supporting sidebar.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutSidebar({ slots }: LayoutProps) {
	return (
		<div className="layout-sidebar">
			<Slot html={slots.default} className="slot-default" />
			<Slot html={slots.sidebar} className="slot-sidebar" />
		</div>
	);
}
