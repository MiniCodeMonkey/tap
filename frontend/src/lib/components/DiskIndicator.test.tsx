import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { DiskIndicator } from './DiskIndicator';
import { useConnectionStore } from '$lib/stores/websocket';

const initialState = useConnectionStore.getState();

afterEach(() => {
	cleanup();
	useConnectionStore.setState(initialState, true);
});

describe('DiskIndicator', () => {
	it('renders nothing while the disk is fine', () => {
		useConnectionStore.setState({ connected: true, diskStatus: 'ok' });
		const { container } = render(<DiskIndicator />);
		expect(container.firstChild).toBeNull();
	});

	it('warns when the disk is low', () => {
		useConnectionStore.setState({ connected: true, diskStatus: 'low' });
		render(<DiskIndicator />);
		expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Disk almost full, recording stops at 1 GB');
	});

	it('says the recording stopped when the disk is full', () => {
		useConnectionStore.setState({ connected: true, diskStatus: 'full' });
		render(<DiskIndicator />);
		expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Recording stopped: disk full');
		expect(screen.getByRole('status')).toHaveClass('disk-full');
	});

	it('hides while disconnected, where the offline badge takes the corner', () => {
		useConnectionStore.setState({ connected: false, diskStatus: 'low' });
		const { container } = render(<DiskIndicator />);
		expect(container.firstChild).toBeNull();
	});
});
