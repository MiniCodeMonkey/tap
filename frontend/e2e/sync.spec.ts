import { test, expect } from '@playwright/test';

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

    // Wait for WebSocket connections to establish
    await audiencePage.waitForTimeout(1000);
    await presenterPage.waitForTimeout(1000);

    // Navigate to slide 2 in AUDIENCE view
    await audiencePage.keyboard.press('ArrowRight');
    await audiencePage.waitForTimeout(500);

    // Verify audience moved to slide 2
    expect(audiencePage.url()).toContain('#2');

    // Wait for sync to presenter
    await presenterPage.waitForTimeout(1000);

    // Verify presenter also moved to slide 2 by checking URL hash
    const presenterUrl = presenterPage.url();
    console.log('Presenter URL after audience navigation:', presenterUrl);
    expect(presenterUrl).toContain('#2');

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

    // Wait for WebSocket connections to establish
    await audiencePage.waitForTimeout(1000);
    await presenterPage.waitForTimeout(1000);

    // Navigate to slide 2 in PRESENTER view
    await presenterPage.keyboard.press('ArrowRight');
    await presenterPage.waitForTimeout(500);

    // Verify presenter moved
    const presenterUrl = presenterPage.url();
    console.log('Presenter URL after navigation:', presenterUrl);
    expect(presenterUrl).toContain('#2');

    // Wait for sync to audience
    await audiencePage.waitForTimeout(1000);

    // Verify audience also moved to slide 2
    const audienceUrl = audiencePage.url();
    console.log('Audience URL after presenter navigation:', audienceUrl);
    expect(audienceUrl).toContain('#2');

    await audienceContext.close();
    await presenterContext.close();
  });
});
