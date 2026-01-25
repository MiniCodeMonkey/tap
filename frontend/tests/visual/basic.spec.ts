import { test, expect, Page } from '@playwright/test';
import { spawn, ChildProcess } from 'child_process';
import { join, dirname } from 'path';
import { mkdirSync, existsSync } from 'fs';
import { fileURLToPath } from 'url';

// ESM compatible __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

/**
 * Visual test that starts tap dev server and captures screenshots of all slides.
 * Tests examples/basic.md template.
 */

// API response types
interface TransformedFragment {
  content: string;
  index: number;
}

interface PresentationSlide {
  index: number;
  content: string;
  html: string;
  fragments: TransformedFragment[];
}

interface PresentationResponse {
  slides: PresentationSlide[];
}

// Error tracking for console and page errors
interface CapturedError {
  templateName: string;
  slideNumber: number;
  errorType: 'console' | 'pageerror';
  message: string;
  timestamp: Date;
}

// Get a dynamic port to avoid conflicts
function getPort(): number {
  // Use a random port between 4000-5000 for visual tests
  return 4000 + Math.floor(Math.random() * 1000);
}

// Wait for HTTP server to be ready
async function waitForServer(url: string, timeout = 30000): Promise<void> {
  const startTime = Date.now();

  while (Date.now() - startTime < timeout) {
    try {
      const response = await fetch(url);
      if (response.ok) {
        return;
      }
    } catch {
      // Server not ready yet, continue waiting
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  throw new Error(`Server did not become ready at ${url} within ${timeout}ms`);
}

// Fetch presentation data from API
async function fetchPresentation(baseUrl: string): Promise<PresentationResponse> {
  const response = await fetch(`${baseUrl}/api/presentation`);
  if (!response.ok) {
    throw new Error(`Failed to fetch presentation: ${response.status}`);
  }
  return response.json() as Promise<PresentationResponse>;
}

// Capture screenshot for a specific slide
async function captureSlideScreenshot(
  page: Page,
  templateName: string,
  slideNumber: number,
  screenshotsDir: string
): Promise<void> {
  const screenshotPath = join(screenshotsDir, `${templateName}-slide-${slideNumber}.png`);
  await page.screenshot({
    path: screenshotPath,
    fullPage: false,
  });
}

// Capture screenshot for a specific fragment state
async function captureFragmentScreenshot(
  page: Page,
  templateName: string,
  slideNumber: number,
  fragmentIndex: number,
  screenshotsDir: string
): Promise<void> {
  const screenshotPath = join(
    screenshotsDir,
    `${templateName}-slide-${slideNumber}-frag-${fragmentIndex}.png`
  );
  await page.screenshot({
    path: screenshotPath,
    fullPage: false,
  });
}

test.describe('Basic Template Visual Test', () => {
  let serverProcess: ChildProcess | null = null;
  let port: number;
  let baseUrl: string;
  const capturedErrors: CapturedError[] = [];
  let currentSlideNumber = 1;

  test.beforeAll(async () => {
    port = getPort();
    baseUrl = `http://localhost:${port}`;

    // Path to tap binary (root of project is 3 levels up from tests/visual/)
    const projectRoot = join(__dirname, '..', '..', '..');
    const tapBinary = join(projectRoot, 'tap');
    // Path to examples/basic.md
    const basicMd = join(projectRoot, 'examples', 'basic.md');

    // Ensure screenshots directory exists
    const screenshotsDir = join(__dirname, 'screenshots');
    if (!existsSync(screenshotsDir)) {
      mkdirSync(screenshotsDir, { recursive: true });
    }

    // Start tap dev server in headless mode
    serverProcess = spawn(tapBinary, ['dev', basicMd, '--port', String(port), '--headless'], {
      stdio: ['ignore', 'pipe', 'pipe'],
      detached: false,
    });

    // Log server output for debugging
    serverProcess.stdout?.on('data', (data) => {
      console.log(`[tap dev stdout]: ${data}`);
    });

    serverProcess.stderr?.on('data', (data) => {
      console.error(`[tap dev stderr]: ${data}`);
    });

    serverProcess.on('error', (error) => {
      console.error(`[tap dev error]: ${error.message}`);
    });

    // Wait for server to be ready
    await waitForServer(baseUrl, 30000);
  });

  test.afterAll(async () => {
    // Stop the server
    if (serverProcess) {
      serverProcess.kill('SIGTERM');
      // Wait a bit for graceful shutdown
      await new Promise((resolve) => setTimeout(resolve, 500));
      serverProcess = null;
    }
  });

  test('captures screenshot of all slides in basic template', async ({ page }) => {
    const screenshotsDir = join(__dirname, 'screenshots');
    const templateName = 'basic';

    // Set up console error listener
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        capturedErrors.push({
          templateName,
          slideNumber: currentSlideNumber,
          errorType: 'console',
          message: msg.text(),
          timestamp: new Date(),
        });
        console.error(`[Console Error] Slide ${currentSlideNumber}: ${msg.text()}`);
      }
    });

    // Set up page error listener (uncaught exceptions)
    page.on('pageerror', (error) => {
      capturedErrors.push({
        templateName,
        slideNumber: currentSlideNumber,
        errorType: 'pageerror',
        message: error.message,
        timestamp: new Date(),
      });
      console.error(`[Page Error] Slide ${currentSlideNumber}: ${error.message}`);
    });

    // Fetch presentation data to get slide count
    const presentation = await fetchPresentation(baseUrl);
    const totalSlides = presentation.slides.length;
    expect(totalSlides).toBeGreaterThan(0);

    console.log(`Found ${totalSlides} slides in ${templateName} template`);

    // Navigate to the presentation
    await page.goto(baseUrl);

    // Wait for the slide content to be visible
    await page.waitForSelector('.slide', { timeout: 10000 });

    // Wait for any animations to settle
    await page.waitForTimeout(500);

    let totalScreenshots = 0;
    let totalFragments = 0;
    currentSlideNumber = 1;

    // Capture screenshot for each slide
    for (let slideNumber = 1; slideNumber <= totalSlides; slideNumber++) {
      // Wait for slide transition to complete
      await page.waitForTimeout(300);

      // Get fragment count for this slide (0-indexed in API, 1-indexed for display)
      const slide = presentation.slides[slideNumber - 1];
      const fragmentCount = slide.fragments?.length || 0;

      if (fragmentCount > 0) {
        // Slide has fragments - capture each fragment state
        console.log(
          `Slide ${slideNumber} has ${fragmentCount} fragments, capturing each state...`
        );

        // First fragment state (fragment 0) - this is the initial state before any fragments revealed
        // Actually, when we land on a slide with fragments, we see fragment index 0 content
        // Pressing ArrowRight reveals the next fragment until all are revealed
        // Then pressing ArrowRight again moves to the next slide

        for (let fragIndex = 0; fragIndex <= fragmentCount; fragIndex++) {
          await page.waitForTimeout(300);

          if (fragIndex === fragmentCount) {
            // Final state - all fragments revealed, capture as the slide's main screenshot
            await captureSlideScreenshot(page, templateName, slideNumber, screenshotsDir);
            console.log(`Captured final state for slide ${slideNumber} (all fragments revealed)`);
          } else {
            // Intermediate fragment state
            await captureFragmentScreenshot(
              page,
              templateName,
              slideNumber,
              fragIndex,
              screenshotsDir
            );
            console.log(
              `Captured fragment ${fragIndex}/${fragmentCount} for slide ${slideNumber}`
            );
            totalFragments++;

            // Advance to next fragment
            await page.keyboard.press('ArrowRight');
          }
        }
        totalScreenshots++;
      } else {
        // No fragments - capture single screenshot
        await captureSlideScreenshot(page, templateName, slideNumber, screenshotsDir);
        console.log(`Captured screenshot for slide ${slideNumber}/${totalSlides}`);
        totalScreenshots++;
      }

      // Navigate to next slide (except for the last one)
      if (slideNumber < totalSlides) {
        await page.keyboard.press('ArrowRight');
        currentSlideNumber = slideNumber + 1;
      }
    }

    console.log(
      `Completed: captured ${totalScreenshots} slides + ${totalFragments} fragment states for ${templateName}`
    );

    // Print error summary
    console.log('\n=== Error Summary ===');
    if (capturedErrors.length === 0) {
      console.log('No errors detected during visual test run.');
    } else {
      console.log(`Total errors detected: ${capturedErrors.length}`);
      console.log('\nError details:');
      for (const error of capturedErrors) {
        console.log(`  [${error.errorType}] ${error.templateName} slide ${error.slideNumber}: ${error.message}`);
      }
    }
    console.log('=====================\n');

    // Fail test if any errors were detected
    expect(capturedErrors.length, `Detected ${capturedErrors.length} console/page errors during rendering`).toBe(0);
  });
});
