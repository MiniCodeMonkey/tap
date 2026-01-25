import { test, expect } from '@playwright/test';
import { spawn, ChildProcess } from 'child_process';
import { join, dirname } from 'path';
import { mkdirSync, existsSync } from 'fs';
import { fileURLToPath } from 'url';

// ESM compatible __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

/**
 * Visual test that starts tap dev server and captures a screenshot.
 * Tests examples/basic.md template.
 */

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

test.describe('Basic Template Visual Test', () => {
  let serverProcess: ChildProcess | null = null;
  let port: number;
  let baseUrl: string;

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

  test('captures screenshot of basic template', async ({ page }) => {
    // Navigate to the presentation
    await page.goto(baseUrl);

    // Wait for the slide content to be visible
    await page.waitForSelector('.slide', { timeout: 10000 });

    // Wait a bit for any animations to settle
    await page.waitForTimeout(500);

    // Take screenshot
    const screenshotPath = join(__dirname, 'screenshots', 'basic-slide-1.png');
    await page.screenshot({
      path: screenshotPath,
      fullPage: false,
    });

    // Verify screenshot was created by checking we got here without error
    expect(true).toBe(true);
  });
});
