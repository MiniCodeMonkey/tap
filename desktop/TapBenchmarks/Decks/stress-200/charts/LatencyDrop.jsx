/*
 * Inline component contract: props are
 * { slots, props, slide, step, steps, active, printMode }.
 * `props` holds the fence body's JSON:
 * { "before": 412, "after": 88, "unit": "ms", "label": "p95 latency" }.
 * Bars animate in once the slide is active; print mode renders the final
 * heights with no animation.
 *
 * Color rule: `theme.accent` and `theme.muted` are fill colors here (the
 * "after" and "before" bars), never used as text. Every label sits on
 * `theme.bg`, so it uses `currentColor` (inherited from the root's
 * `theme.fg`), which stays readable in every theme.
 */
import { useActive, usePrintMode, useTheme } from 'tap';
import { motion } from 'motion/react';

const MAX_BAR_HEIGHT = 220;

export default function LatencyDrop({ props }) {
	const active = useActive();
	const printMode = usePrintMode();
	const theme = useTheme();

	const before = props.before ?? 0;
	const after = props.after ?? 0;
	const unit = props.unit ?? 'ms';
	const label = props.label ?? 'latency';
	const scale = MAX_BAR_HEIGHT / Math.max(before, after, 1);
	const shown = active || printMode;

	const bars = [
		{ key: 'before', value: before, color: theme.muted },
		{ key: 'after', value: after, color: theme.accent }
	];

	return (
		<div style={{ fontFamily: theme.fontBody, color: theme.fg }}>
			<div style={{ fontSize: '1.5rem', color: theme.muted, marginBottom: `calc(${theme.spaceUnit} / 2)` }}>{label}</div>
			<div style={{ display: 'flex', alignItems: 'flex-end', gap: theme.spaceUnit }}>
				{bars.map((bar) => (
					<div key={bar.key} style={{ textAlign: 'center', fontFamily: theme.fontMono }}>
						<motion.div
							initial={printMode ? false : { height: 0 }}
							animate={{ height: shown ? bar.value * scale : 0 }}
							transition={{ duration: printMode ? 0 : 0.5 }}
							style={{ width: '5.5rem', background: bar.color, borderRadius: theme.radius, border: `${theme.strokeWidth} solid currentColor` }}
						/>
						<div style={{ fontSize: '1.75rem', marginTop: `calc(${theme.spaceUnit} / 4)` }}>{bar.value}{unit}</div>
						<div style={{ fontSize: '1.3rem', color: theme.muted }}>{bar.key}</div>
					</div>
				))}
			</div>
		</div>
	);
}
