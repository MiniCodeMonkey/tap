/**
 * LayoutBlank - empty canvas for custom-positioned content.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutBlank({ slots }: LayoutProps) {
	return (
		<div className="layout-blank">
			<Slot html={slots.default} className="slot-default" />
		</div>
	);
}
