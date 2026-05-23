import { render, screen } from '@testing-library/react';
import { FeedRowCompact } from './FeedRowCompact';
import type { ArticleItem } from '@/lib/types';
import { useUIStore } from '@/stores/useUIStore';

const mockItem: ArticleItem = {
  id: 1,
  source_id: 1,
  title: 'Test Title',
  link: '#',
  language: 'en',
  title_translated: '测试标题',
  source_title: 'Vercel',
  published_at: new Date(Date.now() - 3 * 3_600_000).toISOString(),
};

beforeEach(() => {
  useUIStore.setState({ nativeLanguage: 'zh-CN' });
});

test('renders compact row with source label and translated title', () => {
  render(<FeedRowCompact item={mockItem} />);

  expect(screen.getByText('测试标题')).toBeInTheDocument();
  expect(screen.getByText('VERCEL')).toBeInTheDocument();
  expect(screen.getByText('3h')).toBeInTheDocument();
});

test('does not render reading-time footer in compact mode', () => {
  render(<FeedRowCompact item={mockItem} />);
  expect(screen.queryByText(/分钟阅读/i)).not.toBeInTheDocument();
});

test('renders a mobile-sized mark-read action target', () => {
  render(<FeedRowCompact item={mockItem} onMarkRead={vi.fn()} />);

  expect(screen.getByRole('button', { name: '标已读' })).toHaveClass('min-h-11', 'min-w-11');
});
