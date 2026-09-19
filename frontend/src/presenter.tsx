import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '@fontsource/inter/400.css';
import '@fontsource/inter/700.css';
import '@fontsource/jetbrains-mono/400.css';
import './app.css';
import './lib/styles/presenter-view.css';
import { installHost } from '$lib/host';
import PresenterApp from './PresenterApp';

// See main.tsx: the presenter view also renders slides (the current and
// next-slide panels), so it needs the host object installed too.
installHost();

createRoot(document.getElementById('app')!).render(
	<StrictMode>
		<PresenterApp />
	</StrictMode>
);
