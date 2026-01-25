import { test, expect, Page } from '@playwright/test';
import { spawn, ChildProcess } from 'child_process';
import { join, dirname, basename } from 'path';
import { mkdirSync, existsSync, readdirSync } from 'fs';
import { fileURLToPath } from 'url';

// ESM compatible __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

/**
 * Visual test that iterates all example templates, starts tap dev server for each,
 * and captures screenshots of all slides with fragment states.
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

// Test result tracking
interface TemplateResult {
  templateName: string;
  success: boolean;
  slideCount: number;
  fragmentCount: number;
  screenshotCount: number;
  errorCount: number;
  errors: CapturedError[];
}

// Project paths
const projectRoot = join(__dirname, '..', '..', '..');
const tapBinary = join(projectRoot, 'tap');
const examplesDir = join(projectRoot, 'examples');
const screenshotsBaseDir = join(__dirname, 'screenshots');

// Port management - start from a base and increment for each template
let nextPort = 4000;

function getNextPort(): number {
  return nextPort++;
}

// Discover all markdown files in examples directory
function discoverTemplates(): string[] {
  const files = readdirSync(examplesDir)
    .filter((file) => file.endsWith('.md'))
    .sort(); // Alphabetical order for consistent test runs
  return files;
}

// Get template name from filename (without .md extension)
function getTemplateName(filename: string): string {
  return basename(filename, '.md');
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

// Start tap dev server for a template
async function startServer(
  templatePath: string,
  port: number
): Promise<ChildProcess> {
  const serverProcess = spawn(
    tapBinary,
    ['dev', templatePath, '--port', String(port), '--headless'],
    {
      stdio: ['ignore', 'pipe', 'pipe'],
      detached: false,
    }
  );

  serverProcess.stdout?.on('data', (data) => {
    console.log(`[tap dev stdout]: ${data}`);
  });

  serverProcess.stderr?.on('data', (data) => {
    console.error(`[tap dev stderr]: ${data}`);
  });

  serverProcess.on('error', (error) => {
    console.error(`[tap dev error]: ${error.message}`);
  });

  const baseUrl = `http://localhost:${port}`;
  await waitForServer(baseUrl, 30000);

  return serverProcess;
}

// Stop server gracefully
async function stopServer(serverProcess: ChildProcess | null): Promise<void> {
  if (serverProcess) {
    serverProcess.kill('SIGTERM');
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
}

// Fetch presentation data from API
async function fetchPresentation(
  baseUrl: string
): Promise<PresentationResponse> {
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
  const screenshotPath = join(
    screenshotsDir,
    `${templateName}-slide-${slideNumber}.png`
  );
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

// Process a single template and capture all screenshots
async function processTemplate(
  page: Page,
  templateName: string,
  baseUrl: string,
  screenshotsDir: string
): Promise<TemplateResult> {
  const capturedErrors: CapturedError[] = [];
  let currentSlideNumber = 1;

  // Set up console error listener
  const consoleHandler = (msg: { type: () => string; text: () => string }) => {
    if (msg.type() === 'error') {
      const text = msg.text();
      // Ignore network resource loading errors (external images, fonts, etc.)
      // These are usually not critical errors and can be false positives
      if (
        text.includes('net::ERR_') ||
        text.includes('Failed to load resource')
      ) {
        console.warn(
          `[Network Warning] ${templateName} slide ${currentSlideNumber}: ${text}`
        );
        return;
      }
      capturedErrors.push({
        templateName,
        slideNumber: currentSlideNumber,
        errorType: 'console',
        message: text,
        timestamp: new Date(),
      });
      console.error(
        `[Console Error] ${templateName} slide ${currentSlideNumber}: ${text}`
      );
    }
  };

  // Set up page error listener (uncaught exceptions)
  const pageErrorHandler = (error: Error) => {
    capturedErrors.push({
      templateName,
      slideNumber: currentSlideNumber,
      errorType: 'pageerror',
      message: error.message,
      timestamp: new Date(),
    });
    console.error(
      `[Page Error] ${templateName} slide ${currentSlideNumber}: ${error.message}`
    );
  };

  page.on('console', consoleHandler);
  page.on('pageerror', pageErrorHandler);

  try {
    // Fetch presentation data to get slide count
    const presentation = await fetchPresentation(baseUrl);
    const totalSlides = presentation.slides.length;

    console.log(`\n📖 Processing ${templateName}: ${totalSlides} slides`);

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
        for (let fragIndex = 0; fragIndex <= fragmentCount; fragIndex++) {
          await page.waitForTimeout(300);

          if (fragIndex === fragmentCount) {
            // Final state - all fragments revealed, capture as the slide's main screenshot
            await captureSlideScreenshot(
              page,
              templateName,
              slideNumber,
              screenshotsDir
            );
          } else {
            // Intermediate fragment state
            await captureFragmentScreenshot(
              page,
              templateName,
              slideNumber,
              fragIndex,
              screenshotsDir
            );
            totalFragments++;

            // Advance to next fragment
            await page.keyboard.press('ArrowRight');
          }
        }
        totalScreenshots++;
      } else {
        // No fragments - capture single screenshot
        await captureSlideScreenshot(
          page,
          templateName,
          slideNumber,
          screenshotsDir
        );
        totalScreenshots++;
      }

      // Navigate to next slide (except for the last one)
      if (slideNumber < totalSlides) {
        await page.keyboard.press('ArrowRight');
        currentSlideNumber = slideNumber + 1;
      }
    }

    console.log(
      `   ✓ Captured ${totalScreenshots} slides + ${totalFragments} fragment states`
    );

    return {
      templateName,
      success: capturedErrors.length === 0,
      slideCount: totalSlides,
      fragmentCount: totalFragments,
      screenshotCount: totalScreenshots + totalFragments,
      errorCount: capturedErrors.length,
      errors: capturedErrors,
    };
  } finally {
    // Clean up listeners
    page.removeListener('console', consoleHandler);
    page.removeListener('pageerror', pageErrorHandler);
  }
}

// Print summary of all template results
function printSummary(results: TemplateResult[]): void {
  const totalTemplates = results.length;
  const successfulTemplates = results.filter((r) => r.success).length;
  const failedTemplates = results.filter((r) => !r.success);
  const totalSlides = results.reduce((sum, r) => sum + r.slideCount, 0);
  const totalScreenshots = results.reduce((sum, r) => sum + r.screenshotCount, 0);
  const totalErrors = results.reduce((sum, r) => sum + r.errorCount, 0);

  console.log('\n' + '='.repeat(60));
  console.log('                    VISUAL TEST SUMMARY');
  console.log('='.repeat(60));
  console.log(`Templates processed: ${totalTemplates}`);
  console.log(`  Successful: ${successfulTemplates}`);
  console.log(`  Failed: ${failedTemplates.length}`);
  console.log(`Total slides: ${totalSlides}`);
  console.log(`Total screenshots: ${totalScreenshots}`);
  console.log(`Total errors: ${totalErrors}`);

  if (failedTemplates.length > 0) {
    console.log('\n--- Failed Templates ---');
    for (const result of failedTemplates) {
      console.log(`\n${result.templateName} (${result.errorCount} errors):`);
      for (const error of result.errors) {
        console.log(
          `  [${error.errorType}] Slide ${error.slideNumber}: ${error.message}`
        );
      }
    }
  }

  console.log('='.repeat(60) + '\n');
}

test.describe('Visual Tests for All Example Templates', () => {
  // Ensure screenshots directory exists
  test.beforeAll(async () => {
    if (!existsSync(screenshotsBaseDir)) {
      mkdirSync(screenshotsBaseDir, { recursive: true });
    }
  });

  test('captures screenshots of all templates in examples/', async ({
    page,
  }) => {
    const templateFiles = discoverTemplates();
    const results: TemplateResult[] = [];

    console.log(`\nDiscovered ${templateFiles.length} templates:`);
    templateFiles.forEach((file) => console.log(`  - ${file}`));

    for (const templateFile of templateFiles) {
      const templateName = getTemplateName(templateFile);
      const templatePath = join(examplesDir, templateFile);
      const port = getNextPort();
      const baseUrl = `http://localhost:${port}`;

      // Create template-specific screenshots directory
      const templateScreenshotsDir = join(screenshotsBaseDir, templateName);
      if (!existsSync(templateScreenshotsDir)) {
        mkdirSync(templateScreenshotsDir, { recursive: true });
      }

      let serverProcess: ChildProcess | null = null;

      try {
        // Start server for this template
        serverProcess = await startServer(templatePath, port);

        // Process template and capture screenshots
        const result = await processTemplate(
          page,
          templateName,
          baseUrl,
          templateScreenshotsDir
        );
        results.push(result);
      } catch (error) {
        console.error(`\n❌ Failed to process ${templateName}:`, error);
        results.push({
          templateName,
          success: false,
          slideCount: 0,
          fragmentCount: 0,
          screenshotCount: 0,
          errorCount: 1,
          errors: [
            {
              templateName,
              slideNumber: 0,
              errorType: 'pageerror',
              message: error instanceof Error ? error.message : String(error),
              timestamp: new Date(),
            },
          ],
        });
      } finally {
        // Stop server for this template
        await stopServer(serverProcess);
      }
    }

    // Print summary
    printSummary(results);

    // Fail test if any template had errors
    const totalErrors = results.reduce((sum, r) => sum + r.errorCount, 0);
    expect(
      totalErrors,
      `Detected ${totalErrors} console/page errors across ${results.filter((r) => !r.success).length} templates`
    ).toBe(0);
  });
});
