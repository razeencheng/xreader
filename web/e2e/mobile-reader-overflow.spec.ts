import { expect, test } from 'playwright/test';

const user = {
  id: 1,
  github_username: 'jin',
  role: 'user',
  native_language: 'zh-CN',
  density_pref: 'comfortable',
  theme_pref: 'light',
};

const article = {
  id: 1,
  source_id: 1,
  title: '横向溢出测试',
  link: 'https://example.com/articles/overflow',
  language: 'zh-CN',
  source_title: '来源',
  published_at: new Date('2026-04-25T12:00:00Z').toISOString(),
  summary: '用于验证移动端阅读页不会横向晃动。',
  is_read: false,
  is_starred: false,
};

const wideContent = `
  <p>本文方法旨在学术探讨和网络安全防御，强调技术应用需符合法律法规和伦理规范。</p>
  <p>https://arxiv.org/abs/1808.07945?this_is_a_very_long_query_that_should_not_force_horizontal_scrolling_because_reader_should_wrap_long_urls_and_keep_alignment_stable</p>
  <figure><img src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='1400' height='500'%3E%3Crect width='1400' height='500' fill='%230b2447'/%3E%3C/svg%3E" style="width:1400px;max-width:none" /></figure>
  <table><tr><td>longlonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglong</td></tr></table>
`;

test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });

test('mobile reader keeps wide article content pinned to the viewport', async ({ page }) => {
  await page.route('**/api/auth/me', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(user) }));
  await page.route('**/api/articles/changes?**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [] }) }));
  await page.route('**/api/articles?**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [article], next_cursor: null }) }));
  await page.route('**/api/articles/1/highlights', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [] }) }));
  await page.route('**/api/articles/1', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ ...article, content_html: wideContent, content_text: 'overflow fixture' }),
    }),
  );

  await page.goto('/read/1?ctx=today');
  await expect(page.locator('h1')).toHaveText('横向溢出测试');

  const metrics = await page.evaluate(() => {
    const scrollContainer = document.querySelector('[data-reader-scroll="true"]') as HTMLElement | null;

    const articleElements = Array.from(document.querySelectorAll('article :is(p, h1, h2, h3, figure, img, blockquote, pre, table)'));
    const overflowingArticleElements = articleElements
      .map((element) => {
        const rect = element.getBoundingClientRect();
        return { tag: element.tagName, left: rect.left, right: rect.right };
      })
      .filter((rect) => rect.left < -1 || rect.right > window.innerWidth + 1);

    return {
      readerOverflowX: scrollContainer ? getComputedStyle(scrollContainer).overflowX : 'missing',
      overflowingArticleElements,
    };
  });

  expect(metrics.readerOverflowX).toBe('hidden');
  expect(metrics.overflowingArticleElements).toEqual([]);
});
