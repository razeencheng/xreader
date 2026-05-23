import { render, screen } from '@testing-library/react';
import { ReaderHeader } from './ReaderHeader';
import type { ArticleItem } from '@/lib/types';

const article: ArticleItem & { is_starred?: boolean } = {
  id: 1,
  source_id: 1,
  source_title: "Let's Encrypt",
  feed_id: 'entry-1',
  title: 'Original article',
  title_translated: '翻译标题',
  link: 'https://example.com/original',
  language: 'en',
  published_at: new Date().toISOString(),
  reading_time_minutes: 2,
  summary: '',
  is_read: false,
  is_starred: false,
};

test('keeps original-article action out of the sticky reader chrome', () => {
  render(<ReaderHeader article={article} />);

  expect(screen.queryByRole('button', { name: '阅读原文' })).not.toBeInTheDocument();
});
