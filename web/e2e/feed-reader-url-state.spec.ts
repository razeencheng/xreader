import { expect, test, type Page } from 'playwright/test';

const user = {
  id: 1,
  github_username: 'jin',
  role: 'user',
  native_language: 'zh-CN',
  density_pref: 'comfortable',
  theme_pref: 'light',
};

const article = {
  id: 42,
  source_id: 1,
  title: '刷新后仍然阅读这一篇',
  link: 'https://example.com/42',
  language: 'zh-CN',
  source_title: '测试源',
  published_at: new Date('2026-04-25T12:00:00Z').toISOString(),
  summary: '用于验证刷新不会丢失阅读锚点。',
  is_read: false,
  is_starred: false,
};

test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });

async function mockArticlePage(page: Page) {
  await page.route('**/api/auth/me', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(user) }));
  await page.route('**/api/articles/changes?**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [] }) }));
  await page.route('**/api/articles?**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [article], next_cursor: null }) }));
  await page.route('**/api/articles/42/highlights', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [] }) }));
  await page.route('**/api/articles/42', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ ...article, content_html: '<p>这篇文章刷新之后仍然应该展示。</p>', content_text: '这篇文章刷新之后仍然应该展示。' }),
    }),
  );
}

async function openArticleAndReload(page: Page) {
  await page.goto('/');
  await page.getByRole('button', { name: /刷新后仍然阅读这一篇/ }).click();

  await expect(page).toHaveURL(/article=42/);
  await expect(page.locator('h1')).toHaveText('刷新后仍然阅读这一篇');

  await page.reload();

  await expect(page).toHaveURL(/article=42/);
  await expect(page.locator('h1')).toHaveText('刷新后仍然阅读这一篇');
}

async function swipeLeftToReturn(page: Page) {
  await page.locator('[data-reader-scroll="true"]').dispatchEvent('touchstart', {
    touches: [{ identifier: 1, clientX: 320, clientY: 420 }],
  });
  await page.locator('[data-reader-scroll="true"]').dispatchEvent('touchmove', {
    touches: [{ identifier: 1, clientX: 120, clientY: 410 }],
  });
  await page.locator('[data-reader-scroll="true"]').dispatchEvent('touchend', {
    changedTouches: [{ identifier: 1, clientX: 120, clientY: 410 }],
  });
}

test('opened article is encoded in the URL and survives reload', async ({ page }) => {
  await mockArticlePage(page);
  await openArticleAndReload(page);

  await page.getByRole('button', { name: /返回/ }).click();

  await expect(page).not.toHaveURL(/article=42/);
  await expect(page.getByRole('button', { name: /刷新后仍然阅读这一篇/ })).toBeVisible();
});

test('swiping left after reload returns to the list', async ({ page }) => {
  await mockArticlePage(page);
  await openArticleAndReload(page);

  await swipeLeftToReturn(page);

  await expect(page).not.toHaveURL(/article=42/);
  await expect(page.getByRole('button', { name: /刷新后仍然阅读这一篇/ })).toBeVisible();
});
