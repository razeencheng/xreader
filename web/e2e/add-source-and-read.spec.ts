import { expect, test } from 'playwright/test';
import fs from 'node:fs';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import path from 'node:path';

const storageState = process.env.PLAYWRIGHT_STORAGE_STATE ?? 'playwright/.auth/user.json';
const hasStorageState = fs.existsSync(path.resolve(storageState));

if (hasStorageState) {
  test.use({ storageState });
}

test.skip(!hasStorageState, `Missing storage state file: ${storageState}`);

const FEED_FIXTURE = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Playwright Local Feed</title>
    <link>http://127.0.0.1/</link>
    <description>Feed for E2E tests</description>
    <item>
      <title>Playwright Entry</title>
      <link>http://127.0.0.1/entry</link>
      <guid>playwright-entry-1</guid>
      <pubDate>Wed, 22 Apr 2026 12:00:00 GMT</pubDate>
      <description>Local feed item for add-source test.</description>
    </item>
  </channel>
</rss>`;

let feedServer: Server | null = null;
let localFeedUrl = '';

test.beforeAll(async () => {
  if (process.env.PLAYWRIGHT_TEST_FEED_URL) {
    return;
  }

  feedServer = createServer((request, response) => {
    if (request.url === '/feed.xml') {
      response.writeHead(200, { 'Content-Type': 'application/rss+xml; charset=utf-8' });
      response.end(FEED_FIXTURE);
      return;
    }

    response.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
    response.end('not found');
  });

  await new Promise<void>((resolve, reject) => {
    feedServer?.listen(0, '0.0.0.0', () => resolve());
    feedServer?.once('error', reject);
  });

  const address = feedServer.address() as AddressInfo | null;
  if (!address || typeof address === 'string') {
    throw new Error('Failed to start local feed server');
  }

  localFeedUrl = `http://host.docker.internal:${address.port}/feed.xml`;
});

test.afterAll(async () => {
  if (!feedServer) return;
  await new Promise<void>((resolve, reject) => {
    feedServer?.close((error) => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
  });
});

test('can add a source and open the first article', async ({ page }) => {
  await page.goto('/sources');

  const feedUrl = process.env.PLAYWRIGHT_TEST_FEED_URL ?? localFeedUrl;
  await page.getByPlaceholder('https://example.com/feed.xml').fill(feedUrl);
  await page.getByRole('button', { name: '添加' }).click();

  await expect(page.getByText('订阅源已添加。')).toBeVisible();

  await page.goto('/');
  await expect(page.locator('article').first()).toBeVisible({ timeout: 120_000 });
  await page.locator('article').first().click();

  await expect(page.locator('h1').first()).toBeVisible();
  await expect(page.locator('article').first()).toContainText(/./);
});
