import { expect, test } from 'playwright/test';
import fs from 'node:fs';
import path from 'node:path';

const storageState = process.env.PLAYWRIGHT_STORAGE_STATE ?? 'playwright/.auth/user.json';
const hasStorageState = fs.existsSync(path.resolve(storageState));

if (hasStorageState) {
  test.use({ storageState });
}

test.skip(!hasStorageState, `Missing storage state file: ${storageState}`);

test('navigates to the next article from reader chrome', async ({ page }) => {
  await page.goto('/');
  const articleIds = await page.evaluate(async () => {
    const response = await fetch('/api/articles?tab=today', { credentials: 'include' });
    const payload = (await response.json()) as { items?: Array<{ id?: number }> };
    return (payload.items ?? [])
      .map((item) => item.id)
      .filter((id): id is number => typeof id === 'number');
  });

  expect(articleIds.length).toBeGreaterThanOrEqual(2);

  await page.goto(`/read/${articleIds[0]}?ctx=today`);
  await expect(page.locator('h1').first()).toBeVisible({ timeout: 120_000 });

  const currentTitle = await page.locator('h1').first().innerText();
  const nextButton = page.getByRole('button', { name: /下一篇/ }).last();
  await expect(nextButton).toBeVisible();
  await nextButton.click();

  await expect(page).toHaveURL(new RegExp(`/read/${articleIds[1]}`));
  await expect(page.locator('h1').first()).not.toHaveText(currentTitle, { timeout: 120_000 });
});
