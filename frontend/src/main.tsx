import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '@fontsource/inter/400.css';
import '@fontsource/inter/700.css';
import '@fontsource/jetbrains-mono/400.css';
import './app.css';
import { installHost } from '$lib/host';
import App from './App';

// Deck components share this window's React and Motion; the host object
// must exist before any component bundle's dynamic import() resolves its
// host imports, so this runs before the first render.
installHost();

createRoot(document.getElementById('app')!).render(
	<StrictMode>
		<App />
	</StrictMode>
);
