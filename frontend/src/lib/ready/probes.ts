/**
 * The page checks the ready signal runs in each round: stylesheets and web
 * fonts, images, and running animations, plus two animation frames so the
 * last change has painted. Each check resolves when there is nothing left
 * to wait for, or when its time limit runs out, so a broken image or a
 * looping animation never holds the signal forever. The limits are the ones
 * tap's exporter has always used. On a live page the paint check ends on a
 * hidden page at once, and after PAINT_TIMEOUT_MS on a page that reports
 * itself visible but runs no animation frames. A page that requires a
 * paint (see DomProbeOptions.requirePaint) has no paint limit: it reports
 * ready only after two real frames.
 */

/** How long to wait for stylesheets to load. */
export const STYLESHEET_TIMEOUT_MS = 5000;

/** How long to wait for images to load. */
export const IMAGE_TIMEOUT_MS = 5000;

/** How long to wait for running animations to finish. */
export const ANIMATION_TIMEOUT_MS = 3000;

/**
 * How long a live page waits for the animation frames that show it has
 * painted. A window the window server reports visible can still run no
 * frames, while it is covered or not composited, so a live page that
 * waited for them without a limit could never report ready. A page that
 * requires a paint ignores this limit.
 */
export const PAINT_TIMEOUT_MS = 3000;

/** The checks one ready round runs. createDomProbes gives the real ones; tests pass their own. */
export interface ReadyProbes {
	/** Resolves when every stylesheet has loaded or failed, and web fonts are ready. */
	fonts(): Promise<void>;
	/** Resolves when every image has loaded or failed. */
	images(): Promise<void>;
	/** Resolves when running animations have finished. */
	animations(): Promise<void>;
	/** Resolves after two animation frames, so the last change has painted. See DomProbeOptions.requirePaint. */
	paint(): Promise<void>;
	/** Whether nothing is loading right now: no stylesheet and no web font. */
	settledNow(): boolean;
}

export interface DomProbeOptions {
	/**
	 * Wait for animations that repeat forever too, up to
	 * ANIMATION_TIMEOUT_MS. Print and capture pages do, so an export
	 * captures what it always did. A live page skips them, so a spinner
	 * does not hold every ready cycle for the full limit.
	 */
	includeInfiniteAnimations: boolean;
	/**
	 * Whether ready means the page has painted, or only that its DOM has
	 * settled. A capture (a PDF export, an image export, the desktop app's
	 * thumbnail renderer) photographs the page, so it requires a real
	 * paint: without one the picture comes out blank, and it is worth
	 * waiting for a window that never draws until the export's own time
	 * limit runs out. A live page requires only a settled DOM, so a hidden
	 * window still reports ready: WebKit runs no animation frames for a
	 * page that is covered, minimized or on another space, so a paint
	 * there is a wait with no end. A window reported visible that still
	 * runs no frames reports ready after PAINT_TIMEOUT_MS (see
	 * paintWhenVisible).
	 */
	requirePaint: boolean;
	/** The document to check. Defaults to the page's own. */
	document?: Document;
}

/** Stylesheets that failed, or took too long: never waited for again. */
const settledStylesheets = new WeakSet<HTMLLinkElement>();

/** Resolves when `promise` settles or `timeoutMs` passes, whichever is first. */
function settleWithin(promise: Promise<unknown>, timeoutMs: number): Promise<void> {
	return new Promise((resolve) => {
		const timer = setTimeout(resolve, timeoutMs);
		const finish = (): void => {
			clearTimeout(timer);
			resolve();
		};
		promise.then(finish, finish);
	});
}

/** Resolves on the element's next load or error event. */
function loadedOrFailed(element: HTMLElement): Promise<void> {
	return new Promise((resolve) => {
		element.addEventListener('load', () => resolve(), { once: true });
		element.addEventListener('error', () => resolve(), { once: true });
	});
}

function pendingStylesheets(target: Document): HTMLLinkElement[] {
	return Array.from(target.querySelectorAll<HTMLLinkElement>('link[rel="stylesheet"]')).filter(
		(link) => link.sheet === null && !settledStylesheets.has(link)
	);
}

async function waitForStylesheetsAndFonts(target: Document): Promise<void> {
	const stylesheets = pendingStylesheets(target);
	if (stylesheets.length > 0) {
		await settleWithin(Promise.all(stylesheets.map(loadedOrFailed)), STYLESHEET_TIMEOUT_MS);
		for (const link of stylesheets) {
			settledStylesheets.add(link);
		}
	}
	const fonts: FontFaceSet | undefined = target.fonts;
	if (fonts?.ready) {
		// settledNow still reports a font that is loading, so a round that
		// gives up here runs again instead of reporting ready.
		await settleWithin(fonts.ready, STYLESHEET_TIMEOUT_MS);
	}
}

async function waitForImages(target: Document): Promise<void> {
	const images = Array.from(target.querySelectorAll<HTMLImageElement>('img')).filter((image) => !image.complete);
	if (images.length === 0) {
		return;
	}
	await settleWithin(Promise.all(images.map(loadedOrFailed)), IMAGE_TIMEOUT_MS);
}

async function waitForAnimations(target: Document, includeInfinite: boolean): Promise<void> {
	if (typeof target.getAnimations !== 'function') {
		return;
	}
	const running = target
		.getAnimations()
		.filter((animation) => includeInfinite || animation.effect?.getComputedTiming().endTime !== Infinity);
	if (running.length === 0) {
		return;
	}
	await settleWithin(Promise.allSettled(running.map((animation) => animation.finished)), ANIMATION_TIMEOUT_MS);
}

/**
 * Resolves once two animation frames have run, so the last change has
 * painted. A hidden page draws nothing, and WebKit runs no animation
 * frames for one, so unless the caller requires a paint the wait ends the
 * moment the page is hidden, whether it already was or becomes hidden
 * partway through, and otherwise after PAINT_TIMEOUT_MS, for a page that
 * reports itself visible but still runs no frames. Ready then means the
 * DOM has settled, which is what a live page's reader asks about; only a
 * capture needs the pixels, so a page that requires a paint waits for the
 * two frames with no limit.
 */
function paintWhenVisible(target: Document, requirePaint: boolean): Promise<void> {
	return new Promise((resolve) => {
		if (typeof requestAnimationFrame !== 'function') {
			setTimeout(resolve, 16);
			return;
		}
		if (requirePaint) {
			requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
			return;
		}
		let settled = false;
		let timer: ReturnType<typeof setTimeout> | undefined;
		const finish = (): void => {
			if (settled) {
				return;
			}
			settled = true;
			clearTimeout(timer);
			target.removeEventListener('visibilitychange', onVisibilityChange);
			resolve();
		};
		const onVisibilityChange = (): void => {
			if (target.visibilityState === 'hidden') {
				finish();
			}
		};
		if (target.visibilityState === 'hidden') {
			resolve();
			return;
		}
		target.addEventListener('visibilitychange', onVisibilityChange);
		timer = setTimeout(finish, PAINT_TIMEOUT_MS);
		requestAnimationFrame(() => requestAnimationFrame(finish));
	});
}

/** The real checks, against a document. */
export function createDomProbes(options: DomProbeOptions): ReadyProbes {
	const target = options.document ?? document;
	return {
		fonts: () => waitForStylesheetsAndFonts(target),
		images: () => waitForImages(target),
		animations: () => waitForAnimations(target, options.includeInfiniteAnimations),
		paint: () => paintWhenVisible(target, options.requirePaint),
		settledNow: () => {
			const fonts: FontFaceSet | undefined = target.fonts;
			return pendingStylesheets(target).length === 0 && fonts?.status !== 'loading';
		}
	};
}
