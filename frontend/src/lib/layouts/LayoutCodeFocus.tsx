/**
 * LayoutCodeFocus - full-width code block layout.
 * The code block takes prominence, filling most of the available space.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutCodeFocus({ slots }: LayoutProps) {
	return (
		<div className="layout-code-focus">
			<Slot html={slots.default} className="slot-default" />
		</div>
	);
}
