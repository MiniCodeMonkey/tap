/**
 * Renders one named slot's HTML and applies fragment visibility.
 * Layouts render slots through this component and never handle fragment
 * classes themselves.
 */

import { useContext, useLayoutEffect, useRef, type JSX } from 'react';
import { SlideContext } from './SlideContext';

export interface SlotProps {
	html: string | undefined;
	className?: string;
}

export function Slot({ html, className }: SlotProps): JSX.Element | null {
	const ref = useRef<HTMLDivElement>(null);
	const { fragmentIndex, printMode } = useContext(SlideContext);

	useLayoutEffect(() => {
		const element = ref.current;
		if (!element) return;

		const fragments = element.querySelectorAll<HTMLElement>('[data-fragment-index]');
		fragments.forEach((fragment) => {
			const index = Number(fragment.getAttribute('data-fragment-index'));
			const visible = printMode || index <= fragmentIndex;
			fragment.classList.toggle('fragment-visible', visible);
			fragment.classList.toggle('fragment-hidden', !visible);
		});
	}, [html, fragmentIndex, printMode]);

	if (html === undefined) {
		return null;
	}

	const classes = ['slot', 'slot-content', className].filter(Boolean).join(' ');

	return <div ref={ref} className={classes} dangerouslySetInnerHTML={{ __html: html }} />;
}
