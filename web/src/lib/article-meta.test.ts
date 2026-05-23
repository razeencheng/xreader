import { isLikelySummaryOnly } from './article-meta';

test('detects single-paragraph feed summaries with source links', () => {
  expect(
    isLikelySummaryOnly({
      link: 'https://example.com/post',
      content_html: '<p>This feed only includes a short excerpt.</p>',
      content_text: 'This feed only includes a short excerpt.',
    }),
  ).toBe(true);
});

test('does not flag multi-paragraph article bodies as feed summaries', () => {
  expect(
    isLikelySummaryOnly({
      link: 'https://example.com/post',
      content_html: '<p>First full paragraph.</p><p>Second full paragraph.</p>',
      content_text: 'First full paragraph. Second full paragraph.',
    }),
  ).toBe(false);
});

test('does not show a source notice when no original link is available', () => {
  expect(
    isLikelySummaryOnly({
      link: '',
      content_html: '<p>Short text.</p>',
      content_text: 'Short text.',
    }),
  ).toBe(false);
});
