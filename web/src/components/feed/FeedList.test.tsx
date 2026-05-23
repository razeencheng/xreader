import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

vi.mock('react-intersection-observer', () => ({
  useInView: () => ({ ref: vi.fn(), inView: false }),
}));

vi.mock('@/lib/api-client', () => ({
  apiFetch: vi.fn(),
}));

import { apiFetch } from '@/lib/api-client';
import { FeedList } from './FeedList';
import { useUIStore } from '@/stores/useUIStore';

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  vi.mocked(apiFetch).mockReset();
  useUIStore.setState({
    currentView: 'today',
    selectedSourceId: null,
    readFilter: 'unread',
    density: 'comfortable',
    nativeLanguage: 'zh-CN',
  });
});

test('FeedList shows all caught up message when unread filter has no items', async () => {
  vi.mocked(apiFetch).mockResolvedValue({ items: [], next_cursor: null });

  render(<FeedList />, { wrapper });

  expect(await screen.findByText(/全部读完|清空未读|阅读进度|一篇不落|全部搞定|未读清零/)).toBeInTheDocument();
});

test('FeedList turns a zero-source account into subscription onboarding', async () => {
  vi.mocked(apiFetch).mockImplementation(async (path) => {
    if (String(path).startsWith('/api/sources')) {
      return [];
    }

    if (String(path).startsWith('/api/articles?')) {
      return { items: [], next_cursor: null };
    }

    return {};
  });

  render(<FeedList />, { wrapper });

  expect(await screen.findByText('还没有订阅源')).toBeInTheDocument();
  expect(screen.getByText('添加第一个 RSS 源，开始生成你的阅读流。')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: /添加订阅源/ })).toHaveAttribute('href', '/sources#add-source');
  expect(screen.getByRole('link', { name: /导入 OPML/ })).toHaveAttribute('href', '/sources#opml');
});

test('FeedList only draws the split-pane divider on desktop widths', () => {
  vi.mocked(apiFetch).mockResolvedValue({ items: [], next_cursor: null });

  const { container } = render(<FeedList />, { wrapper });
  const shell = container.firstElementChild;

  expect(shell).toHaveClass('lg:border-r');
  expect(shell).toHaveClass('lg:w-[300px]');
  expect(shell).not.toHaveClass('border-r');
  expect(shell).not.toHaveClass('md:w-[300px]');
});

test('FeedList renders items', async () => {
  vi.mocked(apiFetch).mockResolvedValue({
    items: [
      {
        id: 1,
        source_id: 1,
        title: 'Test Article',
        link: 'https://example.com',
        language: 'en',
      },
    ],
    next_cursor: null,
  });

  render(<FeedList />, { wrapper });

  expect(await screen.findByText('Test Article')).toBeInTheDocument();
  expect(screen.getByText('今日')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /未读/i })).toBeInTheDocument();
});

test('renders read filters as a segmented control', async () => {
  vi.mocked(apiFetch).mockResolvedValue({
    items: [
      {
        id: 1,
        source_id: 1,
        title: 'Unread Article',
        link: 'https://example.com/1',
        language: 'en',
        is_read: false,
      },
      {
        id: 2,
        source_id: 1,
        title: 'Read Article',
        link: 'https://example.com/2',
        language: 'en',
        is_read: true,
      },
    ],
    next_cursor: null,
  });

  const { container } = render(<FeedList />, { wrapper });

  await screen.findByText('Unread Article');

  const filterGroup = screen.getByRole('group', { name: '未读 / 全部 / 已读' });
  expect(filterGroup).toHaveClass('rounded-[10px]');
  expect(filterGroup).toHaveClass('bg-[var(--bg-panel)]');

  const unread = screen.getByRole('button', { name: '未读1' });
  const all = screen.getByRole('button', { name: '全部2' });
  const read = screen.getByRole('button', { name: '已读1' });
  expect(unread).toHaveClass('rounded-[8px]');
  expect(unread).toHaveAttribute('aria-pressed', 'true');
  expect(all).toHaveAttribute('aria-pressed', 'false');
  expect(read).toHaveAttribute('aria-pressed', 'false');
  expect(container.querySelector('.read-filter-segment-active')).toBe(unread);
});

test('requests server-side unread filtering and renders durable server counts', async () => {
  useUIStore.setState({
    currentView: 'all',
    selectedSourceId: null,
    readFilter: 'unread',
    density: 'comfortable',
    nativeLanguage: 'zh-CN',
  });
  vi.mocked(apiFetch).mockImplementation(async (path) => {
    if (String(path).startsWith('/api/sources')) {
      return [{ id: 1, title: 'Imported Feed', category: 'General', unread_count: 707, icon_url: null }];
    }

    if (String(path).startsWith('/api/articles?')) {
      return {
        items: [
          {
            id: 707,
            source_id: 1,
            title: 'Imported unread article',
            link: 'https://example.com/707',
            language: 'en',
            is_read: false,
          },
        ],
        counts: { unread: 707, all: 757, read: 50 },
        next_cursor: null,
      };
    }

    return {};
  });

  render(<FeedList />, { wrapper });

  expect(await screen.findByText('Imported unread article')).toBeInTheDocument();
  expect(vi.mocked(apiFetch)).toHaveBeenCalledWith('/api/articles?tab=stream&filter=unread');
  expect(screen.getByRole('button', { name: '未读707' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '全部757' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '已读50' })).toBeInTheDocument();
});

test('FeedList aligns the aggregate sources title with the source browser title', async () => {
  useUIStore.setState({
    currentView: 'sources',
    selectedSourceId: null,
    readFilter: 'unread',
    density: 'comfortable',
    nativeLanguage: 'zh-CN',
  });
  vi.mocked(apiFetch).mockResolvedValue({ items: [], next_cursor: null });

  render(<FeedList />, { wrapper });

  expect(await screen.findByText('订阅源')).toBeInTheDocument();
  expect(screen.queryByText('所有订阅源')).not.toBeInTheDocument();
});

test('FeedList follows externally selected article id', async () => {
  vi.mocked(apiFetch).mockResolvedValue({
    items: [
      {
        id: 1,
        source_id: 1,
        title: 'First Article',
        link: 'https://example.com/1',
        language: 'en',
      },
      {
        id: 2,
        source_id: 1,
        title: 'Second Article',
        link: 'https://example.com/2',
        language: 'en',
      },
    ],
    next_cursor: null,
  });

  render(<FeedList selectedArticleId={2} />, { wrapper });

  const title = await screen.findByText('Second Article');
  expect(title.closest('[role="button"]')).toHaveAttribute('aria-current', 'true');
});

test('keeps a just-read article visible in unread filter before delayed dismissal', async () => {
  vi.mocked(apiFetch).mockImplementation(async (path) => {
    if (String(path).startsWith('/api/articles?')) {
      return {
        items: [
          {
            id: 1,
            source_id: 1,
            title: 'Read Later Article',
            link: 'https://example.com/1',
            language: 'en',
            is_read: false,
          },
        ],
        next_cursor: null,
      };
    }

    return {};
  });

  render(<FeedList />, { wrapper });

  await userEvent.click(await screen.findByRole('button', { name: '标已读' }));

  expect(screen.getByText('Read Later Article')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '撤销已读' })).toBeInTheDocument();
});

test('under server-side unread filter, a just-read row lingers then is removed after the grace period', async () => {
  useUIStore.setState({ currentView: 'today', readFilter: 'unread' });

  vi.mocked(apiFetch).mockImplementation(async (path) => {
    if (String(path).startsWith('/api/articles?')) {
      return {
        items: [{ id: 1, source_id: 1, title: 'Server Filtered', link: 'https://e.com/1', language: 'en', is_read: false }],
        next_cursor: null,
      };
    }
    return {};
  });

  render(<FeedList />, { wrapper });

  await userEvent.click(await screen.findByRole('button', { name: '标已读' }));
  expect(screen.getByText('Server Filtered')).toBeInTheDocument(); // visible during grace

  await new Promise((r) => setTimeout(r, 3100));
  await waitFor(() => expect(screen.queryByText('Server Filtered')).not.toBeInTheDocument()); // removed after grace
}, 8000);

test('removes a just-read article from the unread list after the grace period', async () => {
  vi.mocked(apiFetch).mockImplementation(async (path) => {
    if (String(path).startsWith('/api/articles?')) {
      return {
        items: [{ id: 1, source_id: 1, title: 'Delayed Archive Article', link: 'https://example.com/1', language: 'en', is_read: false }],
        next_cursor: null,
      };
    }
    return {};
  });

  render(<FeedList />, { wrapper });

  await userEvent.click(await screen.findByRole('button', { name: '标已读' }));
  expect(screen.getByText('Delayed Archive Article')).toBeInTheDocument();        // visible during grace
  expect(screen.getByRole('button', { name: '撤销已读' })).toBeInTheDocument();

  await new Promise((resolve) => setTimeout(resolve, 3100));

  await waitFor(() => expect(screen.queryByText('Delayed Archive Article')).not.toBeInTheDocument()); // removed after grace
  expect(screen.queryByRole('button', { name: '撤销已读' })).not.toBeInTheDocument();
}, 8000);

test('can batch mark the current view read and undo it', async () => {
  vi.mocked(apiFetch).mockImplementation(async (path, options) => {
    if (String(path).startsWith('/api/articles?')) {
      return {
        items: [
          {
            id: 1,
            source_id: 1,
            title: 'Bulk One',
            link: 'https://example.com/1',
            language: 'en',
            is_read: false,
          },
          {
            id: 2,
            source_id: 1,
            title: 'Bulk Two',
            link: 'https://example.com/2',
            language: 'en',
            is_read: false,
          },
        ],
        next_cursor: null,
      };
    }

    if (String(path) === '/api/articles/batch/state' && options?.method === 'POST') {
      return { status: 'updated', updated: 2, article_ids: [1, 2] };
    }

    if (String(path).startsWith('/api/articles/') && options?.method === 'PATCH') {
      return { status: 'updated' };
    }

    return {};
  });

  render(<FeedList />, { wrapper });

  expect(screen.queryByRole('button', { name: '整理未读' })).not.toBeInTheDocument();
  const bulkReadButton = await screen.findByRole('button', { name: '全部已读' });
  expect(bulkReadButton).toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: '全部2' }));
  expect(screen.queryByRole('button', { name: '全部已读' })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: '未读2' }));
  expect(screen.getByRole('button', { name: '全部已读' })).toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: '全部已读' }));
  expect(screen.getByText('标记当前列表中的所有为已读')).toBeInTheDocument();
  expect(vi.mocked(apiFetch)).not.toHaveBeenCalledWith(
    '/api/articles/batch/state',
    expect.objectContaining({ method: 'POST' }),
  );

  await userEvent.click(screen.getByRole('button', { name: '确认标记全部已读' }));

  // Bulk-marked rows leave the unread view; undo banner is offered.
  expect(await screen.findByText(/已将当前视图 2 篇标为已读/)).toBeInTheDocument();
  await waitFor(() => expect(screen.queryByText('Bulk One')).not.toBeInTheDocument());
  expect(screen.queryByText('Bulk Two')).not.toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: '撤销批量标已读' }));

  // Undo re-flips is_read=false → rows come back (cache row was never deleted).
  expect(await screen.findByText('Bulk One')).toBeInTheDocument();
  expect(screen.getByText('Bulk Two')).toBeInTheDocument();
});
