import { test, expect, Page } from '@playwright/test';
import { spawn, ChildProcess } from 'child_process';
import { join, dirname, basename } from 'path';
import { mkdirSync, existsSync, readdirSync, writeFileSync } from 'fs';
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
const galleryDir = join(__dirname, 'gallery');

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

// Screenshot info for gallery
interface ScreenshotInfo {
  filename: string;
  relativePath: string;
  slideNumber: number;
  fragmentIndex: number | null;
}

// Discover screenshots for a template
function discoverScreenshots(templateName: string): ScreenshotInfo[] {
  const templateDir = join(screenshotsBaseDir, templateName);
  if (!existsSync(templateDir)) {
    return [];
  }

  const files = readdirSync(templateDir)
    .filter((f) => f.endsWith('.png'))
    .sort();

  return files.map((filename) => {
    // Parse filename: {template}-slide-{number}.png or {template}-slide-{number}-frag-{index}.png
    const fragMatch = filename.match(/-slide-(\d+)-frag-(\d+)\.png$/);
    const slideMatch = filename.match(/-slide-(\d+)\.png$/);

    let slideNumber = 1;
    let fragmentIndex: number | null = null;

    if (fragMatch) {
      slideNumber = parseInt(fragMatch[1], 10);
      fragmentIndex = parseInt(fragMatch[2], 10);
    } else if (slideMatch) {
      slideNumber = parseInt(slideMatch[1], 10);
    }

    return {
      filename,
      relativePath: `../screenshots/${templateName}/${filename}`,
      slideNumber,
      fragmentIndex,
    };
  });
}

// Generate HTML gallery report
function generateGalleryReport(results: TemplateResult[]): void {
  // Ensure gallery directory exists
  if (!existsSync(galleryDir)) {
    mkdirSync(galleryDir, { recursive: true });
  }

  const totalTemplates = results.length;
  const successfulTemplates = results.filter((r) => r.success).length;
  const totalSlides = results.reduce((sum, r) => sum + r.slideCount, 0);
  const totalScreenshots = results.reduce(
    (sum, r) => sum + r.screenshotCount,
    0
  );
  const totalErrors = results.reduce((sum, r) => sum + r.errorCount, 0);
  const timestamp = new Date().toLocaleString();

  // Generate template sections
  const templateSections = results
    .map((result) => {
      const screenshots = discoverScreenshots(result.templateName);
      const statusClass = result.success ? 'success' : 'error';
      const statusIcon = result.success ? '✓' : '✗';

      const screenshotCards = screenshots
        .map((ss) => {
          const label =
            ss.fragmentIndex !== null
              ? `Slide ${ss.slideNumber} - Fragment ${ss.fragmentIndex}`
              : `Slide ${ss.slideNumber}`;

          return `
          <div class="screenshot-card">
            <a href="${ss.relativePath}" target="_blank">
              <img src="${ss.relativePath}" alt="${result.templateName} ${label}" loading="lazy" />
            </a>
            <div class="screenshot-label">${label}</div>
          </div>`;
        })
        .join('\n');

      const errorList =
        result.errors.length > 0
          ? `
        <div class="error-list">
          <h4>Errors:</h4>
          <ul>
            ${result.errors.map((e) => `<li><strong>[${e.errorType}] Slide ${e.slideNumber}:</strong> ${escapeHtml(e.message)}</li>`).join('\n')}
          </ul>
        </div>`
          : '';

      return `
      <section class="template-section" id="${result.templateName}">
        <h2 class="template-header ${statusClass}">
          <span class="status-icon">${statusIcon}</span>
          ${result.templateName}
          <span class="template-stats">${result.slideCount} slides, ${result.screenshotCount} screenshots</span>
        </h2>
        ${errorList}
        <div class="screenshot-grid">
          ${screenshotCards}
        </div>
      </section>`;
    })
    .join('\n');

  // Generate navigation links
  const navLinks = results
    .map((r) => {
      const statusClass = r.success ? 'success' : 'error';
      return `<a href="#${r.templateName}" class="nav-link ${statusClass}">${r.templateName}</a>`;
    })
    .join('\n');

  const html = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Tap Visual QA Gallery</title>
  <style>
    :root {
      --bg-color: #1a1a2e;
      --card-bg: #16213e;
      --text-color: #eee;
      --text-muted: #888;
      --success-color: #4ade80;
      --error-color: #f87171;
      --accent-color: #818cf8;
      --border-color: #334155;
    }

    * {
      box-sizing: border-box;
    }

    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: var(--bg-color);
      color: var(--text-color);
      margin: 0;
      padding: 0;
      line-height: 1.6;
    }

    header {
      background: var(--card-bg);
      padding: 2rem;
      border-bottom: 1px solid var(--border-color);
    }

    h1 {
      margin: 0 0 1rem 0;
      font-size: 2rem;
      font-weight: 600;
    }

    .summary-stats {
      display: flex;
      gap: 2rem;
      flex-wrap: wrap;
    }

    .stat {
      display: flex;
      flex-direction: column;
    }

    .stat-value {
      font-size: 1.5rem;
      font-weight: 600;
      color: var(--accent-color);
    }

    .stat-value.success { color: var(--success-color); }
    .stat-value.error { color: var(--error-color); }

    .stat-label {
      font-size: 0.875rem;
      color: var(--text-muted);
    }

    .timestamp {
      margin-top: 1rem;
      font-size: 0.875rem;
      color: var(--text-muted);
    }

    nav {
      background: var(--card-bg);
      padding: 1rem 2rem;
      border-bottom: 1px solid var(--border-color);
      display: flex;
      gap: 0.5rem;
      flex-wrap: wrap;
    }

    .nav-link {
      padding: 0.5rem 1rem;
      background: var(--bg-color);
      color: var(--text-color);
      text-decoration: none;
      border-radius: 6px;
      font-size: 0.875rem;
      transition: background 0.2s;
    }

    .nav-link:hover {
      background: var(--border-color);
    }

    .nav-link.success::before {
      content: '✓ ';
      color: var(--success-color);
    }

    .nav-link.error::before {
      content: '✗ ';
      color: var(--error-color);
    }

    main {
      padding: 2rem;
    }

    .template-section {
      margin-bottom: 3rem;
    }

    .template-header {
      font-size: 1.5rem;
      font-weight: 600;
      margin: 0 0 1rem 0;
      padding-bottom: 0.5rem;
      border-bottom: 2px solid var(--border-color);
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }

    .template-header.success .status-icon { color: var(--success-color); }
    .template-header.error .status-icon { color: var(--error-color); }

    .template-stats {
      font-size: 0.875rem;
      font-weight: 400;
      color: var(--text-muted);
      margin-left: auto;
    }

    .error-list {
      background: rgba(248, 113, 113, 0.1);
      border: 1px solid var(--error-color);
      border-radius: 8px;
      padding: 1rem;
      margin-bottom: 1rem;
    }

    .error-list h4 {
      margin: 0 0 0.5rem 0;
      color: var(--error-color);
    }

    .error-list ul {
      margin: 0;
      padding-left: 1.5rem;
    }

    .error-list li {
      font-size: 0.875rem;
      margin-bottom: 0.25rem;
    }

    .screenshot-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
      gap: 1.5rem;
    }

    .screenshot-card {
      background: var(--card-bg);
      border-radius: 8px;
      overflow: hidden;
      border: 1px solid var(--border-color);
      transition: transform 0.2s, box-shadow 0.2s;
    }

    .screenshot-card:hover {
      transform: translateY(-2px);
      box-shadow: 0 8px 25px rgba(0, 0, 0, 0.3);
    }

    .screenshot-card img {
      width: 100%;
      height: auto;
      display: block;
    }

    .screenshot-label {
      padding: 0.75rem 1rem;
      font-size: 0.875rem;
      color: var(--text-muted);
      border-top: 1px solid var(--border-color);
    }

    @media (max-width: 600px) {
      .screenshot-grid {
        grid-template-columns: 1fr;
      }

      .summary-stats {
        gap: 1rem;
      }
    }
  </style>
</head>
<body>
  <header>
    <h1>Tap Visual QA Gallery</h1>
    <div class="summary-stats">
      <div class="stat">
        <span class="stat-value">${totalTemplates}</span>
        <span class="stat-label">Templates</span>
      </div>
      <div class="stat">
        <span class="stat-value success">${successfulTemplates}</span>
        <span class="stat-label">Successful</span>
      </div>
      <div class="stat">
        <span class="stat-value ${totalErrors > 0 ? 'error' : ''}">${totalErrors}</span>
        <span class="stat-label">Errors</span>
      </div>
      <div class="stat">
        <span class="stat-value">${totalSlides}</span>
        <span class="stat-label">Slides</span>
      </div>
      <div class="stat">
        <span class="stat-value">${totalScreenshots}</span>
        <span class="stat-label">Screenshots</span>
      </div>
    </div>
    <div class="timestamp">Generated: ${timestamp}</div>
  </header>

  <nav>
    ${navLinks}
  </nav>

  <main>
    ${templateSections}
  </main>
</body>
</html>`;

  const galleryPath = join(galleryDir, 'index.html');
  writeFileSync(galleryPath, html, 'utf-8');
  console.log(`\n📸 Gallery report generated: ${galleryPath}`);
}

// Escape HTML special characters
function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
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

    // Generate HTML gallery report
    generateGalleryReport(results);

    // Fail test if any template had errors
    const totalErrors = results.reduce((sum, r) => sum + r.errorCount, 0);
    expect(
      totalErrors,
      `Detected ${totalErrors} console/page errors across ${results.filter((r) => !r.success).length} templates`
    ).toBe(0);
  });
});
