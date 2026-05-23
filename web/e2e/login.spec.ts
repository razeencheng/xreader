import { expect, test } from 'playwright/test';

test('login page shows GitHub OAuth button', async ({ page }) => {
  await page.goto('/login');

  await expect(page.getByRole('heading', { name: 'xReader' })).toBeVisible();
  await expect(page.getByRole('link', { name: '使用 GitHub 登录' })).toBeVisible();
});
