import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { ConnectionIndicator } from './ConnectionIndicator';
import { useConnectionStore } from '$lib/stores/websocket';

const initialState = useConnectionStore.getState();

afterEach(() => {
	cleanup();
	useConnectionStore.setState(initialState, true);
});

describe('ConnectionIndicator', () => {
	it('renders nothing before static mode detection completes', () => {
		useConnectionStore.setState({ staticModeDetected: false, staticMode: false, connected: false });

		const { container } = render(<ConnectionIndicator />);

		expect(container.firstChild).toBeNull();
	});

	it('renders nothing in static mode', () => {
		useConnectionStore.setState({ staticModeDetected: true, staticMode: true, connected: false });

		const { container } = render(<ConnectionIndicator />);

		expect(container.firstChild).toBeNull();
	});

	it('renders nothing while connected', () => {
		useConnectionStore.setState({ staticModeDetected: true, staticMode: false, connected: true });

		const { container } = render(<ConnectionIndicator />);

		expect(container.firstChild).toBeNull();
	});

	it('shows the disconnected class when the store reports a dropped connection', () => {
		useConnectionStore.setState({
			staticModeDetected: true,
			staticMode: false,
			connected: false,
			reconnecting: false,
			reconnectAttempt: 0
		});

		const { container } = render(<ConnectionIndicator />);

		const indicator = container.querySelector('.connection-indicator');
		expect(indicator).toHaveClass('disconnected');
		expect(indicator).toHaveTextContent('Disconnected');
	});

	it('shows a reconnecting message and class while retrying', () => {
		useConnectionStore.setState({
			staticModeDetected: true,
			staticMode: false,
			connected: false,
			reconnecting: true,
			reconnectAttempt: 3
		});

		const { container } = render(<ConnectionIndicator />);

		const indicator = container.querySelector('.connection-indicator');
		expect(indicator).toHaveClass('reconnecting');
		expect(indicator).toHaveTextContent('Reconnecting... (3)');
	});
});
