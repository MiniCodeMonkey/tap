// Fixture for the components e2e suite: a deliberate syntax error so
// internal/components.Build fails and the frontend shows the error card.
import { useTheme } from 'tap';

export default function Broken({ slots }) {
	const theme = useTheme(
	return <div>this never parses</div>;
}
