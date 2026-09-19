/**
 * LayoutQuote - styled blockquote layout for impactful quotes.
 */

import type { LayoutProps } from '$lib/types';
import { Slot } from '../components/Slot';

export function LayoutQuote({ slots }: LayoutProps) {
	return (
		<div className="layout-quote">
			<Slot html={slots.default} className="slot-default" />
			<Slot html={slots.attribution} className="slot-attribution" />
		</div>
	);
}
