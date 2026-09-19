import { test, expect, type Page } from '@playwright/test';

/**
 * Waits for the presenter view's own connection indicator (see
 * PresenterApp.tsx) instead of a fixed sleep, so these tests only wait as
 * long as the socket actually takes to open.
 */
async function waitForPresenterConnected(page: Page): Promise<void> {
  await expect(page.locator('.presenter-connection-status.connected')).toBeVisible();
}

/**
 * The audience view (App.tsx) renders no connection indicator, so there is
 * nothing observable to wait on before the socket opens. A short fixed
 * sleep here is the one kept per the test file's policy: everything that
 * *can* be expressed as a web-first assertion (a URL, a locator) is below.
 */
async function waitForAudienceSocketToOpen(page: Page): Promise<void> {
  await page.waitForTimeout(1000);
}

// These tests must run serially since they share the same WebSocket server
test.describe.serial('Bidirectional Slide Sync', () => {
  test('should sync slide from audience to presenter view', async ({ browser }) => {
    // Open audience view
    const audienceContext = await browser.newContext();
    const audiencePage = await audienceContext.newPage();
    await audiencePage.goto('/#1'); // Start explicitly on slide 1
    await audiencePage.waitForSelector('.slide-container');

    // Open presenter view
    const presenterContext = await browser.newContext();
    const presenterPage = await presenterContext.newPage();
    await presenterPage.goto('/presenter#1'); // Start explicitly on slide 1
    await presenterPage.waitForSelector('.presenter-view');

    await waitForAudienceSocketToOpen(audiencePage);
    await waitForPresenterConnected(presenterPage);

    // Navigate to slide 2 in AUDIENCE view
    await audiencePage.keyboard.press('ArrowRight');
    await expect(audiencePage).toHaveURL(/#2$/);

    // Verify presenter also moved to slide 2, synced over the socket
    await expect(presenterPage).toHaveURL(/#2$/);

    await audienceContext.close();
    await presenterContext.close();
  });

  test('should sync slide from presenter to audience view', async ({ browser }) => {
    // Open audience view
    const audienceContext = await browser.newContext();
    const audiencePage = await audienceContext.newPage();
    await audiencePage.goto('/#1'); // Start explicitly on slide 1
    await audiencePage.waitForSelector('.slide-container');

    // Open presenter view
    const presenterContext = await browser.newContext();
    const presenterPage = await presenterContext.newPage();
    await presenterPage.goto('/presenter#1'); // Start explicitly on slide 1
    await presenterPage.waitForSelector('.presenter-view');

    await waitForAudienceSocketToOpen(audiencePage);
    await waitForPresenterConnected(presenterPage);

    // Navigate to slide 2 in PRESENTER view
    await presenterPage.keyboard.press('ArrowRight');
    await expect(presenterPage).toHaveURL(/#2$/);

    // Verify audience also moved to slide 2, synced over the socket
    await expect(audiencePage).toHaveURL(/#2$/);

    await audienceContext.close();
    await presenterContext.close();
  });

  test('should sync fragment reveal state from viewer to presenter and back', async ({ browser, request }) => {
    // "Core Features" (testdata/sample.md) has two `<!-- pause -->` markers, so two fragments.
    const presentation = await request.get('/api/presentation').then((r) => r.json());
    const slideIndex = presentation.slides.findIndex((slide: { html: string }) =>
      slide.html.includes('Core Features')
    );
    expect(slideIndex).toBeGreaterThanOrEqual(0);
    const hash = `#${slideIndex + 1}`;

    const audienceContext = await browser.newContext();
    const audiencePage = await audienceContext.newPage();
    await audiencePage.goto(`/${hash}`);
    await audiencePage.waitForSelector('.slide-container');

    const presenterContext = await browser.newContext();
    const presenterPage = await presenterContext.newPage();
    await presenterPage.goto(`/presenter${hash}`);
    await presenterPage.waitForSelector('.presenter-view');

    await waitForAudienceSocketToOpen(audiencePage);
    await waitForPresenterConnected(presenterPage);

    const presenterFragmentCounter = presenterPage.locator('.fragment-counter');
    const presenterVisibleFragments = presenterPage.locator(
      '.presenter-current-slide-panel [data-fragment-index].fragment-visible'
    );

    // Starting state: no fragment revealed yet on either view.
    await expect(presenterFragmentCounter).toHaveText('(0/2)');
    await expect(presenterVisibleFragments).toHaveCount(0);

    // Reveal the first fragment in the viewer.
    await audiencePage.keyboard.press('ArrowRight');

    await expect(presenterFragmentCounter).toHaveText('(1/2)');
    await expect(presenterVisibleFragments).toHaveCount(1);

    // Hide it again from the viewer.
    await audiencePage.keyboard.press('ArrowLeft');

    await expect(presenterFragmentCounter).toHaveText('(0/2)');
    await expect(presenterVisibleFragments).toHaveCount(0);

    // Now drive the reveal from the presenter and expect the viewer to mirror it.
    const audienceVisibleFragments = audiencePage.locator('.slide-container [data-fragment-index].fragment-visible');

    await presenterPage.keyboard.press('ArrowRight');

    await expect(audienceVisibleFragments).toHaveCount(1);
    await expect(presenterFragmentCounter).toHaveText('(1/2)');

    await presenterPage.keyboard.press('ArrowLeft');

    await expect(audienceVisibleFragments).toHaveCount(0);
    await expect(presenterFragmentCounter).toHaveText('(0/2)');

    await audienceContext.close();
    await presenterContext.close();
  });

  test('should sync a return to slide 1 (index 0) from viewer to presenter', async ({ browser }) => {
    // Regression test for the hub dropping slide index 0: 1 -> 2 -> 1 must
    // land the presenter back on slide 1, not leave it stuck on slide 2.
    const audienceContext = await browser.newContext();
    const audiencePage = await audienceContext.newPage();
    await audiencePage.goto('/#1');
    await audiencePage.waitForSelector('.slide-container');

    const presenterContext = await browser.newContext();
    const presenterPage = await presenterContext.newPage();
    await presenterPage.goto('/presenter#1');
    await presenterPage.waitForSelector('.presenter-view');

    await waitForAudienceSocketToOpen(audiencePage);
    await waitForPresenterConnected(presenterPage);

    // 1 -> 2
    await audiencePage.keyboard.press('ArrowRight');
    await expect(audiencePage).toHaveURL(/#2$/);
    await expect(presenterPage).toHaveURL(/#2$/);

    // 2 -> 1
    await audiencePage.keyboard.press('ArrowLeft');
    await expect(audiencePage).toHaveURL(/#1$/);
    await expect(presenterPage).toHaveURL(/#1$/);

    await audienceContext.close();
    await presenterContext.close();
  });

  test('should still sync after a remote navigation makes the viewer revisit its own last-broadcast slide', async ({
    browser
  }) => {
    // Regression test for the send-side broadcast dedupe: viewer -> 2,
    // presenter -> 3, viewer back -> 2 must reach the presenter even though
    // the viewer's outgoing state (slide 2) matches what it last broadcast.
    const audienceContext = await browser.newContext();
    const audiencePage = await audienceContext.newPage();
    await audiencePage.goto('/#1');
    await audiencePage.waitForSelector('.slide-container');

    const presenterContext = await browser.newContext();
    const presenterPage = await presenterContext.newPage();
    await presenterPage.goto('/presenter#1');
    await presenterPage.waitForSelector('.presenter-view');

    await waitForAudienceSocketToOpen(audiencePage);
    await waitForPresenterConnected(presenterPage);

    // Viewer -> slide 2 (viewer's last-broadcast state is now "2").
    await audiencePage.keyboard.press('ArrowRight');
    await expect(audiencePage).toHaveURL(/#2$/);
    await expect(presenterPage).toHaveURL(/#2$/);

    // Presenter -> slide 3 (viewer applies remote state "3", but its
    // last-broadcast state must not silently become "3" as a side effect).
    await presenterPage.keyboard.press('ArrowRight');
    await expect(presenterPage).toHaveURL(/#3$/);
    await expect(audiencePage).toHaveURL(/#3$/);

    // Viewer back -> slide 2. Without the fix this send is skipped because
    // "2" equals the viewer's stale last-broadcast state.
    await audiencePage.keyboard.press('ArrowLeft');
    await expect(audiencePage).toHaveURL(/#2$/);
    await expect(presenterPage).toHaveURL(/#2$/);

    await audienceContext.close();
    await presenterContext.close();
  });

  test('should land a presenter opened mid-talk on the viewer\'s current slide', async ({ browser }) => {
    // Regression test for the hub's late-joiner state: a presenter window
    // opened after the viewer has already moved on must pick up the
    // viewer's current slide, not start at slide 1.
    const audienceContext = await browser.newContext();
    const audiencePage = await audienceContext.newPage();
    await audiencePage.goto('/#1');
    await audiencePage.waitForSelector('.slide-container');
    await waitForAudienceSocketToOpen(audiencePage);

    // Move the viewer to slide 3.
    await audiencePage.keyboard.press('ArrowRight');
    await audiencePage.keyboard.press('ArrowRight');
    await expect(audiencePage).toHaveURL(/#3$/);

    // Now open the presenter, with no hash - it should still land on slide 3.
    const presenterContext = await browser.newContext();
    const presenterPage = await presenterContext.newPage();
    await presenterPage.goto('/presenter');
    await presenterPage.waitForSelector('.presenter-view');

    await expect(presenterPage).toHaveURL(/#3$/);

    await audienceContext.close();
    await presenterContext.close();
  });

  test('a second page with its own hash stays on that hash instead of jumping to the hub state', async ({
    browser
  }) => {
    // Regression test for fault 1: the hub's state must not override a URL
    // hash that names a different slide. Get a viewer onto slide 3 first (so
    // the hub has live state), then load a second page at #5 and confirm it
    // stays on 5.
    const firstContext = await browser.newContext();
    const firstPage = await firstContext.newPage();
    await firstPage.goto('/#1');
    await firstPage.waitForSelector('.slide-container');
    await waitForAudienceSocketToOpen(firstPage);

    await firstPage.keyboard.press('ArrowRight');
    await firstPage.keyboard.press('ArrowRight');
    await expect(firstPage).toHaveURL(/#3$/);

    const secondContext = await browser.newContext();
    const secondPage = await secondContext.newPage();
    await secondPage.goto('/#5');
    await secondPage.waitForSelector('.slide-container');

    // Give the hub state a chance to arrive and settle, then confirm the
    // hash still won: there is no positive DOM signal for "the hub state
    // was received and discarded", only the absence of a URL change.
    await secondPage.waitForTimeout(1000);
    await expect(secondPage).toHaveURL(/#5$/);

    await firstContext.close();
    await secondContext.close();
  });

  test('a second page with no hash lands on the hub\'s current slide', async ({ browser }) => {
    // Regression test for fault 1: with no hash to compete with, the hub's
    // live state still wins for a plain viewer load, not only a presenter
    // load (see the presenter-specific test above).
    const firstContext = await browser.newContext();
    const firstPage = await firstContext.newPage();
    await firstPage.goto('/#1');
    await firstPage.waitForSelector('.slide-container');
    await waitForAudienceSocketToOpen(firstPage);

    await firstPage.keyboard.press('ArrowRight');
    await firstPage.keyboard.press('ArrowRight');
    await expect(firstPage).toHaveURL(/#3$/);

    const secondContext = await browser.newContext();
    const secondPage = await secondContext.newPage();
    await secondPage.goto('/');
    await secondPage.waitForSelector('.slide-container');

    await expect(secondPage).toHaveURL(/#3$/);

    await firstContext.close();
    await secondContext.close();
  });

  test('a second page in print mode renders its own hash slide with all fragments, ignoring the hub state', async ({
    browser,
    request
  }) => {
    // Regression test for fault 1: print mode (PDF export) never connects
    // the websocket, so the hub's live state must never reach it - it always
    // renders exactly the slide its own hash names, with every fragment
    // visible (print mode's "show everything" contract).
    const presentation = await request.get('/api/presentation').then((r) => r.json());
    const fragmentSlideIndex = presentation.slides.findIndex((slide: { html: string }) =>
      slide.html.includes('Core Features')
    );
    expect(fragmentSlideIndex).toBeGreaterThanOrEqual(0);

    const firstContext = await browser.newContext();
    const firstPage = await firstContext.newPage();
    await firstPage.goto('/#1');
    await firstPage.waitForSelector('.slide-container');
    await waitForAudienceSocketToOpen(firstPage);

    await firstPage.keyboard.press('ArrowRight');
    await firstPage.keyboard.press('ArrowRight');
    await expect(firstPage).toHaveURL(/#3$/);

    const printContext = await browser.newContext();
    const printPage = await printContext.newPage();
    await printPage.goto(`/?print=true#${fragmentSlideIndex + 1}`);
    await printPage.waitForSelector('.slide-container');

    // Stays on its own hash slide, not the hub's slide 3.
    await expect(printPage).toHaveURL(new RegExp(`#${fragmentSlideIndex + 1}$`));

    const visibleFragments = printPage.locator('.slide-container [data-fragment-index].fragment-visible');
    const hiddenFragments = printPage.locator('.slide-container [data-fragment-index].fragment-hidden');
    await expect(visibleFragments).toHaveCount(2);
    await expect(hiddenFragments).toHaveCount(0);

    await firstContext.close();
    await printContext.close();
  });

  test('a page with its own hash still follows a live navigation on a hub with no prior state', async ({
    browser
  }) => {
    // Regression test: with nobody having navigated yet (no hub state), the
    // first "slide" message a connected page receives can be another page's
    // live navigation, not the hub's late-joiner state - it must be applied
    // even though this page's own hash names a different slide.
    const pageAContext = await browser.newContext();
    const pageA = await pageAContext.newPage();
    await pageA.goto('/#5');
    await pageA.waitForSelector('.slide-container');
    await waitForAudienceSocketToOpen(pageA);
    await expect(pageA).toHaveURL(/#5$/);

    const pageBContext = await browser.newContext();
    const pageB = await pageBContext.newPage();
    await pageB.goto('/');
    await pageB.waitForSelector('.slide-container');
    await waitForAudienceSocketToOpen(pageB);

    await pageB.keyboard.press('ArrowRight');
    await pageB.keyboard.press('ArrowRight');
    await expect(pageB).toHaveURL(/#3$/);

    // Page A follows the live navigation, despite its own hash naming 5.
    await expect(pageA).toHaveURL(/#3$/);

    await pageAContext.close();
    await pageBContext.close();
  });
});
