import { expect, test } from 'playwright/test';
import fs from 'node:fs';
import path from 'node:path';

const storageState = process.env.PLAYWRIGHT_STORAGE_STATE ?? 'playwright/.auth/user.json';
const hasStorageState = fs.existsSync(path.resolve(storageState));

if (hasStorageState) {
  test.use({ storageState });
}

test.skip(!hasStorageState, `Missing storage state file: ${storageState}`);

test('density preference persists after reload', async ({ page }) => {
  await page.goto('/');

  const compactButton = page.getByRole('button', { name: '紧凑' });
  await expect(compactButton).toBeVisible();

  await compactButton.click();
  await expect(compactButton).toHaveAttribute('aria-pressed', 'true');

  await page.reload();
  await expect(compactButton).toHaveAttribute('aria-pressed', 'true');
});
