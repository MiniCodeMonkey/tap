/**
 * Subtle corner indicator shown while the WebSocket connection is down.
 * Hidden entirely once connected, in static mode, or before static mode
 * detection finishes, so it never flashes on initial load.
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

	const statusText = reconnecting && reconnectAttempt > 0 ? `Reconnecting... (${reconnectAttempt})` : 'Disconnected';

	return (
		<div
			className={`connection-indicator disconnected${reconnecting ? ' reconnecting' : ''}`}
			role="status"
			aria-live="polite"
			aria-label={statusText}
		>
			<span className={`indicator-dot${reconnecting ? ' pulse' : ''}`} />
			<span className="indicator-text">{statusText}</span>
		</div>
	);
}
