/**
 * LayoutDefault - standard content layout for general-purpose slides.
 * Falls back layout for any unrecognized layout name.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutDefault({ slots }: LayoutProps) {
	return (
		<div className="layout-default">
			<Slot html={slots.default} className="slot-default" />
		</div>
	);
}
