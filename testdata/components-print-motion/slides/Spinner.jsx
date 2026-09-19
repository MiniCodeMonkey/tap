/*
 * Regression fixture for print mode never animating forever: both children
 * here ignore usePrintMode() on purpose (a badly-written deck component
 * does too) and animate with repeat: Infinity - one through Motion's
 * transform animation, one through a raw CSS keyframe animation. A print
 * or settled-capture pass must still finish and land unrotated: Motion's
 * <MotionConfig reducedMotion="always"> (see DeckComponent.tsx) stops the
 * motion.div's transform animation, and the [data-print] CSS rule (see
 * DeckComponent.css) stops the CSS-only one, which Motion can never reach.
 */
import { motion } from 'motion/react';

export default function Spinner() {
	return (
		<div style={{ display: 'flex', gap: 40, padding: 40 }}>
			<motion.div
				data-testid="motion-spinner"
				animate={{ rotate: 360 }}
				transition={{ repeat: Infinity, duration: 1, ease: 'linear' }}
				style={{ width: 150, height: 150, background: '#3366ff' }}
			/>
			<div
				data-testid="css-spinner"
				style={{
					width: 150,
					height: 150,
					background: '#ff3366',
					animation: 'components-print-motion-spin 1s linear infinite'
				}}
			/>
			<style>{`
				@keyframes components-print-motion-spin {
					from { transform: rotate(0deg); }
					to { transform: rotate(360deg); }
				}
			`}</style>
		</div>
	);
}
