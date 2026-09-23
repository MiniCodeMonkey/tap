/**
 * Loads and renders one deck-supplied component bundle: the whole-slide
 * form (through LayoutComponent) and each inline `data-component-index`
 * placeholder mount one of these. Loading is a dynamic import() cached by
 * URL, rendered inside React.Suspense (fallback: nothing, so a slide never
 * flashes) and its own error boundary, so one inline component's failure
 * never takes the rest of the slide down with it.
 */

import {
	Component,
	lazy,
	Suspense,
	useCallback,
	useEffect,
	useMemo,
	useRef,
	useState,
	type ComponentType,
	type ReactNode
} from 'react';
import { MotionConfig, PresenceContext } from 'motion/react';
import type { DeckComponentProps, Slide as SlideData } from '$lib/types';
import { isDevRuntime } from '$lib/utils/runtime';
import { useSafeErrorForm } from '$lib/hooks/useSafeErrorForm';
import { useReadyHold } from '$lib/ready/blockers';
import { DeckComponentContext } from '../tap';
import './DeckComponent.css';

/** A dynamic import() function, swappable in tests: jsdom cannot import() a URL. */
export type ComponentImporter = (url: string) => Promise<unknown>;

const defaultImporter: ComponentImporter = (url) => import(/* @vite-ignore */ url);

/** The raw module promise for a bundle URL, cached by absolute URL so a second mount never re-imports it. */
const importCache = new Map<string, Promise<unknown>>();
const loadedStylesheets = new Set<string>();

/**
 * Resolves a bundle URL against the document's base URI, not against the
 * module that imports it - required for a static build, where the slide
 * JSON's URL is relative ("components/Foo-abcd.js").
 */
export function resolveComponentURL(url: string): string {
	return new URL(url, document.baseURI).href;
}

/** Adds one <link rel="stylesheet" data-deck-component> for `cssURL` to document.head, once per absolute URL. */
function loadStylesheet(cssURL: string): void {
	const absolute = resolveComponentURL(cssURL);
	if (loadedStylesheets.has(absolute)) return;
	loadedStylesheets.add(absolute);
	const link = document.createElement('link');
	link.rel = 'stylesheet';
	link.href = absolute;
	link.dataset.deckComponent = '';
	document.head.appendChild(link);
}

/** How long a dynamic import may sit unresolved before it is treated as a load failure. */
export const COMPONENT_LOAD_TIMEOUT_MS = 8000;

/**
 * Imports a bundle's module namespace, caching the promise by its absolute
 * URL. A rejection (a 404 during a `tap dev` restart, a build failure)
 * evicts the entry instead of caching the rejection forever, so leaving
 * and re-entering the slide retries the import instead of repeating
 * "Failed to fetch dynamically imported module".
 *
 * `timeout`, when given, races the import against COMPONENT_LOAD_TIMEOUT_MS:
 * an import() that never settles (a bundler stuck mid-build, a network
 * request that never completes) would otherwise leave the slide blank
 * forever, since Suspense's fallback has nothing else to show. A timeout
 * is treated exactly like a rejected import - same cache eviction, same
 * error boundary - with a message naming the source that never loaded.
 * Omitted for a print/capture pass, which waits on its own terms instead.
 */
function importModule(
	url: string,
	importer: ComponentImporter = defaultImporter,
	timeout?: { source: string }
): Promise<unknown> {
	const absolute = resolveComponentURL(url);
	let cached = importCache.get(absolute);
	if (!cached) {
		const importPromise = importer(absolute);
		let racedPromise: Promise<unknown> = importPromise;
		if (timeout) {
			let timeoutId: ReturnType<typeof setTimeout>;
			const timeoutPromise = new Promise<never>((_, reject) => {
				timeoutId = setTimeout(() => {
					reject(new Error(`component did not load within 8 seconds: ${timeout.source}`));
				}, COMPONENT_LOAD_TIMEOUT_MS);
			});
			racedPromise = Promise.race([importPromise, timeoutPromise]);
			// The import can still resolve after the timeout fires (a slow but
			// eventually successful build); clearing on both settle paths avoids
			// leaving a stray timer around, and an unhandled rejection warning
			// for the timeout promise once nothing is racing it any more.
			importPromise.then(
				() => clearTimeout(timeoutId),
				() => clearTimeout(timeoutId)
			);
		}
		cached = racedPromise.catch((error: unknown) => {
			importCache.delete(absolute);
			throw error;
		});
		importCache.set(absolute, cached);
	}
	return cached;
}

/**
 * Loads a component bundle's default export, caching the underlying
 * import() by its absolute URL so repeated mounts (another inline
 * placeholder, a re-render) never re-import the same bundle. A bundle with
 * no default export rejects with a clear message instead of silently
 * rendering nothing.
 *
 * DeckComponent itself never calls this directly - it renders through
 * makeLazyComponent's React.lazy() wrapper below, which shares the same
 * importModule cache. Exported only because DeckComponent.test.tsx drives
 * the cache and the rejection message through this function directly,
 * without the overhead of mounting a component tree for it.
 */
export function loadDeckComponent(
	url: string,
	importer?: ComponentImporter
): Promise<ComponentType<DeckComponentProps>> {
	return importModule(url, importer).then((module) => {
		const exported = (module as { default?: unknown }).default;
		if (typeof exported !== 'function') {
			throw new Error(`${url} has no default export`);
		}
		return exported as ComponentType<DeckComponentProps>;
	});
}

/** Whether a loaded bundle module opts out of preview/thumbnail rendering with `export const preview = false`. */
export function isPreviewDisabled(module: { preview?: unknown }): boolean {
	return module.preview === false;
}

/**
 * The error card shown in dev when a component fails to build or render.
 * Detected by `tap export images` via the deck-error-card class, and by `tap
 * export pdf` via its data-message attribute (see ErrorCardSelector in
 * internal/pdf/capture.go) - both present in both forms below, so neither
 * tool needs to parse the card's visible text apart from its source path.
 *
 * In the audience-safe form (see shouldUseSafeErrorForm), the full card is
 * kept in the DOM - visually hidden, its message moved to a data-message
 * attribute - rather than removed, so tap export images and tests still find
 * it by class; only a small "component error" marker is visible next to
 * `fallback` (the slide's normal slot content for a whole-slide component,
 * nothing for an inline one), so the room sees the slide instead of an
 * empty one, and never a raw stack-trace-flavored message.
 */
function ErrorCard({ source, message, fallback = null }: { source: string; message: string; fallback?: ReactNode }) {
	const safe = useSafeErrorForm();
	if (safe) {
		return (
			<>
				{fallback}
				<div className="deck-error-card deck-error-card-safe" data-source={source} data-message={message} hidden />
				<div className="deck-error-marker" title={`${source}: ${message}`}>
					component error
				</div>
			</>
		);
	}
	return (
		<div className="deck-error-card" data-source={source} data-message={message}>
			<p className="deck-error-card-source">{source}</p>
			<p className="deck-error-card-message">{message}</p>
		</div>
	);
}

/** Small placeholder shown in a thumbnail/preview render when the module opts out with `export const preview = false`. */
function PreviewPlaceholderCard({ source }: { source: string }) {
	return (
		<div className="deck-component-preview-placeholder" data-source={source}>
			{source}
		</div>
	);
}

interface DeckComponentBoundaryProps {
	source: string;
	buildFallback: ReactNode;
	children: ReactNode;
	/** Called after the boundary has committed its error card or fallback. */
	onCaught?: () => void;
}

interface DeckComponentBoundaryState {
	error: Error | null;
}

/**
 * Catches a load or render failure for one component, own to this
 * component so it never propagates to SlideErrorBoundary and takes the
 * whole slide down. Dev shows an error card with the source and message;
 * a build renders the caller's fallback (raw slot content for an inline
 * component, or nothing).
 */
class DeckComponentBoundary extends Component<DeckComponentBoundaryProps, DeckComponentBoundaryState> {
	state: DeckComponentBoundaryState = { error: null };

	static getDerivedStateFromError(error: unknown): DeckComponentBoundaryState {
		return { error: error instanceof Error ? error : new Error(String(error)) };
	}

	componentDidCatch(error: unknown): void {
		console.error(`[tap] Component ${this.props.source} failed to render`, error);
		this.props.onCaught?.();
	}

	render(): ReactNode {
		if (this.state.error) {
			if (isDevRuntime()) {
				return (
					<ErrorCard
						source={this.props.source}
						message={this.state.error.message}
						fallback={this.props.buildFallback}
					/>
				);
			}
			return this.props.buildFallback;
		}
		return this.props.children;
	}
}

/**
 * Calls onSettled when it commits. Rendered inside Suspense next to the
 * lazy component, it commits only once the bundle has loaded and the
 * component has rendered.
 */
function ComponentSettled({ onSettled }: { onSettled: () => void }) {
	useEffect(() => {
		onSettled();
	}, [onSettled]);
	return null;
}

/** Where one bundle is on its way to the screen, for the ready signal. */
type LoadPhase = 'loading' | 'failed' | 'settled';

export interface DeckComponentHostProps {
	/** The component file's path, relative to the deck, e.g. "slides/RollingDeploy.jsx". */
	source: string;
	/** The bundle's JS URL. */
	url: string;
	/** The bundle's CSS URL, when the component imports any. */
	css?: string;
	/** Formatted build error from the backend bundle; when set, the component is never loaded. */
	buildError?: string;
	props: Record<string, unknown>;
	slots: Record<string, string>;
	slide: SlideData;
	step: number;
	steps: number;
	active: boolean;
	printMode: boolean;
	/** True for a thumbnail/preview render. */
	preview?: boolean;
	/** Rendered in a build when the component can't be shown (raw slot content for inline, nothing for whole-slide - LayoutComponent supplies its own default-layout fallback instead of using this prop). */
	buildFallback?: ReactNode;
	/** Test seam: replaces dynamic import(). */
	importer?: ComponentImporter;
}

/** Renders one deck component bundle: its own error boundary and Suspense around a dynamically imported default export. */
export function DeckComponent({
	source,
	url,
	css,
	buildError,
	props,
	slots,
	slide,
	step,
	steps,
	active,
	printMode,
	preview = false,
	buildFallback = null,
	importer
}: DeckComponentHostProps) {
	const rootRef = useRef<HTMLDivElement | null>(null);

	useEffect(() => {
		if (css) loadStylesheet(css);
	}, [css]);

	const contextValue = useMemo(
		() => ({ step, steps, active, printMode, preview, rootRef }),
		[step, steps, active, printMode, preview]
	);

	// Computed unconditionally, before the buildError early return below, so
	// every render calls the same hooks in the same order (a lookup in
	// lazyCache is cheap even when buildError means it never renders).
	const LazyComponent = useMemo(
		() => makeLazyComponent(url, importer, preview, source, printMode),
		[url, importer, preview, source, printMode]
	);

	// The ready signal waits for this component: a bundle that is still
	// loading holds a "component" blocker, and one that failed holds an
	// "error-card" blocker until the boundary below has committed its card
	// or fallback. A build error renders its card at once and holds
	// nothing. Keyed by source and URL, like the boundary, so a new bundle
	// starts over.
	const loadKey = `${source}\u0000${url}`;
	const [load, setLoad] = useState<{ key: string; phase: LoadPhase }>({ key: loadKey, phase: 'loading' });
	const phase: LoadPhase = buildError ? 'settled' : load.key === loadKey ? load.phase : 'loading';
	useReadyHold('component', phase === 'loading');
	useReadyHold('error-card', phase === 'failed');
	const markSettled = useCallback(() => setLoad({ key: loadKey, phase: 'settled' }), [loadKey]);

	// Watches the same cached import the lazy component uses, to learn
	// that it failed before the boundary has shown the error.
	useEffect(() => {
		if (buildError) {
			return undefined;
		}
		let cancelled = false;
		importModule(url, importer, printMode ? undefined : { source }).catch(() => {
			if (cancelled) return;
			setLoad((previous) =>
				previous.key === loadKey && previous.phase === 'settled' ? previous : { key: loadKey, phase: 'failed' }
			);
		});
		return () => {
			cancelled = true;
		};
	}, [buildError, url, importer, printMode, source, loadKey]);

	if (buildError) {
		return (
			<div ref={rootRef} className="deck-component-root">
				{isDevRuntime() ? (
					<ErrorCard source={source} message={buildError} fallback={buildFallback} />
				) : (
					buildFallback
				)}
			</div>
		);
	}

	return (
		<div ref={rootRef} className="deck-component-root">
			<DeckComponentContext.Provider value={contextValue}>
				{/* Keyed by source+url: a step change must not reset an error a
				    component already threw (retrying on every press would flicker
				    the error card), but a different bundle is a fresh mount, so its
				    error boundary starts clean instead of carrying over a failure
				    from whatever used to be at this placeholder. */}
				<DeckComponentBoundary
					key={`${source}\u0000${url}`}
					source={source}
					buildFallback={buildFallback}
					onCaught={markSettled}
				>
					<Suspense fallback={null}>
						{/* Resets presence context to null instead of inheriting
						    SlideTransition's outer <AnimatePresence initial={false}>,
						    which otherwise reaches every motion element in a deck
						    component's own tree and blocks its mount animation on
						    the first render of the slide it starts on (a reload, a
						    deep link, `tap export images --step k`). A component that
						    wants its own AnimatePresence still nests one normally
						    under this null value. */}
						<PresenceContext.Provider value={null}>
							{/* Print mode (and a settled capture, which passes printMode
							    the same way) forces every Motion transform and layout
							    animation in the component's tree to its end state
							    instantly, so the ready signal's animation check never
							    waits on one and a screenshot never lands mid-animation.
							    It leaves opacity and color animations running - see
							    DeckComponent.css for the CSS-driven animations this does
							    not reach. */}
							<MotionConfig reducedMotion={printMode ? 'always' : 'never'}>
								<LazyComponent slots={slots} props={props} slide={slide} step={step} steps={steps} active={active} printMode={printMode} />
								<ComponentSettled onSettled={markSettled} />
							</MotionConfig>
						</PresenceContext.Provider>
					</Suspense>
				</DeckComponentBoundary>
			</DeckComponentContext.Provider>
		</div>
	);
}

const lazyCache = new Map<string, ComponentType<DeckComponentProps>>();

/**
 * React.lazy() wrapping the bundle's module load, memoized per URL,
 * importer, and preview-render flag so repeated renders of the same
 * placeholder reuse the same lazy component instead of resuspending on
 * every render. In a preview render, resolves to a placeholder card
 * instead of the loaded default export when the module opts out with
 * `export const preview = false`. A rejected load evicts this entry too
 * (React.lazy caches its own rejected promise forever otherwise), so a
 * remount of the same URL builds a fresh lazy component that retries.
 *
 * `printMode` skips the load timeout below: a print/capture pass already
 * waits on its own terms (see internal/pdf/capture.go), and a fixed
 * frontend timeout on top of that would only race it.
 */
function makeLazyComponent(
	url: string,
	importer: ComponentImporter | undefined,
	isPreviewRender: boolean,
	source: string,
	printMode: boolean
): ComponentType<DeckComponentProps> {
	const cacheKey = `${url}\u0000${importer ? 'custom' : 'default'}\u0000${isPreviewRender}`;
	let cached = lazyCache.get(cacheKey);
	if (!cached) {
		cached = lazy(async (): Promise<{ default: ComponentType<DeckComponentProps> }> => {
			try {
				const module = await importModule(url, importer, printMode ? undefined : { source });
				const exported = (module as { default?: unknown }).default;
				if (typeof exported !== 'function') {
					throw new Error(`${url} has no default export`);
				}
				if (isPreviewRender && isPreviewDisabled(module as { preview?: unknown })) {
					return { default: () => <PreviewPlaceholderCard source={source} /> };
				}
				return { default: exported as ComponentType<DeckComponentProps> };
			} catch (error) {
				lazyCache.delete(cacheKey);
				throw error;
			}
		});
		lazyCache.set(cacheKey, cached);
	}
	return cached;
}

// Reset caches for tests: never called from production code.
export function __resetDeckComponentCachesForTests(): void {
	importCache.clear();
	loadedStylesheets.clear();
	lazyCache.clear();
}
