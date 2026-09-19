/*
 * Whole-slide component contract: props are
 * { slots, props, slide, step, steps, active, printMode }. `step`,
 * `steps`, `active`, and `printMode` also arrive as props, but the tap
 * helper module's hooks (useStep, useActive, usePrintMode) are the
 * idiomatic way to read them - use whichever is more convenient deeper in
 * a component tree, without threading props down.
 *
 * export const steps = 5. Step 0: all four servers serve v1. Steps 1-4:
 * the server at that step's index drains, restarts on v2, and rejoins,
 * while the other three keep serving. Step 5: all four servers serve v2.
 *
 * Print mode / preview thumbnails pass step = steps, printMode = true:
 * render the final state, no animation or loops (each is guarded by
 * `active && !printMode`).
 *
 * Color rule: `theme.accent` is a fill color. Text or thin borders that
 * sit directly on `theme.bg` (the load balancer bar, connector lines) use
 * `theme.accentText` or `currentColor`, never `theme.accent` as text -
 * on a light-on-light theme like zine, accent-as-text is unreadable.
 * Text painted ON an accent fill (the v2 server's big version label)
 * needs whichever of `theme.bg` / `theme.fg` contrasts with that theme's
 * accent, which differs per theme, so `textOn` (from the tap module)
 * picks it at render time with a contrast check instead of hardcoding one.
 */
import { Slot, Step, textOn, useActive, usePrintMode, useStep, useTheme } from 'tap';
import { motion } from 'motion/react';
import { useLayoutEffect, useState } from 'react';

export const steps = 5;

const SERVER_COUNT = 4;
const DRAIN_MS = 450;
const RESTART_MS = 950;

/** The server at `index`'s version and state for the given step and roll phase. */
function serverState(index, step, phase) {
	const rollingIndex = step - 1;
	if (index < rollingIndex) return { version: 'v2', state: 'serving' };
	if (index > rollingIndex) return { version: 'v1', state: 'serving' };
	if (phase === 'serving') return { version: 'v2', state: 'serving' };
	return { version: 'v1', state: phase };
}

/** Box styling for one server, by its state and theme tokens. */
function boxStyle(state, isUpgraded, theme, textOnAccent) {
	const base = { flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', borderRadius: theme.radius, fontFamily: theme.fontMono, textAlign: 'center' };
	if (isUpgraded) return { ...base, background: theme.accent, border: `calc(${theme.strokeWidth} * 2) solid ${theme.accentText}`, color: textOnAccent };
	if (state !== 'serving') return { ...base, background: 'transparent', border: `${theme.strokeWidth} dashed ${theme.muted}`, color: theme.muted };
	return { ...base, background: theme.surface, border: `${theme.strokeWidth} solid ${theme.muted}`, color: theme.fg };
}

export default function RollingDeploy({ slots }) {
	const { step, steps: totalSteps } = useStep();
	const active = useActive();
	const printMode = usePrintMode();
	const theme = useTheme();
	const rollingIndex = step - 1;
	const [phase, setPhase] = useState('serving');
	const textOnAccent = textOn(theme.accent, theme);

	// A layout effect, not a plain effect: it resets `phase` to 'draining'
	// before the browser paints the new step, so a press never paints one
	// frame of the previous step's 'serving' box for the server that is
	// about to drain.
	useLayoutEffect(() => {
		if (!active || printMode || rollingIndex < 0 || rollingIndex >= SERVER_COUNT) {
			setPhase('serving');
			return;
		}
		setPhase('draining');
		const toRestart = setTimeout(() => setPhase('restarting'), DRAIN_MS);
		const toServing = setTimeout(() => setPhase('serving'), RESTART_MS);
		return () => {
			clearTimeout(toRestart);
			clearTimeout(toServing);
		};
	}, [step, active, printMode]);

	return (
		<div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', gap: theme.spaceUnit, fontFamily: theme.fontBody, color: theme.fg }}>
			<Slot html={slots.default} />

			<div style={{ display: 'flex', flexDirection: 'column', gap: `calc(${theme.spaceUnit} / 2)`, flex: 1, minHeight: 0 }}>
				<div
					style={{
						textAlign: 'center',
						padding: `calc(${theme.spaceUnit} / 2) ${theme.spaceUnit}`,
						border: `${theme.strokeWidth} solid ${theme.accentText}`,
						borderRadius: theme.radius,
						fontFamily: theme.fontMono,
						fontSize: '1.75rem',
						color: theme.accentText
					}}
				>
					load balancer
				</div>

				{/* One connector per server: solid with a traveling request dot
				    while serving, dashed while draining or restarting. */}
				<div style={{ display: 'flex', height: `calc(${theme.spaceUnit} * 2)` }}>
					{Array.from({ length: SERVER_COUNT }, (_, index) => {
						const connected = serverState(index, step, phase).state === 'serving';
						return (
							<div key={index} style={{ flex: 1, position: 'relative', display: 'flex', justifyContent: 'center' }}>
								<div style={{ width: 0, height: '100%', borderLeft: `${theme.strokeWidth} ${connected ? 'solid' : 'dashed'} ${theme.muted}`, opacity: connected ? 1 : 0.5 }} />
								{connected && !printMode && active && (
									<motion.div
										aria-hidden="true"
										style={{ position: 'absolute', top: 0, width: '0.6rem', height: '0.6rem', borderRadius: '50%', background: theme.accentText }}
										animate={{ top: ['0%', '90%'], opacity: [0, 1, 0] }}
										transition={{ duration: 0.9, repeat: Infinity, ease: 'linear' }}
									/>
								)}
							</div>
						);
					})}
				</div>

				<div style={{ display: 'flex', gap: theme.spaceUnit, flex: 1, minHeight: 0 }}>
					{Array.from({ length: SERVER_COUNT }, (_, index) => {
						const { version, state } = serverState(index, step, phase);
						const isUpgraded = version === 'v2' && state === 'serving';
						const isRestarting = state === 'restarting';
						const pulsing = isRestarting && !printMode && active;

						return (
							<motion.div
								key={index}
								data-testid={`server-${index}`}
								data-version={version}
								data-state={state}
								style={boxStyle(state, isUpgraded, theme, textOnAccent)}
								animate={pulsing ? { opacity: [0.35, 0.85, 0.35] } : { opacity: 1 }}
								transition={pulsing ? { duration: 0.9, repeat: Infinity } : { duration: 0.2 }}
							>
								{isRestarting ? (
									<div style={{ fontSize: '1.5rem' }}>restarting&hellip;</div>
								) : (
									<>
										<div style={{ fontSize: isUpgraded ? '3.25rem' : '2rem', fontWeight: isUpgraded ? 'bold' : 'normal' }}>{version}</div>
										<div style={{ fontSize: '1.5rem', marginTop: `calc(${theme.spaceUnit} / 4)` }}>server {index} &middot; {state}</div>
									</>
								)}
							</motion.div>
						);
					})}
				</div>

				{/* Only visible once the clicker reaches the slide's last step
				    (and always in print mode, which shows the final state). */}
				<Step at={totalSteps}>
					<div style={{ fontSize: '1.5rem', color: theme.accentText, textAlign: 'center' }}>Rollout complete. Every server now serves v2.</div>
				</Step>
			</div>

			<div style={{ fontSize: '2.25rem' }}>
				<Slot html={slots.caption} />
			</div>
		</div>
	);
}
