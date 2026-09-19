/*
 * Regression fixture for the mount-time animation bug: a whole-slide
 * component's own <motion.div initial=.../> must play on the very first
 * render of the slide it starts on (a page load on this slide's hash),
 * not jump straight to its end state because SlideTransition's outer
 * <AnimatePresence initial={false}> reaches into it. See
 * DeckComponent.tsx's PresenceContext.Provider reset.
 */
import { motion } from 'motion/react';

export default function Fade() {
	return (
		<motion.div
			data-testid="fade-target"
			initial={{ opacity: 0 }}
			animate={{ opacity: 1 }}
			transition={{ duration: 1 }}
			style={{ width: 200, height: 200, background: 'red' }}
		/>
	);
}
