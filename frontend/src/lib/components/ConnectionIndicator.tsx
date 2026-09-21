/**
 * Corner pill shown while the Tap server cannot be reached. Quiet enough to
 * leave up during a talk, but it names the host it is trying, so a tab left
 * open on a stopped server (an old port) is easy to spot. Hidden entirely
 * once connected, in static mode, or before static mode detection finishes,
 * so it never flashes on initial load.
 */

import { useConnectionStore } from '$lib/stores/websocket';

export function ConnectionIndicator() {
	const connected = useConnectionStore((state) => state.connected);
	const reconnecting = useConnectionStore((state) => state.reconnecting);
	const reconnectAttempt = useConnectionStore((state) => state.reconnectAttempt);
	const staticMode = useConnectionStore((state) => state.staticMode);
	const staticModeDetected = useConnectionStore((state) => state.staticModeDetected);

	const shouldShow = staticModeDetected && !staticMode && !connected;
	if (!shouldShow) {
		return null;
	}

	const host = typeof window === 'undefined' ? '' : window.location.host;
	const detail = reconnecting && reconnectAttempt > 0 ? `Reconnecting... (${reconnectAttempt})` : 'Disconnected';
	const label = host ? `Tap server offline at ${host}. ${detail}` : `Tap server offline. ${detail}`;

	return (
		<div
			className={`connection-indicator disconnected${reconnecting ? ' reconnecting' : ''}`}
			role="status"
			aria-live="polite"
			aria-label={label}
		>
			<span className={`indicator-dot${reconnecting ? ' pulse' : ''}`} />
			<span className="indicator-text">
				<span className="indicator-title">Tap server offline</span>
				<span className="indicator-detail">
					{host ? `${host} · ` : ''}
					{detail}
				</span>
			</span>
		</div>
	);
}
