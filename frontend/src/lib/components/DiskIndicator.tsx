/**
 * Corner badge shown while a recording is close to filling the disk, or
 * after it stopped because the disk is full. It uses the offline badge's
 * corner and look, and hides while disconnected, where that badge is shown
 * and the disk status cannot be current anyway.
 */

import { useConnectionStore } from '$lib/stores/websocket';

const LABELS = {
	low: { title: 'Disk almost full', detail: 'Recording stops at 1 GB', aria: 'Disk almost full, recording stops at 1 GB' },
	full: { title: 'Recording stopped', detail: 'Disk full', aria: 'Recording stopped: disk full' }
} as const;

export function DiskIndicator() {
	const connected = useConnectionStore((state) => state.connected);
	const diskStatus = useConnectionStore((state) => state.diskStatus);

	if (!connected || diskStatus === 'ok') {
		return null;
	}

	const label = LABELS[diskStatus];
	return (
		<div className={`connection-indicator disk-indicator disk-${diskStatus}`} role="status" aria-live="polite" aria-label={label.aria}>
			<span className="indicator-dot" />
			<span className="indicator-text">
				<span className="indicator-title">{label.title}</span>
				<span className="indicator-detail">{label.detail}</span>
			</span>
		</div>
	);
}
