'use client';

import { render, screen } from '@testing-library/react';
import { FeedRowComfortable } from './FeedRowComfortable';
import type { ArticleItem } from '@/lib/types';
import { useUIStore } from '@/stores/useUIStore';

const mockTranslated: ArticleItem = {
  id: 1,
  source_id: 1,
  title: 'Why Vercel AI SDK hit 2M',
  link: '#',
  language: 'en',
  title_translated: 'Vercel AI SDK 为何周下载量突破 200 万',
  source_title: 'Vercel Blog',
  published_at: new Date(Date.now() - 3 * 3600000).toISOString(),
};

const mockNative: ArticleItem = {
  id: 2,
  source_id: 2,
  title: '在北京租房踩的 12 个坑',
  link: '#',
  language: 'zh',
  source_title: 'V2EX',
  published_at: new Date(Date.now() - 5 * 3600000).toISOString(),
};

beforeEach(() => {
  useUIStore.setState({ nativeLanguage: 'zh-CN' });
});

test('renders translated title with original subtitle', () => {
  render(<FeedRowComfortable item={mockTranslated} />);
  expect(screen.getByText(/Vercel AI SDK 为何/)).toBeInTheDocument();
  expect(screen.getByText(/Why Vercel AI SDK/)).toBeInTheDocument();
});

test('does not render original subtitle for native-language article', () => {
  render(<FeedRowComfortable item={mockNative} />);
  expect(screen.queryByText(/Why Vercel/)).not.toBeInTheDocument();
});

test('shows source name and reading time footer', () => {
  render(<FeedRowComfortable item={mockTranslated} />);
  expect(screen.getByText('VERCEL BLOG')).toBeInTheDocument();
  expect(screen.getByText(/分钟阅读/i)).toBeInTheDocument();
});

test('renders mobile-sized action targets for mark-read and star actions', () => {
  render(<FeedRowComfortable item={mockTranslated} onMarkRead={vi.fn()} onStar={vi.fn()} />);

  expect(screen.getByRole('button', { name: '标已读' })).toHaveClass('min-h-11', 'min-w-11');
  expect(screen.getByRole('button', { name: 'Star article' })).toHaveClass('min-h-11', 'min-w-11');
});
