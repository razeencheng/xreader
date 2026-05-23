import { expect, test } from 'playwright/test';
import fs from 'node:fs';
import path from 'node:path';

const storageState = process.env.PLAYWRIGHT_STORAGE_STATE ?? 'playwright/.auth/user.json';
const hasStorageState = fs.existsSync(path.resolve(storageState));

if (hasStorageState) {
  test.use({ storageState });
}

test.skip(!hasStorageState, `Missing storage state file: ${storageState}`);

test('highlight persists after reload', async ({ page }) => {
  await page.goto('/');
  const articleIds = await page.evaluate(async () => {
    const response = await fetch('/api/articles?tab=today', { credentials: 'include' });
    const payload = (await response.json()) as { items?: Array<{ id?: number }> };
    return (payload.items ?? [])
      .map((item) => item.id)
      .filter((id): id is number => typeof id === 'number');
  });

  expect(articleIds.length).toBeGreaterThanOrEqual(1);
  await page.goto(`/read/${articleIds[0]}?ctx=today`);
  await page.evaluate(async (articleId) => {
    const response = await fetch(`/api/articles/${articleId}/highlights`, { credentials: 'include' });
    const payload = (await response.json()) as { items?: Array<{ id: number }> };
    for (const item of payload.items ?? []) {
      await fetch(`/api/highlights/${item.id}`, { method: 'DELETE', credentials: 'include' });
    }
  }, articleIds[0]);

  const paragraph = page.locator('[data-layer="original"]').first();
  await expect(paragraph).toBeVisible();

  const box = await paragraph.boundingBox();
  if (!box) {
    throw new Error('Unable to get paragraph bounds for selection');
  }

  const startX = box.x + Math.max(16, Math.min(40, box.width * 0.15));
  const startY = box.y + Math.max(14, Math.min(24, box.height * 0.35));
  const endX = Math.min(box.x + box.width - 12, startX + 100);

  await page.mouse.move(startX, startY);
  await page.mouse.down();
  await page.mouse.move(endX, startY, { steps: 8 });
  await page.mouse.up();

  const highlightButton = page.getByRole('button', { name: /^高亮$/ });
  await expect(highlightButton).toBeVisible();
  await highlightButton.click();

  await expect(page.locator('mark[data-highlight-id]')).toBeVisible();
  await page.reload();
  await expect(page.locator('mark[data-highlight-id]')).toBeVisible({ timeout: 120_000 });
});
